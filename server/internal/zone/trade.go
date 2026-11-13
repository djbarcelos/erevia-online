package zone

import (
	"fmt"
	"slices"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Troca entre jogadores. Ninguém perde nem ganha nada até a execução: as ofertas só
// apontam para espaços da bolsa. Mudar uma oferta (ou a bolsa de quem troca) destrava
// as duas. Com as duas travadas e confirmadas, a execução confere tudo de novo, monta
// as bolsas novas em cópias e só então aplica, salvando os dois na mesma transação.

const (
	tradeRange    = 8.0 // metros: mais longe que isso, a troca é cancelada
	tradeMaxItems = 8   // itens diferentes por oferta
)

type tradeItem struct {
	slot    int
	def     *game.ItemDef
	count   int
	enchant int
}

type tradeOffer struct {
	items     []tradeItem
	gold      int64
	locked    bool
	confirmed bool
}

type trade struct {
	p     [2]*entity
	offer [2]tradeOffer
}

func (t *trade) side(p *entity) int {
	if t.p[0] == p {
		return 0
	}
	return 1
}

func (t *trade) unlock() {
	for i := range t.offer {
		t.offer[i].locked, t.offer[i].confirmed = false, false
	}
}

type tradeEvent struct {
	id  uint32
	msg *pb.TradeCmd
}

func (tradeEvent) isEvent() {}

func (z *Zone) onTrade(e tradeEvent) {
	p := z.players[e.id]
	if p == nil || !z.spendAction(p) {
		return
	}
	m := e.msg
	if m.Kind == pb.TradeCmdKind_TRADE_CMD_REQUEST {
		z.tradeRequest(p, z.players[m.TargetId])
		return
	}
	t := p.pl.trade
	if t == nil {
		return
	}
	me := &t.offer[t.side(p)]
	switch m.Kind {
	case pb.TradeCmdKind_TRADE_CMD_ADD_ITEM:
		z.tradeAdd(p, t, int(m.Slot), int(m.Count))
	case pb.TradeCmdKind_TRADE_CMD_REMOVE_ITEM:
		if i := slices.IndexFunc(me.items, func(it tradeItem) bool { return it.slot == int(m.Slot) }); i >= 0 {
			me.items = slices.Delete(me.items, i, i+1)
			t.unlock()
			z.sendTrade(t)
		}
	case pb.TradeCmdKind_TRADE_CMD_SET_GOLD:
		g := min(max(m.Gold, 0), p.pl.inv.gold)
		if g != me.gold {
			me.gold = g
			t.unlock()
			z.sendTrade(t)
		}
	case pb.TradeCmdKind_TRADE_CMD_LOCK:
		me.locked = true
		z.sendTrade(t)
	case pb.TradeCmdKind_TRADE_CMD_CONFIRM:
		if !t.offer[0].locked || !t.offer[1].locked {
			z.system(p, "As duas ofertas precisam estar travadas.")
			return
		}
		me.confirmed = true
		if t.offer[0].confirmed && t.offer[1].confirmed {
			z.executeTrade(t)
			return
		}
		z.sendTrade(t)
	case pb.TradeCmdKind_TRADE_CMD_CANCEL:
		z.closeTrade(t, p.name+" cancelou a troca.")
	}
}

// tradeBlocked diz por que p não pode trocar agora ("" = pode).
func (z *Zone) tradeBlocked(p *entity) string {
	switch {
	case p.dead:
		return "morto"
	case p.pl.trade != nil:
		return "já está trocando"
	case p.pl.channel != nil:
		return "viajando"
	}
	return ""
}

func (z *Zone) tradeRequest(p, t *entity) {
	switch {
	case t == nil:
		z.system(p, "Esse jogador não está online.")
	case t == p:
		z.system(p, "Você não pode trocar consigo mesmo.")
	case dist(p.pos, t.pos) > tradeRange:
		z.system(p, fmt.Sprintf("Chegue mais perto de %s para trocar.", t.name))
	case z.tradeBlocked(p) != "":
		z.system(p, "Você não pode trocar agora ("+z.tradeBlocked(p)+").")
	case z.tradeBlocked(t) != "":
		z.system(p, fmt.Sprintf("%s não pode trocar agora (%s).", t.name, z.tradeBlocked(t)))
	case z.pendingInviteTo(t.id) != nil:
		z.system(p, fmt.Sprintf("%s já tem um convite pendente.", t.name))
	default:
		z.sendInvite(p, t, pb.InviteKind_INVITE_KIND_TRADE, "", 0)
	}
}

// openTrade: o convite foi aceito.
func (z *Zone) openTrade(a, b *entity) {
	switch {
	case dist(a.pos, b.pos) > tradeRange:
		z.system(b, "Vocês estão longe demais para trocar.")
		z.system(a, "Vocês estão longe demais para trocar.")
		return
	case z.tradeBlocked(a) != "" || z.tradeBlocked(b) != "":
		z.system(b, "A troca não pode começar agora.")
		return
	}
	t := &trade{p: [2]*entity{a, b}}
	a.pl.trade, b.pl.trade = t, t
	z.sendTrade(t)
}

func (z *Zone) tradeAdd(p *entity, t *trade, slot, count int) {
	me := &t.offer[t.side(p)]
	if slot < 0 || slot >= len(p.pl.inv.bag) || p.pl.inv.bag[slot].empty() {
		return
	}
	s := p.pl.inv.bag[slot]
	if count <= 0 || count > s.count {
		count = s.count
	}
	i := slices.IndexFunc(me.items, func(it tradeItem) bool { return it.slot == slot })
	if i < 0 && len(me.items) >= tradeMaxItems {
		z.system(p, fmt.Sprintf("No máximo %d itens por troca.", tradeMaxItems))
		return
	}
	it := tradeItem{slot: slot, def: s.def, count: count, enchant: s.enchant}
	if i >= 0 {
		me.items[i] = it
	} else {
		me.items = append(me.items, it)
	}
	t.unlock()
	z.sendTrade(t)
}

// tradeInventoryChanged: a bolsa de quem troca mudou (poção, item pego, venda…).
// Itens da oferta que não estão mais lá saem dela, e as ofertas destravam.
func (z *Zone) tradeInventoryChanged(p *entity) {
	t := p.pl.trade
	if t == nil {
		return
	}
	me := &t.offer[t.side(p)]
	n := len(me.items)
	me.items = slices.DeleteFunc(me.items, func(it tradeItem) bool { return !offerStillThere(&p.pl.inv, it) })
	if me.gold > p.pl.inv.gold {
		me.gold = p.pl.inv.gold
		n = -1
	}
	if len(me.items) != n {
		t.unlock()
		z.sendTrade(t)
	}
}

func offerStillThere(inv *inventory, it tradeItem) bool {
	s := inv.bag[it.slot]
	return !s.empty() && s.def == it.def && s.enchant == it.enchant && s.count >= it.count
}

// executeTrade confere as duas ofertas, monta as bolsas novas em cópias e aplica tudo
// de uma vez. Qualquer problema cancela sem mexer em nada.
func (z *Zone) executeTrade(t *trade) {
	a, b := t.p[0], t.p[1]
	if dist(a.pos, b.pos) > tradeRange || a.dead || b.dead {
		z.closeTrade(t, "A troca foi cancelada.")
		return
	}
	invs := [2]inventory{a.pl.inv, b.pl.inv} // cópias (arrays por valor)
	for side := range 2 {
		inv, o := &invs[side], t.offer[side]
		if o.gold > inv.gold {
			z.closeTrade(t, "A troca foi cancelada: a oferta mudou.")
			return
		}
		for _, it := range o.items {
			if !offerStillThere(inv, it) || !inv.take(it.slot, it.count) {
				z.closeTrade(t, "A troca foi cancelada: a oferta mudou.")
				return
			}
		}
		inv.gold -= o.gold
	}
	for side := range 2 {
		inv, o := &invs[1-side], t.offer[side]
		for _, it := range o.items {
			if inv.put(it.def, it.count, it.enchant) > 0 {
				z.closeTrade(t, fmt.Sprintf("A troca foi cancelada: a bolsa de %s não tem espaço.", t.p[1-side].name))
				return
			}
		}
		inv.gold += o.gold
	}
	a.pl.inv, b.pl.inv = invs[0], invs[1]
	a.pl.trade, b.pl.trade = nil, nil
	for _, p := range t.p {
		z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Trade{Trade: &pb.TradeState{}}})
		z.system(p, "Troca concluída.")
		z.inventoryChanged(p, false)
		z.achEvent(p, game.AchTrade, "")
	}
	// Os dois na mesma leva = mesma transação no banco: a troca nunca fica salva pela metade.
	z.save([]SavedState{z.savedState(a), z.savedState(b)}, false)
	z.log.Info("trade", "a", a.name, "b", b.name, "a_items", len(t.offer[0].items), "a_gold", t.offer[0].gold,
		"b_items", len(t.offer[1].items), "b_gold", t.offer[1].gold)
}

