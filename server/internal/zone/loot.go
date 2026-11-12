package zone

import (
	"fmt"
	"math"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

const (
	groundIDBase  = uint32(1) << 30 // itens no chão: entre os NPCs e os monstros
	groundIDSpan  = uint32(1) << 29
	reservedFor   = 15.0 // segundos em que só quem mais causou dano pode pegar
	groundLife    = 60.0 // segundos até o item sumir
	maxGround     = 500  // limite de itens no chão na zona
	pickupRange   = 1.2
	pickupMaxDist = 40.0 // não atravessa o mapa para pegar algo
	dropScatter   = 1.5
)

// groundItem é um item caído no chão.
type groundItem struct {
	id            uint32
	def           *game.ItemDef
	count         int
	pos           Vec2
	owner         uint32
	reservedUntil float64
	expires       float64
}

// dropLoot sorteia o que o monstro deixa e espalha perto do corpo.
func (z *Zone) dropLoot(m *entity, owner uint32) {
	drops := game.RollLoot(m.mon.def, m.level, z.rng)
	if len(drops) == 0 {
		return
	}
	added := &pb.GroundItemsAdded{}
	for _, d := range drops {
		if len(z.ground) >= maxGround {
			z.log.Warn("too many items on the ground, skipping drop", "item", d.Item.ID)
			break
		}
		pos := m.pos
		if len(drops) > 1 || d.Item.Kind != game.KindGold {
			a := z.rng.Float64() * 2 * math.Pi
			r := 0.4 + z.rng.Float64()*(dropScatter-0.4)
			if spot := (Vec2{X: m.pos.X + r*math.Cos(a), Z: m.pos.Z + r*math.Sin(a)}); z.walkable(spot) {
				pos = spot
			}
		}
		g := &groundItem{
			id: z.nextGroundID(), def: d.Item, count: d.Count, pos: pos,
			owner: owner, reservedUntil: z.now + reservedFor, expires: z.now + groundLife,
		}
		z.ground[g.id] = g
		added.Items = append(added.Items, z.groundPB(g))
	}
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_GroundAdded{GroundAdded: added}}, 0)
}

func (z *Zone) nextGroundID() uint32 {
	for {
		z.groundSeq = (z.groundSeq + 1) % groundIDSpan
		id := groundIDBase + z.groundSeq
		if _, used := z.ground[id]; !used {
			return id
		}
	}
}

func (z *Zone) groundPB(g *groundItem) *pb.GroundItem {
	out := &pb.GroundItem{
		Id: g.id, ItemId: g.def.ID, Count: uint32(g.count),
		X: float32(g.pos.X), Z: float32(g.pos.Z),
	}
	if left := g.reservedUntil - z.now; g.owner != 0 && left > 0 {
		out.OwnerId = g.owner
		out.ReservedMs = uint32(left * 1000)
	}
	return out
}

func (z *Zone) groundList() []*pb.GroundItem {
	out := make([]*pb.GroundItem, 0, len(z.ground))
	for _, g := range z.ground {
		out = append(out, z.groundPB(g))
	}
	return out
}

// expireGround tira do chão os itens que venceram.
func (z *Zone) expireGround() {
	var gone []uint32
	for id, g := range z.ground {
		if z.now >= g.expires {
			gone = append(gone, id)
			delete(z.ground, id)
		}
	}
	if len(gone) > 0 {
		z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_GroundRemoved{GroundRemoved: &pb.GroundItemsRemoved{Ids: gone}}}, 0)
	}
}

// requestPickup: o jogador clicou num item; anda até ele e pega (ver runPickup).
func (z *Zone) requestPickup(p *entity, id uint32) {
	g := z.ground[id]
	if p.dead || g == nil || dist(p.pos, g.pos) > pickupMaxDist {
		return
	}
	if p.stunned(z.now) {
		return
	}
	p.act = action{kind: actPickup, target: id}
}

// runPickup é a ação de pegar: aproxima e, ao chegar, guarda o item.
func (z *Zone) runPickup(p *entity) {
	g := z.ground[p.act.target]
	if g == nil {
		p.clearAction()
		return
	}
	if dist(p.pos, g.pos) > pickupRange {
		z.moveToward(p, g.pos)
		return
	}
	p.stopMoving()
	p.clearAction()
	if g.owner != 0 && g.owner != p.id && z.now < g.reservedUntil {
		z.system(p, "Este item ainda está reservado para quem derrotou o monstro.")
		return
	}
	inv := &p.pl.inv
	if g.def.Kind == game.KindGold {
		inv.gold += int64(g.count)
	} else {
		if !inv.canAdd(g.def, g.count) {
			z.system(p, "Sua bolsa está cheia.")
			return
		}
		inv.add(g.def, g.count)
	}
	delete(z.ground, g.id)
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_GroundRemoved{GroundRemoved: &pb.GroundItemsRemoved{
		Ids: []uint32{g.id}, PickedBy: p.id,
	}}}, 0)
	z.system(p, fmt.Sprintf("Você pegou %s.", itemLabel(g.def, g.count)))
	z.inventoryChanged(p, false)
}
