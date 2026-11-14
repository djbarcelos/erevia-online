package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// As ações do caos. O tipo sai só do sorteio r[0] (pickAction); cada ação lê o estado da
// sessão com o mutex, monta as mensagens e só então envia (send fora do mutex). r[1] decide
// em geral se a variante é inválida de propósito; r[2..5] são os parâmetros.

type chaosAction struct {
	name   string
	weight int
	fn     func(b *chaosBot, s *chaosSession, r [6]uint64)
}

var chaosActions = []chaosAction{
	{"mover", 12, actMove},
	{"atacar", 5, actAttack},
	{"skill", 4, actSkill},
	{"renascer", 2, actRespawn},
	{"pegar_do_chao", 2, actPickup},
	{"mover_item", 4, actMoveItem},
	{"usar_item", 5, actUseItem},
	{"destruir", 2, actDestroy},
	{"comprar", 6, actBuy},
	{"vender", 5, actSell},
	{"encantar", 5, actEnchant},
	{"armazem_item", 5, actStorageMove},
	{"armazem_moedas", 3, actStorageGold},
	{"troca", 16, actTrade},
	{"grupo", 4, actParty},
	{"cla", 4, actClan},
	{"convite", 7, actInviteReply},
	{"teleporte", 2, actTeleport},
	{"cancelar_canal", 1, actCancelChannel},
	{"criar_item", 3, actCraft},
	{"barbeiro", 1, actChangeLook},
	{"atributos", 2, actAttrs},
	{"skills", 2, actLearn},
	{"atalhos", 1, actHotbar},
	{"especializacao", 1, actSpec},
	{"maestria", 1, actMastery},
	{"bestiario", 1, actBestiary},
	{"amigos", 1, actFriend},
	{"correio", 2, actMail},
	{"leilao", 8, actAuction},
	{"ranking", 1, actRanking},
	{"titulo", 1, actTitle},
	{"chat", 2, actChat},
	{"fps", 1, actStats},
	{"combo", 7, actCombo},
	{"cair", 1, actDisconnect},
	{"lixo", 1, actGarbage},
}

var chaosTotalWeight = func() int {
	n := 0
	for _, a := range chaosActions {
		n += a.weight
	}
	return n
}()

func pickAction(r uint64) chaosAction {
	n := int(r % uint64(chaosTotalWeight))
	for _, a := range chaosActions {
		if n < a.weight {
			return a
		}
		n -= a.weight
	}
	return chaosActions[0]
}

// emit envia as mensagens (fora do mutex da sessão) e conta o rótulo.
func (b *chaosBot) emit(s *chaosSession, label string, msgs ...*pb.ClientMessage) {
	b.sent(label)
	for _, m := range msgs {
		s.send(m)
	}
}

func bad(r uint64, p int) bool { return pct(r) < p }

// ---------- Movimento e combate ----------

func actMove(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	var x, z float64
	label := "mover"
	stop := false
	// Trocando (ou com troca a caminho): fica perto do outro, a troca exige 8 m.
	var partner seen
	trading := false
	switch {
	case s.tradeOpen:
		partner, trading = s.ents[s.partner]
	case len(s.invites) > 0 && s.invites[len(s.invites)-1].Kind == pb.InviteKind_INVITE_KIND_TRADE:
		from := s.invites[len(s.invites)-1].FromName
		for _, e := range s.ents {
			if e.name == from {
				partner, trading = e, true
			}
		}
	case s.asked != 0 && time.Since(s.askedAt) < 30*time.Second:
		partner, trading = s.ents[s.asked]
	}
	switch p := pct(r[1]); {
	case trading && p < 80:
		x, z = partner.pos.x+float64(pickN(r[3], 20))/10-1, partner.pos.z+float64(pickN(r[4], 20))/10-1
		label = "mover:parceiro_da_troca"
	case p < 45: // perto de um NPC (é onde as compras, o armazém e as trocas acontecem)
		_, n := s.npcBy(r[2], func(n *npcSeen) bool { return math.Hypot(n.pos.x, n.pos.z) < 30 })
		if n != nil {
			x, z = n.pos.x+float64(pickN(r[3], 50))/10-2.5, n.pos.z+float64(pickN(r[4], 50))/10-2.5
		}
		label = "mover:npc"
	case p < 68:
		a, d := float64(pickN(r[2], 360))*math.Pi/180, float64(pickN(r[3], 140))/10
		x, z = d*math.Cos(a), d*math.Sin(a)
		label = "mover:cidade"
	case p < 84: // caçar
		a, d := float64(pickN(r[2], 360))*math.Pi/180, 45+float64(pickN(r[3], 30))
		x, z = d*math.Cos(a), d*math.Sin(a)
		label = "mover:caca"
	case p < 89:
		stop, label = true, "mover:parar"
	case p < 94:
		x, z = float64(pickN(r[2], 700))-350, float64(pickN(r[3], 700))-350
		label = "mover:longe"
	default:
		vals := []float64{math.NaN(), math.Inf(1), math.Inf(-1), 1e30, -1e30, math.MaxFloat32, -0.0}
		x, z = vals[pickN(r[2], len(vals))], vals[pickN(r[3], len(vals))]
		label = "mover!nan_inf"
	}
	s.mu.Unlock()
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_MoveTo{MoveTo: &pb.MoveTo{X: float32(x), Z: float32(z), Stop: stop}}})
}

