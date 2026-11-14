package zone

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Casa de leilões: anunciar, comprar, cancelar, vencer, bolsa cheia, dois compradores, a
// queda no meio e o vendedor offline, com o banco falso (auction_fake_test.go) fazendo o
// papel do store e da persistência. As consultas de verdade estão em store/auction_test.go.

var auctioneerID = npcIDBase + uint32(len(townNPCs)+len(teleportStones)+2)

type aucRig struct {
	t      *testing.T
	z      *Zone
	f      *fakeAuction
	social *fakeSocial
	saves  chan []SavedState
	now    *time.Time
}

func newAucRig(t *testing.T, shared *fakeAuction) *aucRig {
	t.Helper()
	saves := make(chan []SavedState, 1024)
	now := noon
	cfg := testConfig()
	cfg.Spawns = nil
	r := &aucRig{t: t, saves: saves, now: &now}
	cfg.Clock = func() time.Time { return *r.now }
	r.z = New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), saves)
	if shared != nil {
		r.f, r.social = shared, shared.social
	} else {
		r.social = newFakeSocial(FriendData{ID: 1, Name: "Ana"}, FriendData{ID: 2, Name: "Beto"},
			FriendData{ID: 3, Name: "Caio"}, FriendData{ID: 4, Name: "Dora"})
		r.f = newFakeAuction(r.social)
	}
	r.z.SetSocialBackend(r.social)
	r.z.SetAuctionBackend(r.f)
	if n := r.z.npcs[auctioneerID]; n == nil || n.npc.role != pb.NpcRole_NPC_ROLE_AUCTION {
		t.Fatal("o leiloeiro não está na praça")
	}
	return r
}

func (r *aucRig) pump() { r.z.pumpAuction(r.f, r.saves) }

// join: o personagem entra ao lado do leiloeiro (conta acc) e abre a janela.
func (r *aucRig) join(id uint32, name string, acc int64, gold int64, items ...game.ItemRecord) (*entity, *fakePeer) {
	r.t.Helper()
	n := r.z.npcs[auctioneerID]
	peer := &fakePeer{}
	r.f.char(id, acc, name)
	r.z.handle(joinEvent{info: JoinInfo{ID: id, Name: name, Class: "knight", X: n.pos.X + 1, Z: n.pos.Z, AccountID: acc,
		Gold: gold, Items: items, Peer: peer}})
	r.pump()
	p := r.z.players[id]
	r.cmd(p, &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_OPEN})
	if u := peer.lastAuction(func(u *pb.AuctionUpdate) bool { return u.Open }); u == nil || u.Rules.GetMaxListings() != AuctionMaxListings {
		r.t.Fatalf("%s: a janela não abriu: %+v", name, u)
	}
	return p, peer
}

func (r *aucRig) cmd(p *entity, m *pb.AuctionCmd) {
	p.pl.moveTokens = r.z.cfg.MaxMovesPerSec
	p.pl.auc.nextRead = 0
	if m.NpcId == 0 {
		m.NpcId = auctioneerID
	}
	r.z.handle(socialEvent{id: p.id, msg: m})
	r.pump()
}

func (r *aucRig) list(p *entity, slot, count int, price int64, hours int) {
	r.cmd(p, &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_LIST, Slot: uint32(slot), Count: uint32(count), Price: price, Hours: uint32(hours)})
}

func (r *aucRig) search(p *entity, peer *fakePeer, q *pb.AuctionQuery) []*pb.AuctionListing {
	r.t.Helper()
	peer.reset()
	r.cmd(p, &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_SEARCH, Query: q})
	u := peer.lastAuction(func(u *pb.AuctionUpdate) bool { return u.Searched })
	if u == nil {
		r.t.Fatal("a busca não respondeu")
	}
	return u.Results
}

func (r *aucRig) buy(p *entity, l *pb.AuctionListing) {
	r.cmd(p, &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_BUY, AuctionId: l.Id, Price: l.Price})
}

// inbox abre a caixa de entrada (carrega os anexos na zona) e devolve as cartas.
func (r *aucRig) inbox(p *entity, peer *fakePeer) []*pb.Mail {
	p.pl.moveTokens = r.z.cfg.MaxMovesPerSec
	p.pl.social.nextMail = 0
	r.z.handle(socialEvent{id: p.id, msg: &pb.MailCmd{Kind: pb.MailCmdKind_MAIL_CMD_LIST}})
	r.pump()
	return peer.lastMail().GetMails()
}

func (f *fakePeer) lastAuction(match func(*pb.AuctionUpdate) bool) *pb.AuctionUpdate {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if u := f.msgs[i].GetAuction(); u != nil && (match == nil || match(u)) {
			return u
		}
	}
	return nil
}

