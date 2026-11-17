package zone

import (
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// atSmith põe um jogador ao lado do Ferreiro e devolve o id dele.
func atSmith(t *testing.T) (*Zone, *entity, *fakePeer, uint32) {
	t.Helper()
	z := newCombatZone()
	var smith *entity
	for _, n := range z.npcs {
		if n.npc.crafts {
			smith = n
		}
	}
	if smith == nil {
		t.Fatal("no crafting NPC")
	}
	p, peer := joinAt(z, 1, "knight", smith.pos.X+1, smith.pos.Z, 0)
	p.pl.inv = inventory{}
	return z, p, peer, smith.id
}

func craftEv(z *Zone, p *entity, npc uint32, recipe string) {
	p.pl.moveTokens = z.cfg.MaxMovesPerSec
	z.handle(itemEvent{id: p.id, msg: &pb.Craft{NpcId: npc, RecipeId: recipe}})
}

func TestCraftConsumesMaterialsAndGold(t *testing.T) {
	z, p, peer, smith := atSmith(t)
	r := game.Recipes["craft_ring_rare"]
	p.pl.inv.gold = r.Gold + 5
	p.pl.inv.add(game.Items["bone_old"], 35)
	p.pl.inv.add(game.Items["skull_cracked"], 10)

	craftEv(z, p, smith, r.ID)
	if p.pl.inv.countOf("ring_rare") != 1 {
		t.Fatal("ring should be crafted")
	}
	if p.pl.inv.countOf("bone_old") != 5 || p.pl.inv.countOf("skull_cracked") != 0 || p.pl.inv.gold != 5 {
		t.Fatalf("left: bones=%d skulls=%d gold=%d", p.pl.inv.countOf("bone_old"), p.pl.inv.countOf("skull_cracked"), p.pl.inv.gold)
	}
	if !peer.systemContains("criou") {
		t.Fatal("expected a crafted message")
	}
}

func TestCraftRefusesWithoutMaterialsGoldOrRange(t *testing.T) {
	z, p, peer, smith := atSmith(t)
	r := game.Recipes["craft_ring_rare"]
	p.pl.inv.gold = r.Gold
	p.pl.inv.add(game.Items["bone_old"], 30)
	p.pl.inv.add(game.Items["skull_cracked"], 9)
	before := p.pl.inv

	craftEv(z, p, smith, r.ID)
	if p.pl.inv != before || !peer.systemContains("Faltam 1×") {
		t.Fatal("missing one skull must refuse without touching the bag")
	}

	p.pl.inv.add(game.Items["skull_cracked"], 1)
	p.pl.inv.gold = r.Gold - 1
	before = p.pl.inv
	craftEv(z, p, smith, r.ID)
	if p.pl.inv != before || !peer.systemContains("Moedas insuficientes") {
		t.Fatal("not enough gold must refuse")
	}

	p.pl.inv.gold = r.Gold
	p.teleportTo(Vec2{X: 60, Z: 60})
	before = p.pl.inv
	craftEv(z, p, smith, r.ID)
	if p.pl.inv != before {
		t.Fatal("crafting from far away must do nothing")
	}
}

func TestCraftOnlyAtTheSmith(t *testing.T) {
	z, p, _, _ := atSmith(t)
	r := game.Recipes["craft_ench_armor"]
	p.pl.inv.gold = 1000
	p.pl.inv.add(game.Items["bone_old"], 15)
	p.pl.inv.add(game.Items["skull_cracked"], 2)
	for _, n := range z.npcs {
		if !n.npc.crafts {
			p.teleportTo(Vec2{X: n.pos.X + 1, Z: n.pos.Z})
			before := p.pl.inv
			craftEv(z, p, n.id, r.ID)
			if p.pl.inv != before {
				t.Fatalf("%s must not craft", n.name)
			}
		}
	}
}

func TestCraftWithFullBagKeepsMaterials(t *testing.T) {
	z, p, peer, smith := atSmith(t)
	r := game.Recipes["craft_ench_armor"]
	p.pl.inv.gold = 1000
	filler := game.Items["ring_common"]
	for i := range p.pl.inv.bag {
		p.pl.inv.bag[i] = stack{def: filler, count: 1}
	}
	// Materiais que sobram depois de criar: as pilhas continuam ocupando os espaços.
	p.pl.inv.bag[0] = stack{def: game.Items["bone_old"], count: 20}
	p.pl.inv.bag[1] = stack{def: game.Items["skull_cracked"], count: 5}
	before := p.pl.inv
	craftEv(z, p, smith, r.ID)
	if p.pl.inv != before || !peer.systemContains("bolsa está cheia") {
		t.Fatal("full bag must refuse and keep materials")
	}
}