func actAttack(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	var target uint32
	force := false
	label := "atacar:monstro"
	switch p := pct(r[1]); {
	case p < 60:
		if l := s.nearby(pb.EntityKind_ENTITY_KIND_MONSTER, 40); len(l) > 0 {
			target = l[pickN(r[2], min(3, len(l)))]
		}
	case p < 78:
		if l := s.nearby(pb.EntityKind_ENTITY_KIND_PLAYER, 30); len(l) > 0 {
			target = l[pickN(r[2], len(l))]
		}
		force, label = pct(r[3]) < 50, "atacar:jogador"
	case p < 86:
		target, _ = s.npcBy(r[2], func(*npcSeen) bool { return true })
		label = "atacar!npc"
	default:
		target, label = badU32(badIDs, r[2]), "atacar!id"
		if pct(r[3]) < 30 {
			target = s.id // a si mesmo
		}
	}
	s.mu.Unlock()
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_Attack{Attack: &pb.Attack{TargetId: target, Force: force}}})
}

func actSkill(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	var skill, target uint32
	label := "skill"
	if len(s.skills) > 0 {
		skill = s.skills[pickN(r[2], len(s.skills))]
	}
	if l := s.nearby(pb.EntityKind_ENTITY_KIND_MONSTER, 30); len(l) > 0 {
		target = l[0]
	}
	if bad(r[1], 25) {
		skill, label = badU32(badIDs, r[3]), "skill!id"
		if pct(r[4]) < 50 {
			target = badU32(badIDs, r[5])
		}
	}
	s.mu.Unlock()
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_UseSkill{UseSkill: &pb.UseSkill{SkillId: skill, TargetId: target, Force: pct(r[4]) < 20}}})
}

func actRespawn(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	label := "renascer"
	if !s.dead {
		label = "renascer!vivo"
	}
	s.mu.Unlock()
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_Respawn{Respawn: &pb.Respawn{}}})
}

func actPickup(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	var id uint32
	label := "pegar"
	best := math.MaxFloat64
	for gid, g := range s.ground {
		if d := math.Hypot(g.x-s.pos.x, g.z-s.pos.z); d < best || (d == best && gid < id) {
			best, id = d, gid
		}
	}
	if id == 0 || bad(r[1], 20) {
		id, label = badU32(badIDs, r[2]), "pegar!id"
	}
	s.mu.Unlock()
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_PickUp{PickUp: &pb.PickUp{Id: id}}})
}

// ---------- Itens ----------

func actMoveItem(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	m := &pb.MoveItem{}
	label := ""
	switch p := pct(r[1]); {
	case p < 30:
		m.FromPlace, m.FromSlot, m.ToPlace, m.ToSlot = 0, s.someBag(r[2], nil), 0, uint32(pickN(r[3], game.BagSize))
		label = "mover_item:bolsa"
	case p < 55: // equipar num encaixe (às vezes no errado)
		from := s.someBag(r[2], isGear)
		to := uint32(pickN(r[3], game.EquipSlots))
		if d := game.Items[s.bag[from%game.BagSize].id]; d != nil && len(d.Slots()) > 0 && pct(r[4]) < 70 {
			to = uint32(d.Slots()[pickN(r[5], len(d.Slots()))])
		}
		m.FromPlace, m.FromSlot, m.ToPlace, m.ToSlot = 0, from, 1, to
		label = "mover_item:equipar"
	case p < 72:
		if l := s.equipSlots(); len(l) > 0 {
			m.FromSlot = l[pickN(r[2], len(l))]
		}
		m.FromPlace, m.ToPlace, m.ToSlot = 1, 0, uint32(pickN(r[3], game.BagSize))
		label = "mover_item:desequipar"
	case p < 80:
		if l := s.equipSlots(); len(l) > 0 {
			m.FromSlot = l[pickN(r[2], len(l))]
		}
		m.FromPlace, m.ToPlace, m.ToSlot = 1, 1, uint32(pickN(r[3], game.EquipSlots))
		label = "mover_item:encaixes"
	default:
		m.FromPlace, m.ToPlace = uint32(pickN(r[2], 4)), uint32(pickN(r[3], 4))
		m.FromSlot, m.ToSlot = badU32(badSlots, r[4]), s.someBag(r[5], nil)
		if pct(r[5]) < 50 {
			m.FromSlot, m.ToSlot = s.someBag(r[4], nil), badU32(badSlots, r[5])
		}
		if pct(r[4]) < 25 {
			m.FromPlace = 7
		}
		label = "mover_item!fora"
	}
	s.mu.Unlock()
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_MoveItem{MoveItem: m}})
}

func actUseItem(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	u := &pb.UseItem{}
	label := "usar:bolsa"
	switch p := pct(r[1]); {
	case p < 70:
		u.Slot = s.someBag(r[2], nil)
	case p < 85:
		u.Place, label = 1, "usar:equipado"
		if l := s.equipSlots(); len(l) > 0 {
			u.Slot = l[pickN(r[2], len(l))]
		}
	default:
		u.Place, u.Slot, label = uint32(pickN(r[2], 5)), badU32(badSlots, r[3]), "usar!fora"
	}
	s.mu.Unlock()
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_UseItem{UseItem: u}})
}

func actDestroy(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	d := &pb.DestroyItem{Slot: s.someBag(r[2], nil), Count: 1}
	label := "destruir"
	switch p := pct(r[1]); {
	case p < 50:
		d.Count = uint32(1 + pickN(r[3], 3))
	case p < 70:
		d.Count, label = badU32(badCounts, r[3]), "destruir!quantidade"
	case p < 85:
		d.Slot, label = badU32(badSlots, r[3]), "destruir!fora"
	default:
		d.Count = 0
		label = "destruir:tudo"
	}
	s.mu.Unlock()
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_DestroyItem{DestroyItem: d}})
}

