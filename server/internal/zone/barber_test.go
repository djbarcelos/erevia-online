package zone

import (
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

func (z *Zone) barber() *entity {
	for _, n := range z.npcs {
		if n.npc.role == pb.NpcRole_NPC_ROLE_BARBER {
			return n
		}
	}
	return nil
}

func changeLookReq(z *Zone, p *entity, npcID uint32, l *pb.BodyLook) {
	z.handle(itemEvent{id: p.id, msg: &pb.ChangeLook{NpcId: npcID, Look: l}})
}

func TestBarberChangesLookForGold(t *testing.T) {
	z := newCombatZone()
	b := z.barber()
	if b == nil {
		t.Fatal("no barber in town")
	}
	// O barbeiro aparece com o próprio visual para todos.
	if l := z.entityPB(b).Look; l == nil || l.Head != "mage" {
		t.Fatalf("barber look = %+v", l)
	}
	p, peer := named(z, 1, "Cliente", "knight", b.pos.X+1, b.pos.Z)
	_, other := named(z, 2, "Vizinho", "mage", b.pos.X-1, b.pos.Z)
	want := &pb.BodyLook{Skin: 3, Hair: 2, Outfit: 6, Head: "rogue", HideHat: true}

	p.pl.inv.gold = game.BarberPrice - 1
	changeLookReq(z, p, b.id, want)
	if p.pl.look != (game.Appearance{}) || !peer.systemContains("Moedas insuficientes") {
		t.Fatal("not enough gold: nothing changes")
	}

	p.pl.inv.gold = game.BarberPrice + 10
	changeLookReq(z, p, b.id, &pb.BodyLook{Skin: 99})
	if p.pl.look != (game.Appearance{}) || p.pl.inv.gold != game.BarberPrice+10 {
		t.Fatal("invalid look must be refused")
	}

	other.reset()
	changeLookReq(z, p, b.id, want)
	if p.pl.look != (game.Appearance{Skin: 3, Hair: 2, Outfit: 6, Head: "rogue", HideHat: true}) || p.pl.inv.gold != 10 {
		t.Fatalf("look %+v gold %d", p.pl.look, p.pl.inv.gold)
	}
	if !other.has(func(m *pb.ServerMessage) bool {
		a := m.GetAppearance()
		return a != nil && a.Id == p.id && a.Look.GetHead() == "rogue" && a.Look.GetHideHat()
	}) {
		t.Fatal("others should see the new look")
	}
	if l := z.entityPB(p).Look; l.GetOutfit() != 6 {
		t.Fatalf("spawn look = %+v", l)
	}
	if s := z.savedState(p); s.Look != p.pl.look {
		t.Fatalf("saved look = %+v", s.Look)
	}

	// Pedir o mesmo visual de novo não cobra.
	p.pl.inv.gold = game.BarberPrice
	changeLookReq(z, p, b.id, want)
	if p.pl.inv.gold != game.BarberPrice {
		t.Fatal("same look must be free (refused)")
	}
	// A cabeça da própria classe é o padrão: igual a "".
	changeLookReq(z, p, b.id, &pb.BodyLook{Head: "knight"})
	if p.pl.look != (game.Appearance{}) || p.pl.inv.gold != 0 {
		t.Fatalf("back to default: %+v gold %d", p.pl.look, p.pl.inv.gold)
	}
}

func TestBarberNeedsRightNPCAndDistance(t *testing.T) {
	z := newCombatZone()
	b := z.barber()
	p, peer := named(z, 1, "Longe", "rogue", b.pos.X+20, b.pos.Z)
	p.pl.inv.gold = 1000
	l := &pb.BodyLook{Hair: 1}
	changeLookReq(z, p, b.id, l)
	if p.pl.look.Hair != 0 || !peer.systemContains("Chegue mais perto") {
		t.Fatal("too far")
	}
	// Uma loja não é barbeiro.
	var shop *entity
	for _, n := range z.npcs {
		if n.npc.role == pb.NpcRole_NPC_ROLE_SHOP && n.model == "npc_merchant" {
			shop = n
		}
	}
	p.pos = shop.pos
	changeLookReq(z, p, shop.id, l)
	if p.pl.look.Hair != 0 || p.pl.inv.gold != 1000 {
		t.Fatal("only the barber changes looks")
	}
}

func TestLookSurvivesQuickRelog(t *testing.T) {
	z := newCombatZone()
	b := z.barber()
	p, _ := named(z, 1, "Volta", "barbarian", b.pos.X+1, b.pos.Z)
	p.pl.inv.gold = game.BarberPrice
	changeLookReq(z, p, b.id, &pb.BodyLook{Hair: 4, HideHat: true})
	z.handle(leaveEvent{id: 1})
	p, _ = named(z, 1, "Volta", "barbarian", 0, 0)
	if p.pl.look != (game.Appearance{Hair: 4, HideHat: true}) {
		t.Fatalf("look after relog = %+v", p.pl.look)
	}
}