func (r *aucRig) active() []*AuctionListing {
	var out []*AuctionListing
	for _, a := range r.f.auctions {
		if a.Status == AuctionActive {
			out = append(out, a)
		}
	}
	return out
}

// flows soma a telemetria gravada com as operações: ouro e itens por how.
func (f *fakeAuction) flows() map[string]int64 {
	out := map[string]int64{}
	for _, e := range f.events {
		switch e.Kind {
		case "gold":
			out["gold:"+e.Ref] += e.Amount
		default:
			out[e.Kind+":"+e.Extra["how"].(string)] += e.Amount
		}
	}
	return out
}

func TestAuctionListMovesItemToCustodyInTheSameSave(t *testing.T) {
	r := newAucRig(t, nil)
	ana, pa := r.join(1, "Ana", 1, 500, game.ItemRecord{Slot: 3, ItemID: "potion_hp_s", Count: 10})

	r.z.handle(socialEvent{id: ana.id, msg: &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_LIST, NpcId: auctioneerID,
		Slot: 3, Count: 4, Price: 1000, Hours: 24}})
	// Antes do banco: o item e a taxa já saíram da bolsa e a operação está na memória.
	if got := ana.pl.inv.bag[3].count; got != 6 {
		t.Fatalf("bolsa: %d poções, quero 6", got)
	}
	if ana.pl.inv.gold != 500-AuctionFee(1000) || len(ana.pl.auc.ops) != 1 {
		t.Fatalf("ouro %d (taxa %d), %d operações", ana.pl.inv.gold, AuctionFee(1000), len(ana.pl.auc.ops))
	}
	// O salvamento leva a bolsa nova E a operação.
	batch := <-r.saves
	if len(batch) != 1 || len(batch[0].Auction) != 1 || batch[0].Auction[0].Kind != AuctionOpList {
		t.Fatalf("o salvamento devia levar a operação: %+v", batch)
	}
	res, _ := r.f.apply(batch)
	if len(res) != 1 || !res[0].OK {
		t.Fatalf("o banco devia anunciar: %+v", res)
	}
	r.z.onAuctionResults(res)
	r.pump()
	if len(ana.pl.auc.ops) != 0 {
		t.Fatal("a operação confirmada devia sair da memória")
	}
	act := r.active()
	if len(act) != 1 || act[0].Item != "potion_hp_s" || act[0].Count != 4 || act[0].Price != 1000 ||
		!act[0].ExpiresAt.Equal(noon.Add(24*time.Hour)) {
		t.Fatalf("anúncio: %+v", act)
	}
	if !pa.systemContains("Anunciado") {
		t.Fatal("Ana devia ser avisada")
	}
	fl := r.f.flows()
	if fl["gold:"+HowAuctionFee] != -AuctionFee(1000) || fl["item_out:"+HowAuctionSale] != 4 {
		t.Fatalf("telemetria: %v", fl)
	}
	if mine := pa.lastAuction(func(u *pb.AuctionUpdate) bool { return u.MineListed }); len(mine.GetMine()) != 1 || mine.Active != 1 {
		t.Fatalf("meus anúncios: %+v", mine)
	}
}

func TestAuctionRefusals(t *testing.T) {
	r := newAucRig(t, nil)
	ana, pa := r.join(1, "Ana", 1, 15, game.ItemRecord{Slot: 0, ItemID: "potion_hp_s", Count: 5},
		game.ItemRecord{Slot: 1, ItemID: "sword_good", Count: 1, Enchant: 3})
	cases := []struct {
		slot, count int
		price       int64
		hours       int
		want        string
	}{
		{7, 1, 100, 24, "Escolha um item"},
		{0, 1, AuctionMinPrice - 1, 24, "O preço vai"},
		{0, 1, AuctionMaxPrice + 1, 24, "O preço vai"},
		{0, 1, 100, 6, "prazo"},
		{1, 1, 5000, 24, "taxa do anúncio é de 100"}, // 2% de 5000 > 15 moedas
	}
	for _, c := range cases {
		pa.reset()
		r.list(ana, c.slot, c.count, c.price, c.hours)
		if !pa.systemContains(c.want) || len(ana.pl.auc.ops) != 0 {
			t.Errorf("anunciar %+v: queria %q", c, c.want)
		}
	}
	if ana.pl.inv.gold != 15 || ana.pl.inv.bag[0].count != 5 || ana.pl.inv.bag[1].empty() {
		t.Fatal("as recusas não podem mexer na bolsa")
	}
	// Longe do leiloeiro.
	ana.pos = Vec2{X: 40, Z: 40}
	pa.reset()
	r.list(ana, 0, 1, 100, 24)
	if !pa.systemContains("Chegue mais perto") || len(ana.pl.auc.ops) != 0 {
		t.Fatal("longe do leiloeiro não anuncia")
	}
}