func (s *chaosSession) buyMsg(r [6]uint64) (*pb.ClientMessage, string) {
	npc, n := s.npcBy(r[2], shopNPC)
	m := &pb.Buy{NpcId: npc, Count: []uint32{1, 1, 1, 2, 3, 5, 20}[pickN(r[4], 7)]}
	label := "comprar"
	if n != nil {
		m.ItemId = n.stock[pickN(r[3], len(n.stock))]
	}
	switch p := pct(r[1]); {
	case p < 75:
	case p < 85:
		m.Count, label = badU32(badCounts, r[5]), "comprar!quantidade"
	case p < 92:
		m.ItemId, label = []string{"", "gold", "nao_existe", "potion_hp_s ", "bone_old", "ring_legend"}[pickN(r[5], 6)], "comprar!item"
	default:
		m.NpcId, label = badU32(badIDs, r[5]), "comprar!npc"
		if pct(r[5]) < 50 {
			m.NpcId = s.nearestNPC(storageNPC)
		}
	}
	return &pb.ClientMessage{Msg: &pb.ClientMessage_Buy{Buy: m}}, label
}

func actBuy(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	m, label := s.buyMsg(r)
	s.mu.Unlock()
	b.emit(s, label, m)
}

func (s *chaosSession) sellMsg(r [6]uint64, slot uint32) (*pb.ClientMessage, string) {
	m := &pb.Sell{NpcId: s.nearestNPC(shopNPC), Slot: slot, Count: []uint32{1, 1, 2, 5, 0}[pickN(r[3], 5)]}
	label := "vender"
	switch p := pct(r[1]); {
	case p < 80:
	case p < 90:
		m.Count, label = badU32(badCounts, r[4]), "vender!quantidade"
	default:
		m.Slot, label = badU32(badSlots, r[4]), "vender!fora"
	}
	return &pb.ClientMessage{Msg: &pb.ClientMessage_Sell{Sell: m}}, label
}

func actSell(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	m, label := s.sellMsg(r, s.someBag(r[2], nil))
	s.mu.Unlock()
	b.emit(s, label, m)
}

// enchantMsg: pergaminho da bolsa num item da bolsa ou equipado (às vezes no errado).
func (s *chaosSession) enchantMsg(r [6]uint64, target int) (*pb.ClientMessage, string) {
	e := &pb.Enchant{ScrollSlot: s.someBag(r[2], isScroll)}
	label := "encantar:equipado"
	if l := s.equipSlots(); len(l) > 0 && pct(r[3]) < 50 {
		e.Place, e.Slot = 1, l[pickN(r[4], len(l))]
	} else {
		e.Place, e.Slot, label = 0, s.someBag(r[4], isGear), "encantar:bolsa"
	}
	if target >= 0 {
		e.Place, e.Slot, label = 0, uint32(target), "encantar:combo"
	}
	switch p := pct(r[1]); {
	case p < 80:
	case p < 87:
		e.Slot, label = e.ScrollSlot, "encantar!o_proprio_pergaminho"
		e.Place = 0
	case p < 94:
		e.Place, e.Slot, label = uint32(pickN(r[5], 4)), badU32(badSlots, r[5]), "encantar!fora"
	default:
		e.ScrollSlot, label = badU32(badSlots, r[5]), "encantar!pergaminho_fora"
	}
	return &pb.ClientMessage{Msg: &pb.ClientMessage_Enchant{Enchant: e}}, label
}

func actEnchant(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	m, label := s.enchantMsg(r, -1)
	s.mu.Unlock()
	b.emit(s, label, m)
}

// ---------- Armazém ----------

func (s *chaosSession) storageMoveMsg(r [6]uint64, bagSlot int) (*pb.ClientMessage, string) {
	m := &pb.StorageMove{NpcId: s.nearestNPC(storageNPC)}
	label := ""
	switch p := pct(r[1]); {
	case bagSlot >= 0 || p < 40:
		from := s.someBag(r[2], nil)
		if bagSlot >= 0 {
			from = uint32(bagSlot)
		}
		m.FromPlace, m.FromSlot, m.ToPlace, m.ToSlot = 0, from, 2, uint32(pickN(r[3], 60))
		label = "armazem:guardar"
	case p < 75:
		if l := s.storageSlots(); len(l) > 0 {
			m.FromSlot = l[pickN(r[2], len(l))]
		}
		m.FromPlace, m.ToPlace, m.ToSlot = 2, 0, uint32(pickN(r[3], game.BagSize))
		label = "armazem:retirar"
	case p < 85:
		if l := s.storageSlots(); len(l) > 0 {
			m.FromSlot = l[pickN(r[2], len(l))]
		}
		m.FromPlace, m.ToPlace, m.ToSlot = 2, 2, uint32(pickN(r[3], 60))
		label = "armazem:arrumar"
	default:
		m.FromPlace, m.ToPlace = uint32(pickN(r[2], 4)), uint32(pickN(r[3], 4))
		m.FromSlot, m.ToSlot = badU32(badSlots, r[4]), badU32(badSlots, r[5])
		if pct(r[5]) < 40 {
			m.NpcId = s.nearestNPC(shopNPC)
		}
		label = "armazem!fora"
	}
	m.Count = []uint32{0, 0, 1, 2, 5}[pickN(r[4], 5)]
	if pct(r[5]) < 8 {
		m.Count, label = badU32(badCounts, r[4]), label+"!quantidade"
	}
	return &pb.ClientMessage{Msg: &pb.ClientMessage_StorageMove{StorageMove: m}}, label
}

func actStorageMove(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	m, label := s.storageMoveMsg(r, -1)
	s.mu.Unlock()
	b.emit(s, label, m)
}

