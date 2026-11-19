package zone

import (
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Cristalização e encantamento por grau (tunnel#154).

func crystallizeEv(z *Zone, p *entity, slot int) {
	p.pl.moveTokens = z.cfg.MaxMovesPerSec
	z.handle(itemEvent{id: p.id, msg: &pb.Crystallize{Slot: uint32(slot)}})
}

func TestCrystallizeTurnsGradedGearIntoCrystals(t *testing.T) {
	z := newCombatZone()
	z.SetEventSink(nopSink{})
	p, peer := joinAt(z, 1, "knight", 30, 0, 0)
	inv := &p.pl.inv
	inv.bag = [game.BagSize]stack{}
	sword := item("sword_d16")
	inv.put(sword, 1, 4)
	slot := inv.slotOf(sword.ID)
	crystal, want := game.CrystalYield(sword, 4)
	if crystal != game.CrystalD || want <= game.CrystalBase(sword) {
		t.Fatalf("a D +4 rende cristais D a mais que a +0: %s ×%d (base %d)", crystal, want, game.CrystalBase(sword))
	}
	drainEvents(z)
	crystallizeEv(z, p, slot)
	if inv.countOf(sword.ID) != 0 || inv.countOf(game.CrystalD) != want {
		t.Fatalf("depois de cristalizar: %d espadas, %d cristais (quer 0 e %d)", inv.countOf(sword.ID), inv.countOf(game.CrystalD), want)
	}
	evs := drainEvents(z)
	if e := findEvent(evs, "item_out", sword.ID); e == nil || e.Extra["how"] != "crystal" {
		t.Errorf("a peça sai com how=crystal: %+v", e)
	}
	if e := findEvent(evs, "item_in", game.CrystalD); e == nil || e.Amount != int64(want) || e.Extra["how"] != "crystal" {
		t.Errorf("os cristais entram com how=crystal: %+v", e)
	}
	if !peer.systemContains("virou") {
		t.Error("o jogador é avisado")
	}
}

func TestCrystallizeRefusesUngradedAndUsesTheFreedSlot(t *testing.T) {
	z := newCombatZone()
	p, peer := joinAt(z, 1, "knight", 30, 0, 0)
	inv := &p.pl.inv
	inv.bag = [game.BagSize]stack{}

	// Sem grau (a loja do Ferreiro): não cristaliza.
	inv.add(item("sword_common"), 1)
	crystallizeEv(z, p, inv.slotOf("sword_common"))
	if inv.countOf("sword_common") != 1 || !peer.systemContains("grau D ou maior") {
		t.Fatal("peça sem grau não se cristaliza")
	}

	// Bolsa cheia: os cristais vão para o espaço que a peça deixou.
	for i := range inv.bag {
		inv.bag[i] = stack{def: item("potion_hp_s"), count: 1}
	}
	b := game.TierPieces(game.TierB, game.GradeCommon)[0]
	inv.bag[5] = stack{def: b, count: 1}
	crystallizeEv(z, p, 5)
	if inv.countOf(b.ID) != 0 || inv.bag[5].empty() || inv.bag[5].def.ID != game.CrystalB {
		t.Fatalf("a peça B vira cristais B no espaço dela: %+v", inv.bag[5])
	}
}

func TestEnchantNeedsScrollOfTheTierAndPaysFee(t *testing.T) {
	z := newCombatZone()
	p, peer := joinAt(z, 1, "knight", 30, 0, 0)
	inv := &p.pl.inv
	inv.bag = [game.BagSize]stack{}
	c := item("sword_c26")
	inv.put(c, 1, 0)
	inv.add(item("scroll_ench_weapon"), 1)    // D
	inv.add(item("scroll_ench_weapon_tc"), 2) // C
	target := inv.slotOf(c.ID)

	// Pergaminho do D numa peça do C: recusa sem gastar.
	inv.gold = 1_000_000
	useScroll(z, p, "scroll_ench_weapon", placeBag, target)
	if inv.bag[target].enchant != 0 || inv.countOf("scroll_ench_weapon") != 1 || inv.gold != 1_000_000 {
		t.Fatal("o pergaminho do D não encanta a peça do C (e não é gasto)")
	}
	if !peer.systemContains("grau D") {
		t.Fatal("o jogador sabe por quê")
	}

	// Sem moedas para a taxa: recusa sem gastar o pergaminho.
	fee := game.EnchantFee(game.TierC, 1)
	inv.gold = fee - 1
	useScroll(z, p, "scroll_ench_weapon_tc", placeBag, target)
	if inv.bag[target].enchant != 0 || inv.countOf("scroll_ench_weapon_tc") != 2 || inv.gold != fee-1 {
		t.Fatal("sem moedas para a taxa, nada acontece")
	}

	// Com as moedas: +1 (seguro), cobra a taxa e o pergaminho.
	inv.gold = fee
	useScroll(z, p, "scroll_ench_weapon_tc", placeBag, target)
	if inv.bag[target].enchant != 1 || inv.countOf("scroll_ench_weapon_tc") != 1 || inv.gold != 0 {
		t.Fatalf("encantar o C com o pergaminho do C: +%d, %d pergaminhos, %d moedas", inv.bag[target].enchant, inv.countOf("scroll_ench_weapon_tc"), inv.gold)
	}

	// O do D continua servindo nas peças sem grau.
	inv.add(item("sword_common"), 1)
	inv.gold = 1000
	useScroll(z, p, "scroll_ench_weapon", placeBag, inv.slotOf("sword_common"))
	if inv.bag[inv.slotOf("sword_common")].enchant != 1 {
		t.Fatal("o pergaminho do D encanta a peça sem grau")
	}
}

func TestCraftGradeCWithCrystals(t *testing.T) {
	z, p, _, smith := atSmith(t)
	r := game.Recipes["craft_sword_c26"]
	if r == nil {
		t.Fatal("a espada comum do C sai do Ferreiro")
	}
	p.pl.inv.gold = r.Gold
	for _, m := range r.Materials {
		p.pl.inv.add(game.Items[m.ID], m.Count)
	}
	craftEv(z, p, smith, r.ID)
	if p.pl.inv.countOf("sword_c26") != 1 || p.pl.inv.countOf(game.CrystalC) != 0 || p.pl.inv.gold != 0 {
		t.Fatalf("criar com cristais: espada %d, cristais %d, moedas %d", p.pl.inv.countOf("sword_c26"), p.pl.inv.countOf(game.CrystalC), p.pl.inv.gold)
	}
}