func TestAuctionAccountLimit(t *testing.T) {
	r := newAucRig(t, nil)
	var items []game.ItemRecord
	for i := range AuctionMaxListings + 2 {
		items = append(items, game.ItemRecord{Slot: i, ItemID: "potion_hp_s", Count: 1})
	}
	ana, pa := r.join(1, "Ana", 1, 10000, items...)
	for i := range AuctionMaxListings + 1 {
		r.list(ana, i, 1, 50, 12)
	}
	if n := len(r.active()); n != AuctionMaxListings {
		t.Fatalf("%d anúncios ativos, máximo %d", n, AuctionMaxListings)
	}
	if !pa.systemContains("já tem 10 anúncios") || ana.pl.inv.bag[AuctionMaxListings].empty() {
		t.Fatal("o 11º devia ser recusado sem tirar o item")
	}
	// Outro personagem da mesma conta, em outra zona (o banco decide): volta pelo correio.
	r2 := newAucRig(t, r.f)
	bia, pb2 := r2.join(5, "Bia", 1, 1000, game.ItemRecord{Slot: 0, ItemID: "potion_hp_s", Count: 1})
	bia.pl.auc.active = 0 // a leitura dos anúncios foi antes de a Ana anunciar
	r2.list(bia, 0, 1, 50, 12)
	if n := len(r.active()); n != AuctionMaxListings {
		t.Fatalf("%d anúncios ativos depois da corrida", n)
	}
	if !pb2.systemContains("voltaram pelo correio") {
		t.Fatal("a Bia devia saber que o item voltou")
	}
	r2.inbox(bia, pb2)
	r2.cmd(bia, &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_CLAIM})
	if bia.pl.inv.countOf("potion_hp_s") != 1 || bia.pl.inv.gold != 1000 {
		t.Fatalf("item e taxa deviam voltar: %d poções, %d moedas", bia.pl.inv.countOf("potion_hp_s"), bia.pl.inv.gold)
	}
}

func TestAuctionBuyPaysSellerByMailAndItemGoesToBag(t *testing.T) {
	r := newAucRig(t, nil)
	ana, pa := r.join(1, "Ana", 1, 100, game.ItemRecord{Slot: 2, ItemID: "sword_good", Count: 1, Enchant: 4})
	beto, pbeto := r.join(2, "Beto", 2, 3000)
	r.list(ana, 2, 0, 2000, 48)
	got := r.search(beto, pbeto, &pb.AuctionQuery{Name: "espada de aco"}) // sem acento e minúsculas
	if len(got) != 1 || got[0].ItemId != "sword_good" || got[0].Enchant != 4 || got[0].SellerName != "Ana" || got[0].Mine {
		t.Fatalf("busca: %+v", got)
	}
	r.buy(beto, got[0])
	if beto.pl.inv.gold != 1000 {
		t.Fatalf("Beto pagou errado: %d", beto.pl.inv.gold)
	}
	slot := beto.pl.inv.slotOf("sword_good")
	if slot < 0 || beto.pl.inv.bag[slot].enchant != 4 {
		t.Fatal("a espada +4 devia ir para a bolsa do Beto")
	}
	if !pa.systemContains("Vendido") || !pbeto.systemContains("Você comprou") {
		t.Fatal("os dois deviam ser avisados")
	}
	if len(beto.pl.auc.ops) != 0 || len(ana.pl.auc.ops) != 0 {
		t.Fatal("tudo confirmado")
	}
	// A Ana recebe pelo correio as moedas menos a comissão.
	mails := r.inbox(ana, pa)
	want := int64(2000) - AuctionCommission(2000)
	if len(mails) != 1 || mails[0].Attachment.GetGold() != want || mails[0].Attachment.Claimed {
		t.Fatalf("carta da venda: %+v", mails)
	}
	gold := ana.pl.inv.gold
	r.cmd(ana, &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_CLAIM, MailId: mails[0].Id})
	if ana.pl.inv.gold != gold+want {
		t.Fatalf("Ana: %d moedas, queria %d", ana.pl.inv.gold, gold+want)
	}
	// Retirar de novo (mesmo com uma leitura velha da caixa) não dá nada.
	ana.pl.auc.mail[int64(mails[0].Id)] = MailAttachment{MailID: int64(mails[0].Id), Gold: want, How: HowAuctionSale}
	r.cmd(ana, &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_CLAIM, MailId: mails[0].Id})
	if ana.pl.inv.gold != gold+want {
		t.Fatal("retirou duas vezes")
	}
	fl := r.f.flows()
	if fl["gold:"+HowAuctionBuy] != -2000 || fl["gold:"+HowAuctionSale] != want || fl["item_in:"+HowAuctionBuy] != 1 ||
		fl["item_out:"+HowAuctionSale] != 1 || fl["gold:"+HowAuctionFee] != -AuctionFee(2000) {
		t.Fatalf("telemetria: %v", fl)
	}
	if g, u := r.f.custody([]uint32{1, 2}); g != 0 || len(u) != 0 {
		t.Fatalf("nada devia ficar na custódia: %d moedas, %v", g, u)
	}
}