func (s *chaosSession) storageGoldMsg(r [6]uint64) (*pb.ClientMessage, string) {
	m := &pb.StorageGold{NpcId: s.nearestNPC(storageNPC)}
	label := ""
	switch p := pct(r[1]); {
	case p < 35:
		m.Amount, label = int64(s.gold)*int64(1+pickN(r[2], 4))/4, "armazem_moedas:guardar"
	case p < 70:
		m.Amount, label = -int64(s.stGold)*int64(1+pickN(r[2], 4))/4, "armazem_moedas:retirar"
	case p < 80:
		m.Amount, label = 0, "armazem_moedas:abrir"
	default:
		m.Amount = []int64{math.MaxInt64, math.MinInt64, math.MinInt64 + 1, 1 << 62, -(1 << 62), int64(s.gold) + 1}[pickN(r[2], 6)]
		label = "armazem_moedas!enorme"
	}
	return &pb.ClientMessage{Msg: &pb.ClientMessage_StorageGold{StorageGold: m}}, label
}

func actStorageGold(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	m, label := s.storageGoldMsg(r)
	s.mu.Unlock()
	b.emit(s, label, m)
}

// ---------- Troca ----------

func tradeMsg(kind pb.TradeCmdKind, target, slot, count uint32, gold int64) *pb.ClientMessage {
	return &pb.ClientMessage{Msg: &pb.ClientMessage_TradeCmd{TradeCmd: &pb.TradeCmd{Kind: kind, TargetId: target, Slot: slot, Count: count, Gold: gold}}}
}

// tradeStep: a próxima jogada na troca, conforme o estado (quem não está trocando convida).
func (s *chaosSession) tradeStep(r [6]uint64) (*pb.ClientMessage, string) {
	if !s.tradeOpen {
		var target uint32
		if l := s.nearby(pb.EntityKind_ENTITY_KIND_PLAYER, 7.5); len(l) > 0 {
			target = l[pickN(r[2], min(3, len(l)))]
		} else if l := s.nearby(pb.EntityKind_ENTITY_KIND_PLAYER, 60); len(l) > 0 {
			target = l[0]
		}
		label := "troca:convidar"
		switch p := pct(r[1]); {
		case p < 80:
		case p < 88:
			target, label = s.id, "troca!consigo"
		case p < 94:
			target, label = badU32(badIDs, r[3]), "troca!id"
		default: // comando de troca sem troca aberta
			kinds := []pb.TradeCmdKind{pb.TradeCmdKind_TRADE_CMD_ADD_ITEM, pb.TradeCmdKind_TRADE_CMD_LOCK, pb.TradeCmdKind_TRADE_CMD_CONFIRM, pb.TradeCmdKind_TRADE_CMD_SET_GOLD, pb.TradeCmdKind_TRADE_CMD_CANCEL, 99}
			return tradeMsg(kinds[pickN(r[3], len(kinds))], 0, s.someBag(r[4], nil), 0, 50), "troca!sem_troca"
		}
		s.asked, s.askedAt = target, time.Now()
		return tradeMsg(pb.TradeCmdKind_TRADE_CMD_REQUEST, target, 0, 0, 0), label
	}
	mineLocked := s.mine != nil && s.mine.Locked
	bothLocked := mineLocked && s.them != nil && s.them.Locked
	p := pct(r[1])
	switch {
	case bothLocked && p < 75:
		return tradeMsg(pb.TradeCmdKind_TRADE_CMD_CONFIRM, 0, 0, 0, 0), "troca:confirmar"
	case mineLocked && !bothLocked && p < 85: // esperando o outro travar (sem destravar tudo)
		return tradeMsg(pb.TradeCmdKind_TRADE_CMD_CONFIRM, 0, 0, 0, 0), "troca:confirmar_cedo"
	case !mineLocked && p < 40, bothLocked && p < 82:
		slot := s.someBag(r[2], nil)
		count := []uint32{0, 0, 1, 2}[pickN(r[3], 4)]
		label := "troca:oferecer_item"
		if pct(r[4]) < 10 {
			slot, label = badU32(badSlots, r[5]), "troca!item_fora"
		} else if pct(r[4]) < 18 {
			count, label = badU32(badCounts, r[5]), "troca!quantidade"
		}
		return tradeMsg(pb.TradeCmdKind_TRADE_CMD_ADD_ITEM, 0, slot, count, 0), label
	case !mineLocked && p < 60:
		g := int64(s.gold) * int64(pickN(r[2], 5)) / 4
		label := "troca:moedas"
		if pct(r[3]) < 20 {
			g, label = []int64{-1, math.MinInt64, math.MaxInt64, int64(s.gold) + 1000}[pickN(r[4], 4)], "troca!moedas"
		}
		return tradeMsg(pb.TradeCmdKind_TRADE_CMD_SET_GOLD, 0, 0, 0, g), label
	case !mineLocked && p < 88:
		return tradeMsg(pb.TradeCmdKind_TRADE_CMD_LOCK, 0, 0, 0, 0), "troca:travar"
	case p < 94:
		var slot uint32
		if s.mine != nil && len(s.mine.Items) > 0 {
			slot = s.mine.Items[pickN(r[2], len(s.mine.Items))].Slot
		}
		return tradeMsg(pb.TradeCmdKind_TRADE_CMD_REMOVE_ITEM, 0, slot, 0, 0), "troca:tirar_item"
	case p < 97:
		return tradeMsg(pb.TradeCmdKind_TRADE_CMD_CONFIRM, 0, 0, 0, 0), "troca:confirmar_cedo"
	default:
		return tradeMsg(pb.TradeCmdKind_TRADE_CMD_CANCEL, 0, 0, 0, 0), "troca:cancelar"
	}
}

