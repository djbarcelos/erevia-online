package zone

import (
	"fmt"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

const (
	npcIDBase = uint32(1) << 29 // NPCs: abaixo dos itens no chão e dos monstros
	shopRange = 6.0             // distância máxima para comprar e vender
)

type npcState struct {
	title string
	role  pb.NpcRole
	stock []*game.ItemDef
	stone int // pedra de teleporte: índice em teleportStones (role TELEPORT)
	crafts bool // Ferreiro: cria itens com as receitas
	look  *game.Appearance // NPC com modelo de herói: o visual dele
}

type npcDef struct {
	name, title, model string
	x, z               float64
	stock              []string
	role               pb.NpcRole
	look               *game.Appearance
	crafts             bool
}

// townNPCs: comerciantes da praça, ao lado das tendas e do suporte de armas.
var townNPCs = []npcDef{
	{name: "Lívia", title: "Mercadora", model: "npc_merchant", x: 4.6, z: -4.4, stock: game.MerchantStock},
	{name: "Bruno", title: "Ferreiro", model: "npc_smith", x: -7.4, z: -4.4, stock: game.SmithStock, crafts: true},
	// Barbeiro: ao norte do poço. Um herói com o próprio visual trocado, para mostrar o serviço.
	{name: "Teodoro", title: "Barbeiro", model: "rogue", x: 0, z: 9.5, role: pb.NpcRole_NPC_ROLE_BARBER,
		look: &game.Appearance{Skin: 4, Hair: 5, Outfit: 4, Head: "mage"}},
}

func (z *Zone) spawnNPCs() {
	for i, d := range townNPCs {
		pos := z.freeSpot(Vec2{X: d.x, Z: d.z})
		n := &entity{
			id: npcIDBase + uint32(i), kind: pb.EntityKind_ENTITY_KIND_NPC,
			name: d.name, model: d.model, level: 1, scale: 1,
			pos: pos, dest: pos, stats: game.Stats{MaxHP: 1}, hp: 1,
			npc: &npcState{title: d.title, role: d.role, look: d.look, crafts: d.crafts},
		}
		for _, id := range d.stock {
			if def := game.Items[id]; def != nil {
				n.npc.stock = append(n.npc.stock, def)
			}
		}
		z.npcs[n.id] = n
	}
	z.spawnStones(npcIDBase + uint32(len(townNPCs)))
	z.spawnKeeper(npcIDBase + uint32(len(townNPCs)+len(teleportStones)))
	z.spawnMaster(npcIDBase + uint32(len(townNPCs)+len(teleportStones)+1))
	z.spawnBoard(boardID) // o Quadro de Contratos (contracts.go)
	z.spawnAuctioneer(auctioneerID)
}

// storageKeeper: o Guarda do Armazém, na praça, do lado oeste do poço.
var storageKeeper = npcDef{name: "Otto", title: "Guarda do Armazém", model: "knight", x: -8, z: 6}

func (z *Zone) spawnKeeper(id uint32) {
	d := storageKeeper
	pos := z.freeSpot(Vec2{X: d.x, Z: d.z})
	z.npcs[id] = &entity{
		id: id, kind: pb.EntityKind_ENTITY_KIND_NPC,
		name: d.name, model: d.model, level: 1, scale: 1,
		pos: pos, dest: pos, stats: game.Stats{MaxHP: 1}, hp: 1,
		npc: &npcState{title: d.title, role: pb.NpcRole_NPC_ROLE_STORAGE},
	}
}

func (z *Zone) npcList() []*pb.NpcInfo {
	out := make([]*pb.NpcInfo, 0, len(z.npcs))
	for _, n := range z.npcs {
		info := &pb.NpcInfo{Id: n.id, Role: n.npc.role}
		for _, d := range n.npc.stock {
			info.Stock = append(info.Stock, d.ID)
		}
		if n.npc.role == pb.NpcRole_NPC_ROLE_TELEPORT {
			info.Dests = z.stoneDests(n.npc.stone)
		}
		if n.npc.crafts {
			info.Recipes = recipesPB()
		}
		out = append(out, info)
	}
	return out
}

// shopNPC devolve o NPC se o jogador estiver perto dele (senão avisa e devolve nil).
func (z *Zone) shopNPC(p *entity, id uint32) *entity {
	n := z.npcs[id]
	if n == nil || p.dead {
		return nil
	}
	if dist(p.pos, n.pos) > shopRange {
		z.system(p, fmt.Sprintf("Chegue mais perto de %s.", n.name))
		return nil
	}
	return n
}

func (z *Zone) buy(p *entity, npcID uint32, itemID string, count int) {
	n := z.shopNPC(p, npcID)
	if n == nil {
		return
	}
	var def *game.ItemDef
	for _, d := range n.npc.stock {
		if d.ID == itemID {
			def = d
		}
	}
	if def == nil {
		return
	}
	count = min(max(count, 1), game.MaxStack)
	cost := def.Price * int64(count)
	inv := &p.pl.inv
	if inv.gold < cost {
		z.system(p, "Moedas insuficientes.")
		return
	}
	if !inv.canAdd(def, count) {
		z.system(p, "Sua bolsa está cheia.")
		return
	}
	inv.gold -= cost
	inv.add(def, count)
	z.trackGold(p, "buy", -cost)
	z.trackItem(p, true, def.ID, count, "buy")
	z.system(p, fmt.Sprintf("Você comprou %s por %d moedas.", itemLabel(def, count), cost))
	z.inventoryChanged(p, false)
	z.questEvent(p, game.GoalBuy, def.ID)
}

func (z *Zone) sell(p *entity, npcID uint32, slot, count int) {
	if n := z.shopNPC(p, npcID); n == nil || n.npc.role != pb.NpcRole_NPC_ROLE_SHOP {
		return
	}
	inv := &p.pl.inv
	if slot < 0 || slot >= game.BagSize || inv.bag[slot].empty() {
		return
	}
	s := inv.bag[slot]
	price := s.def.SellPrice()
	if price <= 0 {
		z.system(p, "A loja não compra este item.")
		return
	}
	count = min(max(count, 1), s.count)
	inv.take(slot, count)
	inv.gold += price * int64(count)
	z.trackGold(p, "sell", price*int64(count))
	z.trackItem(p, false, s.def.ID, count, "sell")
	z.system(p, fmt.Sprintf("Você vendeu %s por %d moedas.", itemLabel(s.def, count), price*int64(count)))
	z.inventoryChanged(p, false)
}