func TestAuctionFullBagLeavesItemInMail(t *testing.T) {
	r := newAucRig(t, nil)
	ana, _ := r.join(1, "Ana", 1, 100, game.ItemRecord{Slot: 0, ItemID: "ring_good", Count: 1})
	var full []game.ItemRecord
	for i := range game.BagSize {
		full = append(full, game.ItemRecord{Slot: i, ItemID: "bone_old", Count: game.Items["bone_old"].Stack})
	}
	beto, pbeto := r.join(2, "Beto", 2, 500, full...)
	r.list(ana, 0, 1, 300, 12)
	l := r.search(beto, pbeto, &pb.AuctionQuery{Kind: pb.ItemKind_ITEM_KIND_RING})
	r.buy(beto, l[0])
	if beto.pl.inv.gold != 200 || beto.pl.inv.countOf("ring_good") != 0 || !pbeto.systemContains("espera no correio") {
		t.Fatal("com a bolsa cheia o anel fica no correio")
	}
	mails := r.inbox(beto, pbeto)
	if len(mails) != 1 || mails[0].Attachment.GetItemId() != "ring_good" {
		t.Fatalf("carta do anel: %+v", mails)
	}
	// Apagar a carta com o anexo é recusado (na zona e no banco).
	pbeto.reset()
	beto.pl.moveTokens = r.z.cfg.MaxMovesPerSec
	r.z.handle(socialEvent{id: beto.id, msg: &pb.MailCmd{Kind: pb.MailCmdKind_MAIL_CMD_DELETE, MailId: mails[0].Id}})
	r.pump()
	if !pbeto.systemContains("Retire o anexo") {
		t.Fatal("não pode apagar carta com anexo")
	}
	r.cmd(beto, &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_CLAIM, MailId: mails[0].Id})
	if beto.pl.inv.countOf("ring_good") != 0 {
		t.Fatal("bolsa cheia: não retira")
	}
	beto.pl.inv.bag[5] = stack{}
	r.cmd(beto, &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_CLAIM, MailId: mails[0].Id})
	if beto.pl.inv.countOf("ring_good") != 1 {
		t.Fatal("com espaço, retira")
	}
}

func TestAuctionTwoBuyersSameListing(t *testing.T) {
	r := newAucRig(t, nil)
	ana, _ := r.join(1, "Ana", 1, 100, game.ItemRecord{Slot: 0, ItemID: "potion_hp_s", Count: 10})
	beto, pbeto := r.join(2, "Beto", 2, 1000)
	caio, pcaio := r.join(3, "Caio", 3, 1000)
	r.list(ana, 0, 10, 500, 12)
	lb := r.search(beto, pbeto, nil)[0]
	lc := r.search(caio, pcaio, nil)[0]
	// Os dois clicam no mesmo tick (o banco ainda não respondeu ao primeiro).
	for _, p := range []*entity{beto, caio} {
		p.pl.moveTokens = r.z.cfg.MaxMovesPerSec
	}
	r.z.handle(socialEvent{id: beto.id, msg: &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_BUY, NpcId: auctioneerID, AuctionId: lb.Id, Price: lb.Price}})
	r.z.handle(socialEvent{id: caio.id, msg: &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_BUY, NpcId: auctioneerID, AuctionId: lc.Id, Price: lc.Price}})
	r.pump()
	if !pcaio.systemContains("Alguém já está comprando") || caio.pl.inv.gold != 1000 {
		t.Fatal("o segundo comprador da mesma zona é recusado na hora, sem pagar")
	}
	if beto.pl.inv.countOf("potion_hp_s") != 10 || beto.pl.inv.gold != 500 {
		t.Fatal("o primeiro leva")
	}

	// Em zonas diferentes, os dois pagam e o banco decide: quem chega depois recebe as
	// moedas de volta.
	r.list(ana, 1, 0, 0, 12) // espaço vazio: recusado
	ana.pl.inv.bag[1] = stack{def: game.Items["ring_good"], count: 1}
	r.list(ana, 1, 1, 400, 12)
	r2 := newAucRig(t, r.f)
	dora, pdora := r2.join(4, "Dora", 4, 1000)
	lb = r.search(beto, pbeto, nil)[0]
	ld := r2.search(dora, pdora, nil)[0]
	beto.pl.moveTokens, dora.pl.moveTokens = r.z.cfg.MaxMovesPerSec, r.z.cfg.MaxMovesPerSec
	r.z.handle(socialEvent{id: beto.id, msg: &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_BUY, NpcId: auctioneerID, AuctionId: lb.Id, Price: lb.Price}})
	r2.z.handle(socialEvent{id: dora.id, msg: &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_BUY, NpcId: auctioneerID, AuctionId: ld.Id, Price: ld.Price}})
	if beto.pl.inv.gold != 100 || dora.pl.inv.gold != 600 {
		t.Fatal("os dois pagaram na memória")
	}
	r.pump()  // Beto chega primeiro ao banco
	r2.pump() // Dora depois: reembolso, retirado na hora
	if beto.pl.inv.countOf("ring_good") != 1 || dora.pl.inv.countOf("ring_good") != 0 {
		t.Fatal("só um leva o anel")
	}
	if dora.pl.inv.gold != 1000 || !pdora.systemContains("Compra não concluída") {
		t.Fatalf("a Dora recebe as moedas de volta: %d", dora.pl.inv.gold)
	}
	if fl := r.f.flows(); fl["gold:"+HowAuctionReturn] != 400 {
		t.Fatalf("reembolso na telemetria: %v", fl)
	}
}

