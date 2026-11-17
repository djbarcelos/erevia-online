package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

// Casa de leilões no banco de verdade: a operação vai na mesma transação do personagem,
// uma vez só; dois compradores e comprar no instante em que vence; cancelar, vencer e
// retirar do correio.

type aucDB struct {
	t   *testing.T
	s   *store.Store
	db  *pgx.Conn
	ctx context.Context
	seq int64
}

func newAucDB(t *testing.T) *aucDB {
	s, url := storetest.NewWithURL(t)
	ctx := context.Background()
	db, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(context.Background()) })
	return &aucDB{t: t, s: s, db: db, ctx: ctx, seq: 100}
}

func (a *aucDB) char(name string) store.Character {
	a.t.Helper()
	acc, err := a.s.LoginIdentity(a.ctx, "dev", "auc-"+name, name)
	if err != nil {
		a.t.Fatal(err)
	}
	c, err := a.s.CreateCharacter(a.ctx, acc, name, "knight", game.DefaultLook("knight"))
	if err != nil {
		a.t.Fatal(err)
	}
	return c
}

// save grava o estado do personagem com as operações (ouro e itens como vieram).
func (a *aucDB) save(c store.Character, gold int64, items []game.ItemRecord, ops ...store.AuctionOp) ([]store.AuctionResult, error) {
	a.seq++
	return a.s.SaveStatesAuction(a.ctx, []store.CharacterState{{
		CharacterID: c.ID, Level: 1, HP: -1, MP: -1, Gold: gold, Items: items, Seq: a.seq, Auction: ops,
	}})
}

func (a *aucDB) one(c store.Character, op store.AuctionOp) store.AuctionResult {
	a.t.Helper()
	res, err := a.save(c, 100, nil, op)
	if err != nil || len(res) != 1 {
		a.t.Fatalf("%s %d: %+v, %v", op.Kind, op.ID, res, err)
	}
	return res[0]
}

func (a *aucDB) count(sql string, args ...any) int {
	a.t.Helper()
	var n int
	if err := a.db.QueryRow(a.ctx, sql, args...).Scan(&n); err != nil {
		a.t.Fatal(err)
	}
	return n
}

var aucAt = time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

func listOp(id int64, item string, count int, price int64) store.AuctionOp {
	return store.AuctionOp{ID: id, Kind: "list", At: aucAt, Item: item, Count: count, Price: price, Fee: 20, Hours: 12, MaxListings: 10,
		Events: []store.GameEvent{{At: aucAt, Kind: "gold", Ref: "auction_fee", Amount: -20},
			{At: aucAt, Kind: "item_out", Ref: item, Amount: int64(count), Extra: map[string]any{"how": "auction_sale"}}}}
}

func buyOp(id, auction int64, price int64, at time.Time) store.AuctionOp {
	return store.AuctionOp{ID: id, Kind: "buy", At: at, AuctionID: auction, Price: price, Commission: price * 5 / 100,
		Events: []store.GameEvent{{At: at, Kind: "gold", Ref: "auction_buy", Amount: -price}}}
}

func claimOp(id int64, a store.MailAttachment) store.AuctionOp {
	return store.AuctionOp{ID: id, Kind: "claim", At: aucAt, MailID: a.MailID, Item: a.Item, Count: a.Count, Enchant: a.Enchant, Gold: a.Gold}
}

