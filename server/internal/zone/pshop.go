package zone

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Loja pessoal (tunnel#161). O vendedor abre a loja parado na área do mercado: os itens
// continuam na bolsa dele, a loja só aponta para os espaços (como a troca). Comprar confere
// tudo de novo, monta as duas bolsas em cópias e só então aplica, salvando vendedor e
// comprador na mesma leva (a mesma transação no banco): uma queda no meio nunca duplica
// nem some com nada. Andar, morrer, viajar ou sair fecha a loja.

type pshopItem struct {
	slot    int
	def     *game.ItemDef
	count   int
	enchant int
	price   int64
}

type privateShop struct {
	title   string
	items   []pshopItem
	pos     Vec2
	viewers map[uint32]bool // quem está com a janela desta loja aberta
}

// pshopMoveTolerance: o vendedor saiu do lugar (andou) e a loja fecha.
const pshopMoveTolerance = 0.5

func (z *Zone) onPrivateShopCmd(p *entity, m *pb.PrivateShopCmd) {
	switch m.Kind {
	case pb.PrivateShopCmdKind_PRIVATE_SHOP_CMD_OPEN:
		z.pshopOpen(p, m.Title, m.Items)
	case pb.PrivateShopCmdKind_PRIVATE_SHOP_CMD_CLOSE:
		if p.pl.pshop != nil {
			z.pshopClose(p, "Você fechou a loja.")
		}
	case pb.PrivateShopCmdKind_PRIVATE_SHOP_CMD_VIEW:
		z.pshopView(p, m.SellerId)
	case pb.PrivateShopCmdKind_PRIVATE_SHOP_CMD_BUY:
		z.pshopBuy(p, m.SellerId, int(m.Slot), int(m.Count), int64(min(m.Price, math.MaxInt64)))
	}
}

// pshopBlocked diz por que p não pode abrir a loja aqui ("" = pode).
func (z *Zone) pshopBlocked(p *entity) string {
	switch {
	case p.dead:
		return "Você está morto."
	case z.cfg.NoTown:
		return "A loja pessoal só abre no mercado de Vel'Aran."
	case p.pl.trade != nil:
		return "Termine a troca antes de abrir a loja."
	case p.pl.channel != nil:
		return "Você está viajando."
	case p.pl.pshop != nil:
		return "Sua loja já está aberta. Feche antes de montar outra."
	case !game.InPrivateShopArea(p.pos.X, p.pos.Z):
		return "A loja pessoal só abre na área do mercado, entre a praça e o prédio do Mercado."
	}
	open := 0
	for _, o := range z.players {
		if o == p || o.pl.pshop == nil {
			continue
		}
		open++
		if dist(o.pos, p.pos) < game.PrivateShopSpacing {
			return fmt.Sprintf("Muito perto da loja de %s: afaste-se um pouco.", o.name)
		}
	}
	if open >= game.PrivateShopMax {
		return fmt.Sprintf("O mercado está cheio (%d lojas). Tente mais tarde.", game.PrivateShopMax)
	}
	return ""
}

func (z *Zone) pshopOpen(p *entity, title string, items []*pb.PrivateShopItem) {
	if why := z.pshopBlocked(p); why != "" {
		z.system(p, why)
		return
	}
	if len(items) == 0 {
		z.system(p, "Escolha pelo menos um item para vender.")
		return
	}
	if len(items) > game.PrivateShopItems {
		z.system(p, fmt.Sprintf("No máximo %d itens por loja.", game.PrivateShopItems))
		return
	}
	shop := &privateShop{viewers: map[uint32]bool{}}
	seen := map[int]bool{}
	for _, it := range items {
		slot := int(it.Slot)
		if slot < 0 || slot >= game.BagSize || seen[slot] {
			z.system(p, "Item inválido na loja.")
			return
		}
		seen[slot] = true
		s := p.pl.inv.bag[slot]
		switch {
		case s.empty() || !game.PrivateShopSellable(s.def):
			z.system(p, "Item inválido na loja.")
			return
		case it.Price < 1 || it.Price > game.PrivateShopMaxPrice:
			z.system(p, fmt.Sprintf("O preço de %s precisa ser de 1 a %d moedas.", s.def.Name, game.PrivateShopMaxPrice))
			return
		}
		count := int(it.Count)
		if count <= 0 || count > s.count {
			count = s.count
		}
		shop.items = append(shop.items, pshopItem{slot: slot, def: s.def, count: count, enchant: s.enchant, price: int64(it.Price)})
	}
	shop.title = pshopTitle(p, title)
	p.pl.pathGoal = nil
	p.stopMoving()
	p.clearAction()
	shop.pos = p.pos
	p.pl.pshop = shop
	z.broadcastStatus(p)
	z.sendTo(p, z.pshopViewMsg(p, true))
	z.system(p, "Sua loja está aberta. Andar fecha a loja.")
	z.log.Info("pshop_open", "id", p.id, "items", len(shop.items))
}

// pshopTitle: a placa limpa ("Loja de Fulano" se vier vazia).
func pshopTitle(p *entity, title string) string {
	t := strings.TrimSpace(SanitizeChat(title))
	if r := []rune(t); len(r) > game.PrivateShopTitleLen {
		t = strings.TrimSpace(string(r[:game.PrivateShopTitleLen]))
	}
	if t == "" {
		t = "Loja de " + p.name
	}
	return t
}

// pshopClose fecha a loja de p, avisa quem estava olhando e mostra a todos que a placa saiu.
func (z *Zone) pshopClose(p *entity, reason string) {
	shop := p.pl.pshop
	if shop == nil {
		return
	}
	p.pl.pshop = nil
	closed := &pb.ServerMessage{Msg: &pb.ServerMessage_PrivateShop{PrivateShop: &pb.PrivateShopView{SellerId: p.id, SellerName: p.name}}}
	for id := range shop.viewers {
		if v := z.players[id]; v != nil && v != p {
			z.sendTo(v, closed)
		}
	}
	if z.players[p.id] == p {
		z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_PrivateShop{PrivateShop: &pb.PrivateShopView{SellerId: p.id, SellerName: p.name, Mine: true}}})
		if reason != "" {
			z.system(p, reason)
		}
	}
	z.broadcastStatus(p)
}