func TestAuctionBuyAtTheExpiryInstant(t *testing.T) {
	for _, expireFirst := range []bool{true, false} {
		r := newAucRig(t, nil)
		ana, pa := r.join(1, "Ana", 1, 100, game.ItemRecord{Slot: 0, ItemID: "ring_good", Count: 1})
		beto, pbeto := r.join(2, "Beto", 2, 1000)
		r.list(ana, 0, 1, 300, 12)
		l := r.search(beto, pbeto, nil)[0]
		*r.now = noon.Add(12*time.Hour - time.Millisecond) // o último milissegundo
		beto.pl.moveTokens = r.z.cfg.MaxMovesPerSec
		r.z.handle(socialEvent{id: beto.id, msg: &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_BUY, NpcId: auctioneerID, AuctionId: l.Id, Price: l.Price}})
		*r.now = noon.Add(12 * time.Hour) // venceu enquanto a compra ia para o banco
		if expireFirst {
			r.z.queueAuctionExpire()
			if fn := (<-r.z.auction.jobs)(t.Context(), r.f); fn != nil {
				r.z.handle(socialDone{fn: fn})
			}
		}
		r.pump()
		r.z.queueAuctionExpire()
		r.pump()
		// Em qualquer ordem, o anel está em um lugar só.
		mailsA := r.inbox(ana, pa)
		r.cmd(ana, &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_CLAIM})
		r.inbox(beto, pbeto)
		r.cmd(beto, &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_CLAIM})
		rings := ana.pl.inv.countOf("ring_good") + beto.pl.inv.countOf("ring_good")
		gold := ana.pl.inv.gold + beto.pl.inv.gold
		if rings != 1 {
			t.Fatalf("expireFirst=%v: %d anéis (cartas da Ana: %d)", expireFirst, rings, len(mailsA))
		}
		if expireFirst {
			if ana.pl.inv.countOf("ring_good") != 1 || beto.pl.inv.gold != 1000 {
				t.Fatal("venceu antes: o anel volta para a Ana e o Beto recebe as moedas")
			}
		} else if beto.pl.inv.countOf("ring_good") != 1 || gold != 100-AuctionFee(300)+1000-AuctionCommission(300) {
			t.Fatalf("a compra chegou antes: Beto leva (ouro total %d)", gold)
		}
	}
}