func TestAuctionListBuyClaimInTheSaveTransaction(t *testing.T) {
	a := newAucDB(t)
	ana, beto := a.char("Ana"), a.char("Beto")
	ring := []game.ItemRecord{{Slot: 0, ItemID: "potion_hp_s", Count: 3}}

	r := a.one(ana, listOp(1, "ring_good", 1, 1000))
	if !r.OK || !r.Fresh || r.AuctionID == 0 {
		t.Fatalf("anunciar: %+v", r)
	}
	// O mesmo salvamento gravou a bolsa nova.
	if n := a.count(`SELECT count(*) FROM character_items WHERE character_id = $1 AND item_id = 'potion_hp_s'`, ana.ID); n != 0 {
		t.Fatalf("a bolsa salva com a operação era vazia: %d itens", n)
	}
	// Repetida (vem em todo salvamento até a zona saber): não aplica de novo.
	res, err := a.save(ana, 100, ring, listOp(1, "ring_good", 1, 1000))
	if err != nil || len(res) != 1 || res[0].Fresh || res[0].AuctionID != r.AuctionID {
		t.Fatalf("repetida: %+v, %v", res, err)
	}
	if n := a.count(`SELECT count(*) FROM auctions`); n != 1 {
		t.Fatalf("%d anúncios", n)
	}
	if n := a.count(`SELECT count(*) FROM game_events WHERE ref = 'auction_fee'`); n != 1 {
		t.Fatalf("taxa na telemetria %d vezes", n)
	}

	list, total, err := a.s.SearchAuctions(a.ctx, store.AuctionSearch{Now: aucAt, Limit: 20})
	if err != nil || total != 1 || list[0].SellerName != "Ana" || list[0].Item != "ring_good" {
		t.Fatalf("busca: %+v %d %v", list, total, err)
	}

	b := a.one(beto, buyOp(2, r.AuctionID, 1000, aucAt.Add(time.Minute)))
	if !b.OK || b.SellerID != ana.ID || b.Gold != 950 || b.MailID == 0 || b.Item != "ring_good" {
		t.Fatalf("comprar: %+v", b)
	}
	sale, _ := a.s.MailAttachments(a.ctx, ana.ID)
	got, _ := a.s.MailAttachments(a.ctx, beto.ID)
	if len(sale) != 1 || sale[0].Gold != 950 || sale[0].How != "auction_sale" || len(got) != 1 || got[0].Item != "ring_good" ||
		got[0].How != "auction_buy" || got[0].MailID != b.MailID {
		t.Fatalf("anexos: vendedora %+v, comprador %+v", sale, got)
	}
	if n := a.count(`SELECT count(*) FROM auctions WHERE status = 'sold' AND commission = 50 AND buyer_id = $1`, beto.ID); n != 1 {
		t.Fatal("o anúncio fica vendido, com a comissão")
	}

	// Com o anexo, a carta não pode ser apagada.
	if err := a.s.DeleteMail(a.ctx, beto.ID, got[0].MailID, aucAt); err != nil {
		t.Fatal(err)
	}
	if box, _ := a.s.Inbox(a.ctx, beto.ID); len(box) != 1 {
		t.Fatal("a carta com anexo não some")
	}
	c := a.one(beto, claimOp(3, got[0]))
	if !c.OK {
		t.Fatalf("retirar: %+v", c)
	}
	if again := a.one(beto, claimOp(4, got[0])); again.OK || again.Reason != "clash" {
		t.Fatalf("retirar de novo: %+v", again)
	}
	_ = a.s.DeleteMail(a.ctx, beto.ID, got[0].MailID, aucAt)
	if box, _ := a.s.Inbox(a.ctx, beto.ID); len(box) != 0 {
		t.Fatal("retirado, a carta pode ser apagada")
	}
}

func TestAuctionFailedSaveAppliesNothing(t *testing.T) {
	a := newAucDB(t)
	ana := a.char("Ana")
	// O lote falha no meio (item inválido): nem o estado nem a operação ficam gravados.
	_, err := a.save(ana, 7, []game.ItemRecord{{Slot: 0, ItemID: "potion_hp_s", Count: -1}}, listOp(1, "ring_good", 1, 500))
	if err == nil {
		t.Fatal("o lote devia falhar")
	}
	if n := a.count(`SELECT count(*) FROM auctions`) + a.count(`SELECT count(*) FROM auction_ops`); n != 0 {
		t.Fatal("nada do lote que falhou pode ficar")
	}
	if g := a.count(`SELECT gold FROM characters WHERE id = $1`, ana.ID); g == 7 {
		t.Fatal("o estado do lote que falhou não pode ficar")
	}
	// O próximo salvamento leva a mesma operação, que entra uma vez.
	if r := a.one(ana, listOp(1, "ring_good", 1, 500)); !r.OK || !r.Fresh {
		t.Fatalf("depois da falha: %+v", r)
	}
}