func (z *Zone) pshopView(p *entity, sellerID uint32) {
	s := z.players[sellerID]
	switch {
	case s == nil || s.pl.pshop == nil:
		z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_PrivateShop{PrivateShop: &pb.PrivateShopView{SellerId: sellerID}}})
		return
	case s != p && dist(s.pos, p.pos) > game.PrivateShopRange:
		z.system(p, fmt.Sprintf("Chegue mais perto da loja de %s.", s.name))
		return
	}
	if s != p {
		s.pl.pshop.viewers[p.id] = true
	}
	z.sendTo(p, z.pshopViewMsg(s, s == p))
}

func (z *Zone) pshopViewMsg(s *entity, mine bool) *pb.ServerMessage {
	shop := s.pl.pshop
	v := &pb.PrivateShopView{SellerId: s.id, SellerName: s.name, Mine: mine}
	if shop != nil {
		v.Open, v.Title = true, shop.title
		for _, it := range shop.items {
			v.Items = append(v.Items, &pb.PrivateShopItem{Slot: uint32(it.slot), ItemId: it.def.ID, Count: uint32(it.count),
				Enchant: uint32(it.enchant), Price: uint64(it.price)})
		}
	}
	return &pb.ServerMessage{Msg: &pb.ServerMessage_PrivateShop{PrivateShop: v}}
}

// pshopRefresh manda a loja de novo para o dono e para quem está olhando.
func (z *Zone) pshopRefresh(s *entity) {
	shop := s.pl.pshop
	if shop == nil {
		return
	}
	z.sendTo(s, z.pshopViewMsg(s, true))
	msg := z.pshopViewMsg(s, false)
	for _, id := range slices.Sorted(maps.Keys(shop.viewers)) {
		v := z.players[id]
		if v == nil || dist(v.pos, s.pos) > game.PrivateShopRange*2 {
			delete(shop.viewers, id)
			continue
		}
		z.sendTo(v, msg)
	}
}