func TestAuctionCancelAndExpireReturnItemByMail(t *testing.T) {
	r := newAucRig(t, nil)
	ana, pa := r.join(1, "Ana", 1, 1000, game.ItemRecord{Slot: 0, ItemID: "potion_hp_s", Count: 10},
		game.ItemRecord{Slot: 1, ItemID: "ring_good", Count: 1})
	r.list(ana, 0, 6, 100, 12)
	r.list(ana, 1, 1, 900, 24)
	gold := ana.pl.inv.gold
	if gold != 1000-AuctionFee(100)-AuctionFee(900) {
		t.Fatalf("taxas: %d", gold)
	}
	mine := pa.lastAuction(func(u *pb.AuctionUpdate) bool { return u.MineListed }).Mine
	var potions *pb.AuctionListing
	for _, l := range mine {
		if l.ItemId == "potion_hp_s" {
			potions = l
		}
	}
	r.cmd(ana, &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_CANCEL, AuctionId: potions.Id})
	if !pa.systemContains("Anúncio cancelado") {
		t.Fatal("cancelar avisa")
	}
	r.cmd(ana, &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_CANCEL, AuctionId: potions.Id})
	if !pa.systemContains("Escolha um dos seus anúncios ativos") && !pa.systemContains("já foi vendido") {
		t.Fatal("cancelar duas vezes é recusado")
	}
	// O anel vence.
	*r.now = noon.Add(25 * time.Hour)
	r.z.queueAuctionExpire()
	r.pump()
	if !pa.systemContains("venceu") || len(r.active()) != 0 {
		t.Fatal("o anúncio vencido volta")
	}
	r.inbox(ana, pa)
	r.cmd(ana, &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_CLAIM})
	if ana.pl.inv.countOf("potion_hp_s") != 10 || ana.pl.inv.countOf("ring_good") != 1 || ana.pl.inv.gold != gold {
		t.Fatalf("tudo volta menos a taxa: %d poções, %d anéis, %d moedas", ana.pl.inv.countOf("potion_hp_s"),
			ana.pl.inv.countOf("ring_good"), ana.pl.inv.gold)
	}
	fl := r.f.flows()
	if fl["item_in:"+HowAuctionReturn] != 7 || fl["item_out:"+HowAuctionSale] != 7 {
		t.Fatalf("telemetria: %v", fl)
	}
}

func TestAuctionSellerOffline(t *testing.T) {
	r := newAucRig(t, nil)
	ana, _ := r.join(1, "Ana", 1, 100, game.ItemRecord{Slot: 0, ItemID: "ring_good", Count: 1})
	r.list(ana, 0, 1, 1000, 12)
	r.z.removePlayer(ana.id) // a Ana sai
	r.pump()
	beto, pbeto := r.join(2, "Beto", 2, 5000)
	r.buy(beto, r.search(beto, pbeto, nil)[0])
	if beto.pl.inv.countOf("ring_good") != 1 {
		t.Fatal("comprou com a vendedora offline")
	}
	ana, pa := r.join(1, "Ana", 1, 0) // volta (do estado recente)
	mails := r.inbox(ana, pa)
	if len(mails) != 1 || mails[0].Attachment.GetGold() != 1000-AuctionCommission(1000) {
		t.Fatalf("as moedas esperam no correio: %+v", mails)
	}
	gold := ana.pl.inv.gold
	r.cmd(ana, &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_CLAIM})
	if ana.pl.inv.gold != gold+1000-AuctionCommission(1000) {
		t.Fatal("retira as moedas")
	}
}