func TestAuctionTwoBuyersAtOnce(t *testing.T) {
	a := newAucDB(t)
	ana, beto, caio := a.char("Ana"), a.char("Beto"), a.char("Caio")
	for i := range 8 {
		l := a.one(ana, listOp(int64(10+i), "ring_good", 1, 300))
		var wg sync.WaitGroup
		res := make([]store.AuctionResult, 2)
		errs := make([]error, 2)
		for k, c := range []store.Character{beto, caio} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var out []store.AuctionResult
				out, errs[k] = a.s.SaveStatesAuction(a.ctx, []store.CharacterState{{CharacterID: c.ID, Level: 1, HP: -1, MP: -1,
					Seq: time.Now().UnixNano(), Auction: []store.AuctionOp{buyOp(int64(100+i), l.AuctionID, 300, aucAt)}}})
				if len(out) == 1 {
					res[k] = out[0]
				}
			}()
		}
		wg.Wait()
		if errs[0] != nil || errs[1] != nil {
			t.Fatal(errs)
		}
		if res[0].OK == res[1].OK {
			t.Fatalf("rodada %d: exatamente um compra: %+v", i, res)
		}
		lost := res[0]
		if lost.OK {
			lost = res[1]
		}
		if lost.Reason != "gone" || lost.Gold != 300 || lost.MailID == 0 {
			t.Fatalf("quem perde recebe as moedas de volta: %+v", lost)
		}
	}
	if n := a.count(`SELECT count(*) FROM mail_attachments WHERE item_id = 'ring_good'`); n != 8 {
		t.Fatalf("%d anéis entregues em 8 anúncios", n)
	}
}

func TestAuctionBuyAtTheInstantItExpires(t *testing.T) {
	a := newAucDB(t)
	ana, beto := a.char("Ana"), a.char("Beto")
	sold, expired := 0, 0
	for i := range 8 {
		l := a.one(ana, listOp(int64(10+i), "ring_good", 1, 300))
		end := aucAt.Add(12 * time.Hour)
		var wg sync.WaitGroup
		var buy []store.AuctionResult
		var gone []store.AuctionClosed
		var e1, e2 error
		wg.Add(2)
		go func() {
			defer wg.Done()
			buy, e1 = a.s.SaveStatesAuction(a.ctx, []store.CharacterState{{CharacterID: beto.ID, Level: 1, HP: -1, MP: -1,
				Seq: time.Now().UnixNano(), Auction: []store.AuctionOp{buyOp(int64(100+i), l.AuctionID, 300, end.Add(-time.Millisecond))}}})
		}()
		go func() {
			defer wg.Done()
			gone, e2 = a.s.ExpireAuctions(a.ctx, end, 100)
		}()
		wg.Wait()
		if e1 != nil || e2 != nil {
			t.Fatal(e1, e2)
		}
		// O vencimento pode ter pulado a linha travada pela compra: roda de novo.
		more, _ := a.s.ExpireAuctions(a.ctx, end, 100)
		gone = append(gone, more...)
		switch {
		case buy[0].OK && len(gone) == 0:
			sold++
		case !buy[0].OK && len(gone) == 1 && buy[0].Gold == 300:
			expired++
		default:
			t.Fatalf("rodada %d: compra %+v, vencidos %+v", i, buy[0], gone)
		}
		if n := a.count(`SELECT count(*) FROM mail_attachments WHERE auction_id = $1 AND item_id = 'ring_good'`, l.AuctionID); n != 1 {
			t.Fatalf("rodada %d: o anel foi para %d lugares", i, n)
		}
	}
	t.Logf("vendidos %d, vencidos %d", sold, expired)
	// Clique depois do prazo: nunca compra.
	l := a.one(ana, listOp(50, "ring_good", 1, 300))
	if r := a.one(beto, buyOp(51, l.AuctionID, 300, aucAt.Add(12*time.Hour))); r.OK || r.Reason != "gone" {
		t.Fatalf("depois do prazo: %+v", r)
	}
}