func actTrade(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	m, label := s.tradeStep(r)
	s.mu.Unlock()
	b.emit(s, label, m)
}

// ---------- Grupo, clã e convites ----------

func actParty(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	m := &pb.PartyCmd{}
	label := ""
	var members []uint32
	if s.party != nil {
		for _, pm := range s.party.Members {
			if pm.Id != s.id {
				members = append(members, pm.Id)
			}
		}
	}
	switch p := pct(r[1]); {
	case p < 45:
		m.Kind, label = pb.PartyCmdKind_PARTY_CMD_INVITE, "grupo:convidar"
		if l := s.nearby(pb.EntityKind_ENTITY_KIND_PLAYER, 40); len(l) > 0 && pct(r[2]) < 50 {
			m.TargetId = l[pickN(r[3], len(l))]
		} else {
			m.Name = b.otherBot(r[3])
		}
	case p < 62:
		m.Kind, label = pb.PartyCmdKind_PARTY_CMD_LEAVE, "grupo:sair"
	case p < 77:
		m.Kind, label = pb.PartyCmdKind_PARTY_CMD_KICK, "grupo:expulsar"
		if len(members) > 0 {
			m.TargetId = members[pickN(r[3], len(members))]
		}
	case p < 90:
		m.Kind, label = pb.PartyCmdKind_PARTY_CMD_LEADER, "grupo:lider"
		if len(members) > 0 {
			m.TargetId = members[pickN(r[3], len(members))]
		}
	default:
		m.Kind, m.TargetId, label = pb.PartyCmdKind(pickN(r[2], 7)), badU32(badIDs, r[3]), "grupo!id"
		if pct(r[4]) < 40 {
			m.TargetId, m.Name = 0, []string{"", "   ", strings.Repeat("x", 300), b.name}[pickN(r[5], 4)]
		}
	}
	s.mu.Unlock()
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_PartyCmd{PartyCmd: m}})
}

func (b *chaosBot) clanName(r uint64) string {
	return fmt.Sprintf("K%s%02d", b.run.tag, pickN(r, 12)) // poucos nomes: colisões de propósito
}

func (s *chaosSession) clanMembers() []uint32 {
	var out []uint32
	if s.clan != nil {
		for _, m := range s.clan.Members {
			if uint32(m.CharacterId) != s.id {
				out = append(out, m.CharacterId)
			}
		}
	}
	return out
}

func (s *chaosSession) clanMsg(b *chaosBot, r [6]uint64) (*pb.ClientMessage, string) {
	m := &pb.ClanCmd{}
	label := ""
	members := s.clanMembers()
	pickMember := func() uint32 {
		if len(members) == 0 {
			return badU32(badIDs, r[4])
		}
		return members[pickN(r[3], len(members))]
	}
	inClan := s.clan != nil && s.clan.Id != 0
	switch p := pct(r[1]); {
	case !inClan && p < 50:
		m.Kind, m.Name, label = pb.ClanCmdKind_CLAN_CMD_CREATE, b.clanName(r[2]), "cla:criar"
	case p < 35:
		m.Kind, label = pb.ClanCmdKind_CLAN_CMD_INVITE, "cla:convidar"
		if l := s.nearby(pb.EntityKind_ENTITY_KIND_PLAYER, 40); len(l) > 0 && pct(r[2]) < 50 {
			m.TargetId = l[pickN(r[3], len(l))]
		} else {
			m.Name = b.otherBot(r[3])
		}
	case p < 45:
		m.Kind, label = pb.ClanCmdKind_CLAN_CMD_LEAVE, "cla:sair"
	case p < 55:
		m.Kind, m.TargetId, label = pb.ClanCmdKind_CLAN_CMD_KICK, pickMember(), "cla:expulsar"
	case p < 65:
		m.Kind, m.TargetId, m.Rank, label = pb.ClanCmdKind_CLAN_CMD_RANK, pickMember(), pb.ClanRank(pickN(r[5], 3)), "cla:posto"
		if pct(r[5]) < 15 {
			m.Rank, label = pb.ClanRank(7+pickN(r[2], 100)), "cla!posto"
		}
	case p < 73:
		m.Kind, m.TargetId, label = pb.ClanCmdKind_CLAN_CMD_LEADER, pickMember(), "cla:lider"
	case p < 83:
		m.Kind, label = pb.ClanCmdKind_CLAN_CMD_DISBAND, "cla:desfazer"
	case p < 92:
		m.Kind, label = pb.ClanCmdKind_CLAN_CMD_NOTICE, "cla:aviso"
		m.Text = []string{"Reunião na praça", "", strings.Repeat("ã", 400), "linha\x00nul‮", "<b>oi</b>"}[pickN(r[2], 5)]
	default:
		m.Kind, label = pb.ClanCmdKind(pickN(r[2], 12)), "cla!lixo"
		m.Name = []string{"", "a", "nome com espaço", strings.Repeat("Z", 40), b.clanName(r[3]), "ÇÃO"}[pickN(r[3], 6)]
		m.TargetId = badU32(badIDs, r[4])
	}
	return &pb.ClientMessage{Msg: &pb.ClientMessage_ClanCmd{ClanCmd: m}}, label
}

func actClan(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	m, label := s.clanMsg(b, r)
	s.mu.Unlock()
	b.emit(s, label, m)
}