// pshopBuy: compra count unidades do item no espaço slot da loja de sellerID, ao preço que o
// comprador viu. Tudo em cópias; qualquer problema recusa sem mexer em nada.
func (z *Zone) pshopBuy(p *entity, sellerID uint32, slot, count int, price int64) {
	s := z.players[sellerID]
	switch {
	case p.dead:
		return
	case s == nil || s.pl.pshop == nil:
		z.system(p, "Essa loja fechou.")
		return
	case s == p:
		return
	case dist(s.pos, p.pos) > game.PrivateShopRange:
		z.system(p, fmt.Sprintf("Chegue mais perto da loja de %s.", s.name))
		return
	case p.pl.trade != nil:
		z.system(p, "Termine a troca antes de comprar.")
		return
	}
	shop := s.pl.pshop
	i := slices.IndexFunc(shop.items, func(it pshopItem) bool { return it.slot == slot })
	if i < 0 {
		z.system(p, "Esse item já foi vendido.")
		z.sendTo(p, z.pshopViewMsg(s, false))
		return
	}
	line := shop.items[i]
	if price != line.price {
		z.system(p, "O preço mudou. Confira a loja de novo.")
		z.sendTo(p, z.pshopViewMsg(s, false))
		return
	}
	if count <= 0 || count > line.count {
		z.system(p, fmt.Sprintf("Só há %d à venda.", line.count))
		return
	}
	total := line.price * int64(count) // preço ≤ 1e9 e quantidade ≤ pilha: não estoura
	sinv, binv := s.pl.inv, p.pl.inv   // cópias (arrays por valor)
	switch {
	case binv.gold < total:
		z.system(p, fmt.Sprintf("Faltam %d moedas.", total-binv.gold))
		return
	case !offerStillThere(&sinv, tradeItem{slot: line.slot, def: line.def, count: count, enchant: line.enchant}) || !sinv.take(line.slot, count):
		z.pshopValidate(s)
		z.system(p, "Esse item não está mais à venda.")
		return
	}
	if binv.put(line.def, count, line.enchant) > 0 {
		z.system(p, "Sua bolsa está cheia.")
		return
	}
	binv.gold -= total
	sinv.gold += total
	s.pl.inv, p.pl.inv = sinv, binv
	shop.items[i].count -= count
	if shop.items[i].count <= 0 {
		shop.items = slices.Delete(shop.items, i, i+1)
	}
	label := itemLabelEnchant(line.def, count, line.enchant)
	z.system(p, fmt.Sprintf("Você comprou %s de %s por %d moedas.", label, s.name, total))
	z.system(s, fmt.Sprintf("%s comprou %s por %d moedas.", p.name, label, total))
	shop.viewers[p.id] = true
	z.inventoryChanged(p, false)
	z.inventoryChanged(s, false) // revalida a loja (pshopValidate)
	if s.pl.pshop != nil && len(s.pl.pshop.items) == 0 {
		z.pshopClose(s, "Você vendeu tudo: a loja fechou.")
	} else {
		z.pshopRefresh(s)
	}
	// Os dois na mesma leva = mesma transação no banco: a compra nunca fica salva pela metade.
	z.save([]SavedState{z.savedState(s), z.savedState(p)}, false)
	z.log.Info("pshop_buy", "seller", s.name, "buyer", p.name, "item", line.def.ID, "count", count, "gold", total)
}

// pshopValidate: a bolsa do vendedor mudou (usou, vendeu, guardou…). Linhas cujo item não
// está mais lá saem, e as quantidades descem ao que sobrou. Sem nada à venda, a loja fecha.
func (z *Zone) pshopValidate(p *entity) {
	shop := p.pl.pshop
	if shop == nil {
		return
	}
	n := len(shop.items)
	shop.items = slices.DeleteFunc(shop.items, func(it pshopItem) bool {
		s := p.pl.inv.bag[it.slot]
		return s.empty() || s.def != it.def || s.enchant != it.enchant
	})
	changed := len(shop.items) != n
	for i := range shop.items {
		if have := p.pl.inv.bag[shop.items[i].slot].count; have < shop.items[i].count {
			shop.items[i].count = have
			changed = true
		}
	}
	switch {
	case len(shop.items) == 0:
		z.pshopClose(p, "Nada mais à venda: a loja fechou.")
	case changed:
		z.pshopRefresh(p)
	}
}

// checkPrivateShops: 1×/s, fecha a loja de quem andou, morreu ou começou a viajar.
func (z *Zone) checkPrivateShops() {
	for _, id := range slices.Sorted(maps.Keys(z.players)) {
		p := z.players[id]
		shop := p.pl.pshop
		if shop == nil {
			continue
		}
		switch {
		case p.dead:
			z.pshopClose(p, "Você morreu: a loja fechou.")
		case p.pl.channel != nil:
			z.pshopClose(p, "Você começou a viajar: a loja fechou.")
		case dist(p.pos, shop.pos) > pshopMoveTolerance:
			z.pshopClose(p, "Você saiu do lugar: a loja fechou.")
		}
	}
}

// pshopTitleOf: a placa da loja aberta ("" = nenhuma).
func pshopTitleOf(p *entity) string {
	if p.pl == nil || p.pl.pshop == nil {
		return ""
	}
	return p.pl.pshop.title
}