func TestAuctionRefusalsCancelExpireAndMine(t *testing.T) {
	a := newAucDB(t)
	ana, beto := a.char("Ana"), a.char("Beto")
	l := a.one(ana, listOp(1, "potion_hp_s", 5, 100))
	// Preço diferente do anúncio e o próprio anúncio.
	if r := a.one(beto, buyOp(2, l.AuctionID, 99, aucAt)); r.OK || r.Reason != "price" || r.Gold != 99 {
		t.Fatalf("preço: %+v", r)
	}
	if r := a.one(ana, buyOp(3, l.AuctionID, 100, aucAt)); r.OK || r.Reason != "own" {
		t.Fatalf("o próprio: %+v", r)
	}
	if r := a.one(beto, buyOp(4, 999999, 100, aucAt)); r.OK || r.Reason != "gone" {
		t.Fatalf("inexistente: %+v", r)
	}
	// Limite da conta (1 neste teste): volta o item e a taxa.
	op := listOp(5, "ring_good", 1, 700)
	op.MaxListings = 1
	if r := a.one(ana, op); r.OK || r.Reason != "limit" || r.Gold != 20 || r.MailID == 0 {
		t.Fatalf("limite: %+v", r)
	}
	// Cancelar: só o dono, uma vez.
	if _, err := a.s.CancelAuction(a.ctx, beto.ID, l.AuctionID, aucAt); !errors.Is(err, store.ErrAuctionGone) {
		t.Fatalf("outro cancela: %v", err)
	}
	c, err := a.s.CancelAuction(a.ctx, ana.ID, l.AuctionID, aucAt)
	if err != nil || c.MailID == 0 || c.Auction.Item != "potion_hp_s" || c.Auction.Count != 5 {
		t.Fatalf("cancelar: %+v %v", c, err)
	}
	if _, err := a.s.CancelAuction(a.ctx, ana.ID, l.AuctionID, aucAt); !errors.Is(err, store.ErrAuctionGone) {
		t.Fatalf("cancelar de novo: %v", err)
	}
	// Vencer.
	l2 := a.one(ana, listOp(6, "ring_good", 1, 400))
	if g, _ := a.s.ExpireAuctions(a.ctx, aucAt.Add(11*time.Hour), 10); len(g) != 0 {
		t.Fatal("antes do prazo nada vence")
	}
	g, err := a.s.ExpireAuctions(a.ctx, aucAt.Add(12*time.Hour), 10)
	if err != nil || len(g) != 1 || g[0].Auction.ID != l2.AuctionID || g[0].MailID == 0 {
		t.Fatalf("vencer: %+v %v", g, err)
	}
	mine, active, err := a.s.MyAuctions(a.ctx, ana.ID, ana.AccountID, 30)
	if err != nil || len(mine) != 2 || active != 0 {
		t.Fatalf("meus anúncios: %+v %d %v", mine, active, err)
	}
	atts, _ := a.s.MailAttachments(a.ctx, ana.ID)
	items := map[string]int{}
	var gold int64
	for _, x := range atts {
		items[x.Item] += x.Count
		gold += x.Gold
		if x.How != "auction_return" {
			t.Fatalf("how: %+v", x)
		}
	}
	if items["potion_hp_s"] != 5 || items["ring_good"] != 2 || gold != 20+100 { // a taxa recusada e a compra do próprio
		t.Fatalf("devoluções: %v, %d moedas", items, gold)
	}
}

func TestAuctionSearchSortAndPages(t *testing.T) {
	a := newAucDB(t)
	ana := a.char("Ana")
	for i, p := range []int64{500, 100, 300, 200, 400} {
		a.one(ana, listOp(int64(i+1), "potion_hp_s", 1, p))
	}
	a.one(ana, listOp(9, "ring_good", 1, 150))
	prices := func(q store.AuctionSearch) ([]int64, int) {
		q.Now, q.Limit = aucAt, max(q.Limit, 2)
		l, total, err := a.s.SearchAuctions(a.ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		var out []int64
		for _, x := range l {
			out = append(out, x.Price)
		}
		return out, total
	}
	if p, total := prices(store.AuctionSearch{ItemIDs: []string{"potion_hp_s"}}); total != 5 || p[0] != 100 || p[1] != 200 {
		t.Fatalf("preço crescente: %v %d", p, total)
	}
	if p, _ := prices(store.AuctionSearch{ItemIDs: []string{"potion_hp_s"}, Offset: 4}); len(p) != 1 || p[0] != 500 {
		t.Fatalf("última página: %v", p)
	}
	if p, _ := prices(store.AuctionSearch{Sort: 1}); p[0] != 500 {
		t.Fatalf("decrescente: %v", p)
	}
	if p, total := prices(store.AuctionSearch{MinPrice: 150, MaxPrice: 300, Limit: 10}); total != 3 || len(p) != 3 {
		t.Fatalf("faixa: %v %d", p, total)
	}
	if _, total := prices(store.AuctionSearch{ItemIDs: []string{}}); total != 0 {
		t.Fatal("lista vazia de itens não acha nada")
	}
	// Vencido (mesmo sem a rodada do vencimento) some da busca.
	if _, total, _ := a.s.SearchAuctions(a.ctx, store.AuctionSearch{Now: aucAt.Add(13 * time.Hour), Limit: 5}); total != 0 {
		t.Fatal("vencido não aparece")
	}
}

func TestAuctionOpOfDeletedCharacterIsSkipped(t *testing.T) {
	a := newAucDB(t)
	ana := a.char("Ana")
	if _, err := a.s.DeleteAccount(a.ctx, ana.AccountID); err != nil {
		t.Fatal(err)
	}
	res, err := a.save(ana, 10, nil, listOp(1, "ring_good", 1, 500))
	if err != nil || len(res) != 0 {
		t.Fatalf("personagem apagado: %+v %v", res, err)
	}
}
