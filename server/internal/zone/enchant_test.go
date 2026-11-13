package zone

import (
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// enchantSetup: guerreiro com a arma inicial equipada e pergaminhos na bolsa.
func enchantSetup(t *testing.T) (*Zone, *entity, *fakePeer) {
	z := newCombatZone()
	p, peer := named(z, 1, "Ferreiro", "knight", 30, 0)
	p.pl.inv.add(item("scroll_ench_weapon"), 200)
	p.pl.inv.add(item("scroll_ench_armor"), 5)
	p.pl.inv.add(item("scroll_ench_weapon_b"), 50)
	if p.pl.inv.equip[game.SlotWeapon].empty() {
		t.Fatal("the starter kit should have a weapon equipped")
	}
	return z, p, peer
}

func useScroll(z *Zone, p *entity, id string, place, slot int) {
	z.handle(itemEvent{id: p.id, msg: &pb.Enchant{ScrollSlot: uint32(p.pl.inv.slotOf(id)), Place: uint32(place), Slot: uint32(slot)}})
	z.tickFor(0.2) // repõe os tokens de ação
}

func TestEnchantSafeLevelsAndBonus(t *testing.T) {
	z, p, peer := enchantSetup(t)
	w := &p.pl.inv.equip[game.SlotWeapon]
	atk := p.stats.PAtk
	scrolls := p.pl.inv.countOf("scroll_ench_weapon")
	for range game.EnchantSafe {
		useScroll(z, p, "scroll_ench_weapon", placeEquip, game.SlotWeapon)
	}
	if w.enchant != game.EnchantSafe {
		t.Fatalf("up to +%d never fails: %d", game.EnchantSafe, w.enchant)
	}
	if p.pl.inv.countOf("scroll_ench_weapon") != scrolls-game.EnchantSafe {
		t.Fatal("each try uses one scroll")
	}
	want := game.EnchantBonus(w.def, game.EnchantSafe).PAtk
	if want <= 0 || p.stats.PAtk-atk < want-0.01 {
		t.Fatalf("enchant bonus not applied: +%.1f, want +%.1f", p.stats.PAtk-atk, want)
	}
	if !peer.has(func(m *pb.ServerMessage) bool {
		a := m.GetAppearance()
		return a != nil && a.Id == p.id && a.WeaponEnchant == game.EnchantSafe
	}) {
		t.Fatal("others should see the weapon enchant")
	}
	if r := (&p.pl.inv).records(); r[0].Enchant != game.EnchantSafe {
		t.Fatalf("enchant is saved: %+v", r[0])
	}
}

func TestEnchantFailureBreaksOrResets(t *testing.T) {
	z, p, peer := enchantSetup(t)
	w := &p.pl.inv.equip[game.SlotWeapon]
	def := w.def

	// Abençoado: falhar volta a +0, sem quebrar.
	reset := false
	for range 50 {
		w.enchant = game.MaxEnchant - 1 // próxima tentativa: 40%
		useScroll(z, p, "scroll_ench_weapon_b", placeEquip, game.SlotWeapon)
		if w.enchant == 0 {
			reset = true
			break
		}
	}
	if !reset || w.def != def {
		t.Fatal("a blessed failure resets to +0 and keeps the item")
	}

	// Comum: falhar quebra.
	broke := false
	for range 50 {
		if w.empty() {
			broke = true
			break
		}
		w.enchant = game.MaxEnchant - 1
		useScroll(z, p, "scroll_ench_weapon", placeEquip, game.SlotWeapon)
	}
	if !broke || !peer.systemContains("quebrou") {
		t.Fatal("a normal failure above the safe level destroys the item")
	}
	if !peer.has(func(m *pb.ServerMessage) bool { r := m.GetEnchantResult(); return r != nil && r.Broke }) {
		t.Fatal("the client gets a broke result")
	}
}

func TestEnchantChanceAboveSafe(t *testing.T) {
	z, p, _ := enchantSetup(t)
	p.pl.inv.add(item("scroll_ench_weapon"), 799)
	w := &p.pl.inv.equip[game.SlotWeapon]
	def := w.def
	ok, tries := 0, 0
	for range 400 {
		*w = stack{def: def, count: 1, enchant: game.EnchantSafe}
		useScroll(z, p, "scroll_ench_weapon", placeEquip, game.SlotWeapon)
		tries++
		if !w.empty() && w.enchant == game.EnchantSafe+1 {
			ok++
		}
	}
	rate := float64(ok) / float64(tries)
	if rate < 0.62 || rate > 0.78 {
		t.Fatalf("+%d→+%d success rate %.2f, want about %.2f", game.EnchantSafe, game.EnchantSafe+1, rate, game.EnchantChance(game.EnchantSafe+1))
	}
}

func TestEnchantRefusesWrongScroll(t *testing.T) {
	z, p, peer := enchantSetup(t)
	armorScrolls := p.pl.inv.countOf("scroll_ench_armor")
	useScroll(z, p, "scroll_ench_armor", placeEquip, game.SlotWeapon)
	if p.pl.inv.equip[game.SlotWeapon].enchant != 0 || p.pl.inv.countOf("scroll_ench_armor") != armorScrolls {
		t.Fatal("armor scrolls do not work on weapons (and are not used)")
	}
	if !peer.systemContains("não armas") {
		t.Fatal("the player should be told why")
	}
	// Itens que empilham (poções) não se encantam.
	slot := p.pl.inv.slotOf("potion_hp_s")
	useScroll(z, p, "scroll_ench_armor", placeBag, slot)
	if p.pl.inv.countOf("scroll_ench_armor") != armorScrolls {
		t.Fatal("stackable items cannot be enchanted")
	}
}

func TestEnchantedItemsDoNotStack(t *testing.T) {
	var inv inventory
	inv.put(item("ring_common"), 1, 4)
	inv.put(item("ring_common"), 1, 0)
	if inv.bag[0].enchant != 4 || inv.bag[1].enchant != 0 || inv.bag[1].empty() {
		t.Fatalf("enchanted items keep their own slot: %+v %+v", inv.bag[0], inv.bag[1])
	}
}