// A queda no meio: o comprador clica e cai antes de o banco gravar. O estado da saída leva
// a operação; quando ela é aplicada, a zona tira do estado recente; ao voltar, o item está
// no correio. Com o banco fora, a operação espera (nunca é esquecida) e vai uma vez só.
func TestAuctionBuyerDropsMidway(t *testing.T) {
	r := newAucRig(t, nil)
	ana, _ := r.join(1, "Ana", 1, 100, game.ItemRecord{Slot: 0, ItemID: "ring_good", Count: 1},
		game.ItemRecord{Slot: 1, ItemID: "potion_hp_s", Count: 3})
	beto, pbeto := r.join(2, "Beto", 2, 5000)
	r.list(ana, 0, 1, 1000, 12)
	r.list(ana, 1, 3, 100, 12)
	res := r.search(beto, pbeto, nil) // poções (100) e anel (1000)

	// 1) Cai logo depois do clique, com o banco no ar.
	beto.pl.moveTokens = r.z.cfg.MaxMovesPerSec
	r.z.handle(socialEvent{id: beto.id, msg: &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_BUY, NpcId: auctioneerID, AuctionId: res[1].Id, Price: res[1].Price}})
	r.z.removePlayer(beto.id)
	if len(r.z.recent[2].state.Auction) != 1 {
		t.Fatal("o estado da saída leva a operação")
	}
	r.pump()
	if len(r.z.recent[2].state.Auction) != 0 {
		t.Fatal("confirmada, sai do estado recente")
	}

	// 2) O banco cai: a compra das poções não grava; ele sai e volta (do estado recente).
	r.f.down = true
	beto, pbeto = r.join(2, "Beto", 2, 0)
	if beto.pl.inv.gold != 4000 {
		t.Fatalf("voltou com %d moedas (pagou 1000)", beto.pl.inv.gold)
	}
	r.search(beto, pbeto, nil)
	r.buy(beto, res[0])
	r.z.removePlayer(beto.id)
	*r.now = r.now.Add(time.Hour) // muito depois do recentKeep
	r.z.saveAll(false)
	r.pump()
	if st, ok := r.z.recent[2]; !ok || len(st.state.Auction) != 1 {
		t.Fatal("com operação sem confirmação, o estado recente não é esquecido")
	}
	r.f.down = false
	beto, pbeto = r.join(2, "Beto", 2, 0) // o salvamento da entrada leva a operação
	r.z.saveAll(false)
	r.pump()
	r.z.saveAll(false) // de novo: o banco ignora a repetida
	r.pump()
	if len(beto.pl.auc.ops) != 0 {
		t.Fatalf("operações pendentes: %+v", beto.pl.auc.ops)
	}
	r.inbox(beto, pbeto)
	r.cmd(beto, &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_CLAIM})
	if beto.pl.inv.countOf("ring_good") != 1 || beto.pl.inv.countOf("potion_hp_s") != 3 || beto.pl.inv.gold != 3900 {
		t.Fatalf("Beto: %d anéis, %d poções, %d moedas", beto.pl.inv.countOf("ring_good"), beto.pl.inv.countOf("potion_hp_s"), beto.pl.inv.gold)
	}
	if n := len(r.f.events); n == 0 {
		t.Fatal("sem telemetria")
	}
	buys := 0
	for _, e := range r.f.events {
		if e.Kind == "gold" && e.Ref == HowAuctionBuy {
			buys++
		}
	}
	if buys != 2 {
		t.Fatalf("%d compras na telemetria (quero 2: uma por compra, mesmo repetida)", buys)
	}
}

// O panic de um handler volta o jogador ao último estado salvo, que já levava a operação
// junto com a bolsa sem o item: o item fica em um lugar só (nada pela metade).
func TestAuctionUnsavedOpGoesAwayWithPanicRollback(t *testing.T) {
	r := newAucRig(t, nil)
	ana, _ := r.join(1, "Ana", 1, 100, game.ItemRecord{Slot: 0, ItemID: "ring_good", Count: 1})
	r.z.saveAll(false)
	r.pump()
	r.f.down = true // o salvamento do anúncio não grava
	r.list(ana, 0, 1, 300, 12)
	r.z.markBroken(ana.id) // um panic derruba a Ana antes do próximo salvamento
	last := r.z.errs.lastSaved[ana.id]
	r.z.removePlayer(ana.id)
	st := r.z.recent[ana.id].state
	if st.Seq != last.Seq {
		t.Fatal("o estado recente é o último salvo")
	}
	// O último salvo já levava a operação (ela foi junto com a bolsa sem o anel): o banco
	// aplica as duas coisas juntas quando voltar.
	r.f.down = false
	ana, _ = r.join(1, "Ana", 1, 0)
	r.z.saveAll(false)
	r.pump()
	hasRing := ana.pl.inv.countOf("ring_good") == 1
	listed := len(r.active()) == 1
	if hasRing == listed {
		t.Fatalf("o anel está na bolsa (%v) e no anúncio (%v)", hasRing, listed)
	}
}