func actInviteReply(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	m := &pb.InviteReply{Accept: pct(r[2]) < 85}
	label := "convite:sem_convite"
	if n := len(s.invites); n > 0 {
		inv := s.invites[n-1]
		if pct(r[3]) < 20 && n > 1 { // um convite antigo (talvez vencido)
			inv = s.invites[pickN(r[4], n)]
		}
		m.InviteId = inv.Id
		label = "convite:" + strings.ToLower(strings.TrimPrefix(inv.Kind.String(), "INVITE_KIND_"))
		if !m.Accept {
			label += ":recusar"
		}
		s.invites = s.invites[:n-1]
	}
	if bad(r[1], 10) {
		m.InviteId, label = badU32(badIDs, r[5]), "convite!id"
	}
	s.mu.Unlock()
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_InviteReply{InviteReply: m}})
}

// ---------- Teleporte, criação, visual e progressão ----------

func (s *chaosSession) teleportMsg(r [6]uint64) (*pb.ClientMessage, string) {
	npc := s.nearestNPC(func(n *npcSeen) bool { return n.role == pb.NpcRole_NPC_ROLE_TELEPORT })
	m := &pb.Teleport{NpcId: npc}
	label := "teleporte"
	if n := s.npcs[npc]; n != nil && len(n.dests) > 0 {
		m.DestId = n.dests[pickN(r[2], len(n.dests))]
	}
	if bad(r[1], 25) {
		m.DestId, label = []uint32{0, 1, 6, 99, math.MaxUint32}[pickN(r[3], 5)], "teleporte!destino"
		if pct(r[4]) < 40 {
			m.NpcId, label = s.nearestNPC(shopNPC), "teleporte!npc"
		}
	}
	return &pb.ClientMessage{Msg: &pb.ClientMessage_Teleport{Teleport: m}}, label
}

func actTeleport(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	m, label := s.teleportMsg(r)
	s.mu.Unlock()
	b.emit(s, label, m)
}

func actCancelChannel(b *chaosBot, s *chaosSession, r [6]uint64) {
	b.emit(s, "cancelar_canal", &pb.ClientMessage{Msg: &pb.ClientMessage_CancelChannel{CancelChannel: &pb.CancelChannel{}}})
}

func (s *chaosSession) craftMsg(r [6]uint64) (*pb.ClientMessage, string) {
	npc, n := s.npcBy(r[2], smithNPC)
	m := &pb.Craft{NpcId: npc}
	label := "criar"
	if n != nil {
		m.RecipeId = n.recipes[pickN(r[3], len(n.recipes))]
	}
	if bad(r[1], 20) {
		m.RecipeId, label = []string{"", "nao_existe", "craft_ench_armor ", "CRAFT_ENCH_ARMOR"}[pickN(r[4], 4)], "criar!receita"
		if pct(r[5]) < 40 {
			m.NpcId, label = s.nearestNPC(shopNPC), "criar!npc"
		}
	}
	return &pb.ClientMessage{Msg: &pb.ClientMessage_Craft{Craft: m}}, label
}

func actCraft(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	m, label := s.craftMsg(r)
	s.mu.Unlock()
	b.emit(s, label, m)
}

func actChangeLook(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	npc := s.nearestNPC(func(n *npcSeen) bool { return n.role == pb.NpcRole_NPC_ROLE_BARBER })
	look := &pb.BodyLook{Skin: uint32(pickN(r[2], 6)), Hair: uint32(pickN(r[3], 6)), Outfit: uint32(pickN(r[4], 6)),
		Head: []string{"", "knight", "mage", "rogue", "barbarian", "ranger"}[pickN(r[5], 6)], HideHat: pct(r[5]) < 30}
	label := "barbeiro"
	if bad(r[1], 25) {
		look = &pb.BodyLook{Skin: math.MaxUint32, Hair: 1000, Outfit: 1 << 31, Head: "dragao"}
		label = "barbeiro!visual"
		if pct(r[2]) < 30 {
			look = nil
		}
	}
	s.mu.Unlock()
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_ChangeLook{ChangeLook: &pb.ChangeLook{NpcId: npc, Look: look}}})
}

func actAttrs(b *chaosBot, s *chaosSession, r [6]uint64) {
	a := &pb.Attrs{}
	label := "atributos"
	switch pickN(r[2], 6) {
	case 0:
		a.Str = 1
	case 1:
		a.Dex = 1
	case 2:
		a.Con = 1
	case 3:
		a.Int = 1
	case 4:
		a.Wis = 1
	default:
		a.Spi = 1
	}
	if bad(r[1], 30) {
		a = &pb.Attrs{Str: math.MaxUint32, Dex: math.MaxUint32, Con: math.MaxUint32, Int: math.MaxUint32, Wis: math.MaxUint32, Spi: math.MaxUint32}
		label = "atributos!enorme"
		if pct(r[3]) < 40 {
			a, label = nil, "atributos!vazio"
		}
	}
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_AllocateAttrs{AllocateAttrs: &pb.AllocateAttrs{Add: a}}})
}

func actLearn(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	var ranks []*pb.SkillRank
	label := "skills"
	if len(s.skills) > 0 {
		id := s.skills[pickN(r[2], len(s.skills))]
		ranks = append(ranks, &pb.SkillRank{SkillId: id, Rank: s.ranks[id] + 1})
	}
	if bad(r[1], 30) {
		label = "skills!lixo"
		ranks = []*pb.SkillRank{{SkillId: badU32(badIDs, r[3]), Rank: 1}, {SkillId: 0, Rank: math.MaxUint32}}
		if len(s.skills) > 0 {
			ranks = append(ranks, &pb.SkillRank{SkillId: s.skills[0], Rank: math.MaxUint32}, &pb.SkillRank{SkillId: s.skills[0], Rank: 0})
		}
	}
	s.mu.Unlock()
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_LearnSkills{LearnSkills: &pb.LearnSkills{Ranks: ranks}}})
}