// closeTrade fecha a janela dos dois, sem mexer em nenhum item.
func (z *Zone) closeTrade(t *trade, reason string) {
	for _, p := range t.p {
		if p.pl.trade != t {
			continue
		}
		p.pl.trade = nil
		z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Trade{Trade: &pb.TradeState{}}})
		if reason != "" {
			z.system(p, reason)
		}
	}
}

// checkTrades: 1×/s, cancela trocas de quem se afastou, morreu ou começou a viajar.
func (z *Zone) checkTrades() {
	for _, p := range z.players {
		t := p.pl.trade
		if t == nil || t.p[0] != p {
			continue
		}
		a, b := t.p[0], t.p[1]
		if a.dead || b.dead || a.pl.channel != nil || b.pl.channel != nil || dist(a.pos, b.pos) > tradeRange {
			z.closeTrade(t, "A troca foi cancelada: vocês se afastaram.")
		}
	}
}

func (z *Zone) sendTrade(t *trade) {
	for side, p := range t.p {
		z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Trade{Trade: &pb.TradeState{
			Open: true, PartnerId: t.p[1-side].id, PartnerName: t.p[1-side].name,
			Mine: offerPB(t.offer[side]), Theirs: offerPB(t.offer[1-side]),
		}}})
	}
}

func offerPB(o tradeOffer) *pb.TradeOffer {
	out := &pb.TradeOffer{Gold: o.gold, Locked: o.locked, Confirmed: o.confirmed}
	for _, it := range o.items {
		out.Items = append(out.Items, &pb.TradeItem{Slot: uint32(it.slot), ItemId: it.def.ID, Count: uint32(it.count), Enchant: uint32(it.enchant)})
	}
	return out
}
