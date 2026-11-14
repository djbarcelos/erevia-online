package main

import (
	"context"
	"fmt"
	"math"
	"strings"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Casa de leilões no caos: anunciar, buscar, comprar (inclusive o mesmo anúncio que outro
// bot viu, e comprar e cair), cancelar, meus anúncios e retirar do correio, mais as
// variantes inválidas (preço fora da faixa, prazo, espaço, anúncio que não existe, de outro
// vendedor, NPC errado). A conferência da economia conta a custódia (auctionPool).

// aucSeen: um anúncio que o bot viu (busca ou meus anúncios).
type aucSeen struct {
	id     uint64
	price  int64
	seller string
	mine   bool
	active bool
}

func auctionNPC(n *npcSeen) bool { return n.role == pb.NpcRole_NPC_ROLE_AUCTION }

// onAuction guarda o que a janela mostraria (com s.mu travado).
func (s *chaosSession) onAuction(u *pb.AuctionUpdate) {
	if u.Searched {
		s.aucFound = s.aucFound[:0]
		for _, l := range u.Results {
			s.aucFound = append(s.aucFound, aucSeen{l.Id, l.Price, l.SellerName, l.Mine, l.Status == pb.AuctionStatus_AUCTION_STATUS_ACTIVE})
		}
	}
	if u.MineListed {
		s.aucMine = s.aucMine[:0]
		for _, l := range u.Mine {
			s.aucMine = append(s.aucMine, aucSeen{l.Id, l.Price, l.SellerName, true, l.Status == pb.AuctionStatus_AUCTION_STATUS_ACTIVE})
		}
	}
	if u.GoneId != 0 {
		for i, l := range s.aucFound {
			if l.id == u.GoneId {
				s.aucFound = append(s.aucFound[:i], s.aucFound[i+1:]...)
				break
			}
		}
	}
}

// onMailAttachments: as cartas com anexo ainda não retirado (com s.mu travado).
func (s *chaosSession) onMailAttachments(m *pb.MailBox) {
	s.attach = s.attach[:0]
	for _, ml := range m.Mails {
		if a := ml.Attachment; a != nil && !a.Claimed {
			s.attach = append(s.attach, ml.Id)
		}
	}
}

func auctionMsg(m *pb.AuctionCmd) *pb.ClientMessage {
	return &pb.ClientMessage{Msg: &pb.ClientMessage_AuctionCmd{AuctionCmd: m}}
}

func actAuction(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	npc := s.nearestNPC(auctionNPC)
	var msgs []*pb.ClientMessage
	label := ""
	drop := 0 // 2 = cai abrupto depois, 3 = reloga por cima
	far := true
	if n := s.npcs[npc]; n != nil {
		far = math.Hypot(n.pos.x-s.pos.x, n.pos.z-s.pos.z) > 5
		if far && pct(r[5]) < 60 { // vai até o leiloeiro (anunciar, comprar e cancelar exigem estar perto)
			s.mu.Unlock()
			b.emit(s, "leilao:ir", &pb.ClientMessage{Msg: &pb.ClientMessage_MoveTo{MoveTo: &pb.MoveTo{
				X: float32(n.pos.x + float64(pickN(r[3], 30))/10 - 1.5), Z: float32(n.pos.z + float64(pickN(r[4], 30))/10 - 1.5)}}})
			return
		}
	}
	m := &pb.AuctionCmd{NpcId: npc}
	prefix := b.run.cfg.prefix
	switch p := pct(r[1]); {
	case p < 8:
		m.Kind, label = pb.AuctionCmdKind_AUCTION_CMD_OPEN, "leilao:abrir"
	case p < 22:
		m.Kind, label = pb.AuctionCmdKind_AUCTION_CMD_SEARCH, "leilao:buscar"
		q := &pb.AuctionQuery{Sort: pb.AuctionSort(pickN(r[2], 4)), Page: uint32(pickN(r[3], 3) / 2)}
		switch pickN(r[4], 6) {
		case 0:
			q.Name = []string{"poção", "POCAO", "anel", "espada", "osso", "xyz"}[pickN(r[5], 6)]
		case 1:
			q.Kind = pb.ItemKind(1 + pickN(r[5], 15))
		case 2:
			q.Grade = uint32(1 + pickN(r[5], 4))
		case 3:
			q.MyClass = true
		case 4:
			q.MinPrice, q.MaxPrice = 10, int64(100+pickN(r[5], 2000))
		}
		m.Query = q
	case p < 44:
		m.Kind, label = pb.AuctionCmdKind_AUCTION_CMD_LIST, "leilao:anunciar"
		m.Slot = s.someBag(r[2], func(it slotItem) bool { return it.id != "" })
		m.Count = []uint32{0, 0, 1, 2}[pickN(r[3], 4)]
		m.Price = int64(10 + pickN(r[4], 600))
		m.Hours = []uint32{12, 24, 48}[pickN(r[5], 3)]
		if bad(r[5], 15) {
			m.Price = []int64{0, 9, -5, game.MaxStack, 100_000_001, math.MaxInt64}[pickN(r[4], 6)]
			m.Hours = []uint32{0, 1, 13, 49, math.MaxUint32, 24}[pickN(r[3], 6)]
			m.Slot = []uint32{m.Slot, game.BagSize, 255, math.MaxUint32}[pickN(r[2], 4)]
			label = "leilao!anunciar"
		}
	case p < 66:
		m.Kind, label = pb.AuctionCmdKind_AUCTION_CMD_BUY, "leilao:comprar"
		// O mais barato de outro bot desta rodada (os bots convergem: dois compram o mesmo).
		var pick *aucSeen
		for i := range s.aucFound {
			l := &s.aucFound[i]
			if l.active && !l.mine && strings.HasPrefix(l.seller, prefix) && (pick == nil || pickN(r[2], 4) == 0) {
				pick = l
			}
		}
		switch {
		case pick == nil:
			// Sem nada visto: busca primeiro (no mesmo passo).
			m.Kind, m.Query, label = pb.AuctionCmdKind_AUCTION_CMD_SEARCH, &pb.AuctionQuery{}, "leilao:buscar_para_comprar"
		case bad(r[3], 12):
			m.AuctionId, m.Price, label = pick.id, pick.price-1, "leilao!comprar_preco"
		default:
			m.AuctionId, m.Price = pick.id, pick.price
			if pct(r[4]) < 15 { // compra e cai antes da resposta (a queda no meio)
				drop, label = 2+pickN(r[5], 2), "leilao:comprar_e_cair"
			}
		}
	case p < 74:
		m.Kind, label = pb.AuctionCmdKind_AUCTION_CMD_CANCEL, "leilao:cancelar"
		var ids []uint64
		for _, l := range s.aucMine {
			if l.active {
				ids = append(ids, l.id)
			}
		}
		if len(ids) > 0 {
			m.AuctionId = ids[pickN(r[2], len(ids))]
		} else {
			m.AuctionId, label = uint64(1+pickN(r[2], 1000)), "leilao!cancelar_de_outro"
		}
	case p < 80:
		m.Kind, label = pb.AuctionCmdKind_AUCTION_CMD_MINE, "leilao:meus"
	case p < 93:
		// Retira do correio (às vezes lê a caixa antes, na mesma rajada).
		m.Kind, label = pb.AuctionCmdKind_AUCTION_CMD_CLAIM, "leilao:retirar_tudo"
		if len(s.attach) > 0 && pct(r[3]) < 60 {
			m.MailId, label = s.attach[pickN(r[2], len(s.attach))], "leilao:retirar"
		}
		if pct(r[4]) < 40 {
			msgs = append(msgs, &pb.ClientMessage{Msg: &pb.ClientMessage_MailCmd{MailCmd: &pb.MailCmd{Kind: pb.MailCmdKind_MAIL_CMD_LIST}}})
		}
	case p < 97: // anuncia e cai logo (o item sai da bolsa e vai junto do salvamento da saída)
		m.Kind, label = pb.AuctionCmdKind_AUCTION_CMD_LIST, "leilao:anunciar_e_cair"
		m.Slot = s.someBag(r[2], func(it slotItem) bool { return it.id != "" })
		m.Price, m.Hours = int64(10+pickN(r[4], 300)), 12
		drop = 2 + pickN(r[5], 2)
	default:
		m.Kind = pb.AuctionCmdKind(pickN(r[2], 10))
		m.AuctionId = []uint64{0, math.MaxUint64, 1 << 63, 7}[pickN(r[3], 4)]
		m.MailId = []uint64{0, math.MaxUint64, 3}[pickN(r[4], 3)]
		m.Price = []int64{math.MinInt64, -1, 0, math.MaxInt64}[pickN(r[5], 4)]
		m.Count, m.Slot = badU32(badCounts, r[3]), badU32(badSlots, r[4])
		if pct(r[5]) < 50 {
			m.NpcId = s.nearestNPC(shopNPC) // NPC errado
		}
		label = "leilao!lixo"
	}
	s.mu.Unlock()
	if far && m.Kind != pb.AuctionCmdKind_AUCTION_CMD_CLAIM && m.Kind != pb.AuctionCmdKind_AUCTION_CMD_MINE {
		label += "!longe"
	}
	b.emit(s, label, append(msgs, auctionMsg(m))...)
	switch drop {
	case 2:
		s.close(true, true)
		b.offline = 1 + pickN(r[5], 6)
	case 3:
		b.relogOver(s)
	}
}

// ---------- Conferência: a custódia ----------

// auctionPool: o que a telemetria diz que entrou e saiu da casa de leilões contra o que o
// banco tem nela (anúncios ativos e anexos não retirados) para os personagens da rodada.
type auctionPool struct {
	// Telemetria: itens que saíram das bolsas para a casa (auction_sale) e que voltaram
	// dela (auction_buy, auction_return); moedas que entraram nela (compras) e saíram
	// (venda, reembolso, devolução da taxa).
	itemsOut, itemsIn map[string]int64
	goldIn, goldOut   int64
	// Banco.
	custody    map[string]int64 // anúncios ativos
	unclaimed  map[string]int64 // anexos de item não retirados
	goldMail   int64            // anexos de moedas não retirados
	commission int64            // comissão das vendas (sai do jogo)
	feeBack    int64            // taxas devolvidas com o item (anúncio recusado pelo limite)
	listed     int64            // anúncios criados
	sold       int64
}

func (d *chaosDB) auctionPool(ctx context.Context, ids []int64, mark int64) (auctionPool, error) {
	p := auctionPool{itemsOut: map[string]int64{}, itemsIn: map[string]int64{}, custody: map[string]int64{}, unclaimed: map[string]int64{}}
	rows, err := d.pool.Query(ctx, `
		SELECT kind, ref, coalesce(extra->>'how', ''), sum(amount)::bigint FROM game_events
		WHERE id > $1 AND character_id = ANY($2) AND kind IN ('gold', 'item_in', 'item_out')
			AND (ref LIKE 'auction_%' OR extra->>'how' LIKE 'auction_%')
		GROUP BY 1, 2, 3`, mark, ids)
	if err != nil {
		return p, err
	}
	for rows.Next() {
		var kind, ref, how string
		var n int64
		if err := rows.Scan(&kind, &ref, &how, &n); err != nil {
			rows.Close()
			return p, err
		}
		switch {
		case kind == "gold" && ref == "auction_buy":
			p.goldIn += -n
		case kind == "gold" && (ref == "auction_sale" || ref == "auction_return"):
			p.goldOut += n
		case kind == "item_out" && how == "auction_sale":
			p.itemsOut[ref] += n
		case kind == "item_in" && (how == "auction_buy" || how == "auction_return"):
			p.itemsIn[ref] += n
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return p, err
	}
	rows, err = d.pool.Query(ctx, `SELECT item_id, sum(count)::bigint FROM auctions WHERE seller_id = ANY($1) AND status = 'active' GROUP BY 1`, ids)
	if err != nil {
		return p, err
	}
	for rows.Next() {
		var id string
		var n int64
		if err := rows.Scan(&id, &n); err != nil {
			rows.Close()
			return p, err
		}
		p.custody[id] = n
	}
	rows.Close()
	rows, err = d.pool.Query(ctx, `SELECT item_id, sum(count)::bigint, sum(gold)::bigint FROM mail_attachments
		WHERE to_id = ANY($1) AND claimed_at IS NULL GROUP BY 1`, ids)
	if err != nil {
		return p, err
	}
	for rows.Next() {
		var id string
		var n, g int64
		if err := rows.Scan(&id, &n, &g); err != nil {
			rows.Close()
			return p, err
		}
		if id != "" {
			p.unclaimed[id] += n
		}
		p.goldMail += g
	}
	rows.Close()
	if err := d.pool.QueryRow(ctx, `SELECT coalesce(sum(commission), 0)::bigint, count(*) FILTER (WHERE status = 'sold'), count(*)
		FROM auctions WHERE seller_id = ANY($1)`, ids).Scan(&p.commission, &p.sold, &p.listed); err != nil {
		return p, err
	}
	err = d.pool.QueryRow(ctx, `SELECT coalesce(sum(gold), 0)::bigint FROM mail_attachments
		WHERE to_id = ANY($1) AND item_id <> '' AND gold > 0`, ids).Scan(&p.feeBack)
	return p, err
}

// printAuctionPool imprime a conferência da custódia e devolve quantas linhas não fecham.
func (r *chaosReport) printAuctionPool() int {
	p := r.pool
	if r.poolErr != nil {
		fmt.Printf("casa de leilões: não deu para conferir: %v\n", r.poolErr)
		return 1
	}
	bad := 0
	fmt.Printf("casa de leilões: %d anúncios, %d vendidos, comissão %d, %d moedas em anexos\n", p.listed, p.sold, p.commission, p.goldMail)
	// Moedas: entraram (compras + taxas devolvidas) − saíram (vendas, reembolsos) = anexos + comissão.
	in, want := p.goldIn+p.feeBack-p.goldOut, p.goldMail+p.commission
	fmt.Printf("  moedas: compras %d + taxas devolvidas %d − retiradas %d = %d; anexos %d + comissão %d = %d", p.goldIn, p.feeBack, p.goldOut, in, p.goldMail, p.commission, want)
	if in != want {
		fmt.Printf("  <-- NÃO FECHA (diferença %+d)\n", in-want)
		bad++
	} else {
		fmt.Println("  ok")
	}
	keys := map[string]bool{}
	for _, m := range []map[string]int64{p.itemsOut, p.itemsIn, p.custody, p.unclaimed} {
		for k := range m {
			keys[k] = true
		}
	}
	ok := 0
	for _, k := range sortedKeys(keys) {
		if p.itemsOut[k]-p.itemsIn[k] == p.custody[k]+p.unclaimed[k] {
			ok++
			continue
		}
		bad++
		fmt.Printf("  item %-22s anunciado %5d − retirado %5d = %5d; em custódia %5d + no correio %5d  <-- NÃO FECHA\n",
			k, p.itemsOut[k], p.itemsIn[k], p.itemsOut[k]-p.itemsIn[k], p.custody[k], p.unclaimed[k])
	}
	fmt.Printf("  itens: %d tipos na custódia conferidos, %d fecham\n", len(keys), ok)
	return bad
}