func actHotbar(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	slots := make([]string, 10)
	for i := range slots {
		switch pickN(r[2]>>uint(i*3), 3) {
		case 0:
			if len(s.skills) > 0 {
				slots[i] = fmt.Sprintf("skill:%d", s.skills[pickN(r[3]>>uint(i*2), len(s.skills))])
			}
		case 1:
			if it := s.bag[pickN(r[4]>>uint(i*2), game.BagSize)]; it.id != "" {
				slots[i] = "item:" + it.id
			}
		}
	}
	label := "atalhos"
	if bad(r[1], 30) {
		label = "atalhos!lixo"
		slots = []string{"skill:999999", "item:nao_existe", "skill:-1", "lixo", strings.Repeat("a", 90)}
		for range 30 {
			slots = append(slots, "")
		}
	}
	s.mu.Unlock()
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_SetHotbar{SetHotbar: &pb.SetHotbar{Slots: slots}}})
}

func actSpec(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	npc := s.nearestNPC(func(n *npcSeen) bool { return n.role == pb.NpcRole_NPC_ROLE_MASTER })
	m := &pb.SpecCmd{Kind: pb.SpecCmdKind_SPEC_CMD_TALK, NpcId: npc}
	label := "especializacao:falar"
	if pct(r[1]) < 50 && len(s.specs) > 0 {
		m.Kind, m.Spec, label = pb.SpecCmdKind_SPEC_CMD_CHOOSE, s.specs[pickN(r[2], len(s.specs))], "especializacao:escolher"
	}
	if bad(r[3], 15) {
		m.Kind, m.Spec, m.NpcId, label = pb.SpecCmdKind(pickN(r[4], 5)), "nao_existe", badU32(badIDs, r[5]), "especializacao!lixo"
	}
	s.mu.Unlock()
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_SpecCmd{SpecCmd: m}})
}

// actMastery: Maestria do nível 30 (pedir a árvore, gastar ponto num nó, devolver os pontos),
// às vezes com tipo ou nó que não existem.
func actMastery(b *chaosBot, s *chaosSession, r [6]uint64) {
	m := &pb.MasteryCmd{Kind: pb.MasteryCmdKind(1 + pickN(r[1], 3))}
	if m.Kind == pb.MasteryCmdKind_MASTERY_CMD_SPEND {
		m.Node = game.MasteryNodes[pickN(r[2], len(game.MasteryNodes))].ID
	}
	label := "maestria:" + strings.ToLower(strings.TrimPrefix(m.Kind.String(), "MASTERY_CMD_"))
	if bad(r[3], 15) {
		m.Kind, m.Node, label = pb.MasteryCmdKind(pickN(r[4], 6)), []string{"", "nao_existe", strings.Repeat("n", 200)}[pickN(r[5], 3)], "maestria!lixo"
	}
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_MasteryCmd{MasteryCmd: m}})
}

func actFriend(b *chaosBot, s *chaosSession, r [6]uint64) {
	m := &pb.FriendCmd{Kind: pb.FriendCmdKind(1 + pickN(r[2], 3)), Name: b.otherBot(r[3])}
	label := "amigos:" + strings.ToLower(strings.TrimPrefix(m.Kind.String(), "FRIEND_CMD_"))
	if bad(r[1], 20) {
		m.Name, m.TargetId, label = []string{"", "ninguem", b.name, strings.Repeat("q", 100)}[pickN(r[4], 4)], badU32(badIDs, r[5]), "amigos!lixo"
	}
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_FriendCmd{FriendCmd: m}})
}

func actMail(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	m := &pb.MailCmd{}
	label := ""
	switch p := pct(r[1]); {
	case p < 40:
		m.Kind, m.To, label = pb.MailCmdKind_MAIL_CMD_SEND, b.otherBot(r[2]), "correio:enviar"
		m.Subject = []string{"", "Oi", "Troca?", strings.Repeat("s", 200)}[pickN(r[3], 4)]
		m.Body = []string{"Vamos caçar?", "", strings.Repeat("corpo ", 150), "\x00\x01\x02"}[pickN(r[4], 4)]
	case p < 65:
		m.Kind, label = pb.MailCmdKind_MAIL_CMD_LIST, "correio:listar"
	case p < 85:
		m.Kind, label = pb.MailCmdKind_MAIL_CMD_READ, "correio:ler"
		if len(s.mails) > 0 {
			m.MailId = s.mails[pickN(r[2], len(s.mails))]
		}
	case p < 95:
		m.Kind, label = pb.MailCmdKind_MAIL_CMD_DELETE, "correio:apagar"
		if len(s.mails) > 0 {
			m.MailId = s.mails[pickN(r[2], len(s.mails))]
		}
	default:
		m.Kind, m.MailId, m.To, label = pb.MailCmdKind(pickN(r[2], 9)), math.MaxUint64, "", "correio!lixo"
	}
	s.mu.Unlock()
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_MailCmd{MailCmd: m}})
}

func actRanking(b *chaosBot, s *chaosSession, r [6]uint64) {
	b.emit(s, "ranking", &pb.ClientMessage{Msg: &pb.ClientMessage_RankingRequest{RankingRequest: &pb.RankingRequest{}}})
}

func actTitle(b *chaosBot, s *chaosSession, r [6]uint64) {
	s.mu.Lock()
	m := &pb.AchievementCmd{Kind: pb.AchievementCmdKind_ACHIEVEMENT_CMD_SET_TITLE}
	label := "titulo"
	if len(s.titles) > 0 {
		m.Id = s.titles[pickN(r[2], len(s.titles))]
	}
	switch p := pct(r[1]); {
	case p < 70:
	case p < 80:
		m.Id, label = "", "titulo:nenhum"
	default:
		m.Kind, m.Id, label = pb.AchievementCmdKind(pickN(r[3], 4)), []string{"nao_existe", strings.Repeat("t", 300)}[pickN(r[4], 2)], "titulo!lixo"
	}
	s.mu.Unlock()
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_AchievementCmd{AchievementCmd: m}})
}