func TestAuctionSearchFilters(t *testing.T) {
	r := newAucRig(t, nil)
	ana, _ := r.join(1, "Ana", 1, 10000, game.ItemRecord{Slot: 0, ItemID: "potion_hp_s", Count: 5},
		game.ItemRecord{Slot: 1, ItemID: "ring_good", Count: 1}, game.ItemRecord{Slot: 2, ItemID: "staff_common", Count: 1})
	beto, pbeto := r.join(2, "Beto", 2, 100)
	r.list(ana, 0, 5, 50, 12)
	r.list(ana, 1, 1, 700, 12)
	r.list(ana, 2, 1, 300, 12)
	names := func(l []*pb.AuctionListing) string {
		var s []string
		for _, x := range l {
			s = append(s, x.ItemId)
		}
		return strings.Join(s, ",")
	}
	if got := names(r.search(beto, pbeto, nil)); got != "potion_hp_s,staff_common,ring_good" {
		t.Fatalf("tudo, pelo preço: %s", got)
	}
	if got := names(r.search(beto, pbeto, &pb.AuctionQuery{MinPrice: 100, MaxPrice: 500})); got != "staff_common" {
		t.Fatalf("faixa de preço: %s", got)
	}
	if got := names(r.search(beto, pbeto, &pb.AuctionQuery{MyClass: true})); strings.Contains(got, "staff") {
		t.Fatalf("o cavaleiro não usa cajado: %s", got)
	}
	if got := names(r.search(beto, pbeto, &pb.AuctionQuery{Grade: uint32(game.GradeGood)})); got != "ring_good" {
		t.Fatalf("raridade: %s", got)
	}
	if got := names(r.search(beto, pbeto, &pb.AuctionQuery{Name: "POÇÃO"})); got != "potion_hp_s" {
		t.Fatalf("nome: %s", got)
	}
	if got := r.search(beto, pbeto, &pb.AuctionQuery{Name: "nada assim"}); len(got) != 0 {
		t.Fatal("nome que não existe")
	}
	// Comprar sem ter visto, com outro preço ou sem moedas.
	pbeto.reset()
	r.cmd(beto, &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_BUY, AuctionId: 999, Price: 1})
	if !pbeto.systemContains("não está mais na lista") {
		t.Fatal("anúncio que ele não viu")
	}
	l := r.search(beto, pbeto, &pb.AuctionQuery{Name: "anel"})[0]
	r.cmd(beto, &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_BUY, AuctionId: l.Id, Price: 1})
	if !pbeto.systemContains("preço deste anúncio mudou") {
		t.Fatal("preço diferente do anúncio")
	}
	r.buy(beto, l)
	if !pbeto.systemContains("Moedas insuficientes") || beto.pl.inv.gold != 100 {
		t.Fatal("sem moedas")
	}
	// O próprio anúncio.
	own := r.search(ana, r.peerOf(ana), nil)[0]
	r.buy(ana, own)
	if !r.peerOf(ana).systemContains("da sua conta") {
		t.Fatal("não compra o próprio anúncio")
	}
}

func (r *aucRig) peerOf(p *entity) *fakePeer { return p.pl.peer.(*fakePeer) }

func TestAuctionClosedWithoutBackend(t *testing.T) {
	z := newTestZone(nil)
	p := join(z, 1, "Ana")
	z.players[1].pl.moveTokens = z.cfg.MaxMovesPerSec
	z.handle(socialEvent{id: 1, msg: &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_OPEN, NpcId: auctioneerID}})
	if !p.systemContains("fechada") {
		t.Fatal("sem banco, a casa avisa que está fechada")
	}
}

// O fuzz semântico chega até a compra (e a contabilidade dele conta a custódia): Ana
// anuncia, Beto busca e compra, cada passo conferido pelo checkEconomy/checkState do fuzz.
func TestFuzzHarnessReachesAuction(t *testing.T) {
	open := &pb.ClientMessage{Msg: &pb.ClientMessage_AuctionCmd{AuctionCmd: &pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_OPEN, NpcId: fuzzAuctioneerID}}}
	h := newFuzzHarness(t, &fuzzReader{b: fuzzFixedSetup(fuzzSpotFor(open))})
	auc := func(m *pb.AuctionCmd) *pb.ClientMessage {
		m.NpcId = fuzzAuctioneerID
		return &pb.ClientMessage{Msg: &pb.ClientMessage_AuctionCmd{AuctionCmd: m}}
	}
	beto := h.z.players[h.ids[1]]
	beto.pl.inv.gold = 1000
	h.step(0, open, 1)
	h.step(0, auc(&pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_LIST, Slot: 9, Count: 3, Price: 100, Hours: 24}), 1)
	if len(h.auction.auctions) != 1 {
		t.Fatalf("a Ana devia ter anunciado: %v", h.trace)
	}
	h.step(1, open, 1)
	h.step(1, auc(&pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_SEARCH}), 20)
	before := beto.pl.inv.countOf("potion_hp_s")
	h.step(1, auc(&pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_BUY, AuctionId: 1, Price: 100}), 1)
	if beto.pl.inv.countOf("potion_hp_s") != before+3 || beto.pl.inv.gold != 900 {
		t.Fatalf("o Beto devia ter comprado: %d poções, %d moedas\n%s", beto.pl.inv.countOf("potion_hp_s"), beto.pl.inv.gold, strings.Join(h.trace, "\n"))
	}
	h.step(0, &pb.ClientMessage{Msg: &pb.ClientMessage_MailCmd{MailCmd: &pb.MailCmd{Kind: pb.MailCmdKind_MAIL_CMD_LIST}}}, 1)
	h.step(0, auc(&pb.AuctionCmd{Kind: pb.AuctionCmdKind_AUCTION_CMD_CLAIM}), 1)
	if g, u := h.auction.custody(h.ids[:]); g != 0 || len(u) != 0 {
		t.Fatalf("a custódia devia estar vazia: %d moedas, %v", g, u)
	}
}