var chatCommands = []string{"/bug caos: relato de teste", "/amigos", "/correio", "/sentar", "/dancar", "/acenar",
	"/gm", "/give %s gold 999999", "/tp 0 0", "/kick %s", "/evento xp2 10", "/raid", "/", "/ ", "/convidar %s"}

func actChat(b *chaosBot, s *chaosSession, r [6]uint64) {
	m := &pb.ChatSend{}
	label := ""
	switch p := pct(r[1]); {
	case p < 30:
		m.Text, label = "caos "+b.name, "chat:geral"
	case p < 45:
		m.Text, m.To, label = "psiu", b.otherBot(r[2]), "chat:sussurro"
	case p < 55:
		m.Text, m.Channel, label = "grupo!", pb.ChatChannel_CHAT_CHANNEL_PARTY, "chat:grupo"
	case p < 65:
		m.Text, m.Channel, label = "clã!", pb.ChatChannel_CHAT_CHANNEL_CLAN, "chat:cla"
	case p < 85:
		c := chatCommands[pickN(r[2], len(chatCommands))]
		if strings.Contains(c, "%s") {
			c = fmt.Sprintf(c, b.otherBot(r[3]))
		}
		m.Text, label = c, "chat:comando"
	default:
		m.Text = []string{strings.Repeat("ç", 500), "\x00\x00", "‮gnirts", "", "  ", "a\nb\rc\td"}[pickN(r[2], 6)]
		m.Channel = pb.ChatChannel(pickN(r[3], 8))
		label = "chat!lixo"
	}
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_ChatSend{ChatSend: m}})
}

func actStats(b *chaosBot, s *chaosSession, r [6]uint64) {
	m := &pb.ClientStats{FpsAvg: float32(20 + pickN(r[2], 100)), FpsLow: 15, Device: "desktop caos", Seconds: 60}
	label := "fps"
	if bad(r[1], 30) {
		vals := []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1)), -5, 1e9, 60}
		m.FpsAvg, m.FpsLow, m.Seconds = vals[pickN(r[3], len(vals))], vals[pickN(r[4], len(vals))], math.MaxUint32
		m.Device = []string{strings.Repeat("d", 200), "mobile\x00nul", strings.Repeat("é", 41)}[pickN(r[5], 3)]
		label = "fps!lixo"
	}
	b.emit(s, label, &pb.ClientMessage{Msg: &pb.ClientMessage_ClientStats{ClientStats: m}})
}

// ---------- Quedas e lixo ----------

func actDisconnect(b *chaosBot, s *chaosSession, r [6]uint64) {
	switch p := pct(r[1]); {
	case p < 35:
		b.sent("cair:fechar")
		s.close(true, false)
	case p < 70:
		b.sent("cair:abrupto")
		s.close(true, true)
	case p < 85: // relogar com a sessão antiga ainda aberta (o servidor derruba a velha)
		b.sent("cair:relogin_por_cima")
		b.relogOver(s)
		return
	default:
		b.sent("cair:volta_ja")
		s.close(true, true)
		return // volta no próximo passo
	}
	b.offline = 1 + pickN(r[2], 25)
}

// relogOver abre outra conexão do mesmo personagem antes de fechar a atual.
func (b *chaosBot) relogOver(old *chaosSession) {
	old.mu.Lock()
	old.byUs = true // o kick "entrou em outro lugar" é esperado
	old.mu.Unlock()
	s, err := b.dial(b.ctx)
	if err != nil {
		b.run.count(b.run.errs, "relogin: "+shortErr(err), 1)
		old.close(true, true)
		return
	}
	old.close(true, true)
	b.sess = s
}

func actGarbage(b *chaosBot, s *chaosSession, r [6]uint64) {
	switch pickN(r[1], 6) {
	case 0:
		b.sent("lixo:bytes")
		junk := make([]byte, 1+pickN(r[2], 200))
		for i := range junk {
			junk[i] = byte(r[3] >> uint(i%8*8))
		}
		s.sendRaw(websocket.MessageBinary, junk)
	case 1:
		b.sent("lixo:vazia")
		s.send(&pb.ClientMessage{})
	case 2:
		b.sent("lixo:texto")
		s.sendRaw(websocket.MessageText, []byte(`{"buy":{"npcId":1}}`))
	case 3:
		b.sent("lixo:join_de_novo")
		s.send(&pb.ClientMessage{Msg: &pb.ClientMessage_Join{Join: &pb.Join{CharacterId: badU32(badIDs, r[2])}}})
	case 4:
		b.sent("lixo:campo_desconhecido")
		raw, _ := proto.Marshal(&pb.ClientMessage{Msg: &pb.ClientMessage_Ping{Ping: &pb.Ping{ClientTime: 1}}})
		raw = append(raw, 0xF8, 0xFF, 0x03, 0x01) // campo 65535 varint
		s.sendRaw(websocket.MessageBinary, raw)
	default: // maior que o limite do servidor (1 KiB): ele fecha a conexão
		b.sent("lixo:grande_demais")
		s.mu.Lock()
		s.byUs = true
		s.mu.Unlock()
		s.send(&pb.ClientMessage{Msg: &pb.ClientMessage_ChatSend{ChatSend: &pb.ChatSend{Text: strings.Repeat("x", 3000)}}})
	}
}
