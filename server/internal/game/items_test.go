package game

import (
	"math/rand/v2"
	"testing"
)

func TestCatalogIsConsistent(t *testing.T) {
	perSlot := map[int]map[Grade]bool{}
	for id, d := range Items {
		if d.ID != id || d.Name == "" || d.Icon == "" {
			t.Errorf("%s: bad def", id)
		}
		if d.Equippable() && d.Stack != 1 {
			t.Errorf("%s: equipment must not stack", id)
		}
		for _, sl := range d.Slots() {
			if sl < 0 || sl >= EquipSlots {
				t.Errorf("%s: slot %d out of range", id, sl)
			}
			if perSlot[sl] == nil {
				perSlot[sl] = map[Grade]bool{}
			}
			perSlot[sl][d.Grade] = true
		}
		if d.Kind == KindWeapon && (d.Model == "" || d.WeaponType == "") {
			t.Errorf("%s: weapon without model or type", id)
		}
		if d.Equippable() && d.Price <= 0 {
			t.Errorf("%s: equipment without price", id)
		}
	}
	// Todos os 13 encaixes têm itens das 4 raridades (anéis e brincos usam os dois encaixes).
	for sl := 0; sl < EquipSlots; sl++ {
		for _, g := range []Grade{GradeCommon, GradeGood, GradeRare, GradeEpic} {
			if !perSlot[sl][g] {
				t.Errorf("slot %d (%s) has no grade %d item", sl, SlotNames[sl], g)
			}
		}
	}
	for _, list := range [][]string{MerchantStock, SmithStock} {
		seen := map[string]bool{}
		for _, id := range list {
			if Items[id] == nil || Items[id].Price <= 0 || seen[id] {
				t.Errorf("shop item %s missing, free or repeated", id)
			}
			seen[id] = true
		}
	}
	for c := range Classes {
		for _, it := range StarterKit(c) {
			d := Items[it.ItemID]
			if d == nil || (it.Equipped && (!d.Fits(it.Slot) || !d.UsableBy(c))) {
				t.Errorf("%s starter kit: bad %+v", c, it)
			}
		}
	}
	for _, m := range Monsters {
		for _, e := range m.Loot {
			if Items[e.ItemID] == nil {
				t.Errorf("%s drops unknown %s", m.ID, e.ItemID)
			}
		}
	}
}

func TestStarterLooks(t *testing.T) {
	for c := range Classes {
		o, ok := StarterLooks[c]
		if !ok || len(o.Weapons) == 0 || len(o.Offhands) == 0 || !ValidEmblem(o.Emblem) {
			t.Fatalf("%s: bad options %+v", c, o)
		}
		if !ValidLook(c, DefaultLook(c)) {
			t.Errorf("%s: default look invalid", c)
		}
		// Toda opção é um item comum de nível 1 que a classe pode usar no encaixe certo.
		for _, id := range o.Weapons {
			d := Items[id]
			if d == nil || d.Kind != KindWeapon || !d.UsableBy(c) || d.ReqLevel != 1 || d.Grade != GradeCommon {
				t.Errorf("%s: bad weapon option %s", c, id)
			}
		}
		for _, id := range o.Offhands {
			if id == "" {
				continue
			}
			if d := Items[id]; d == nil || d.Kind != KindShield || !d.UsableBy(c) || d.ReqLevel != 1 {
				t.Errorf("%s: bad offhand option %s", c, id)
			}
		}
		// Cada combinação gera um kit válido.
		for _, w := range o.Weapons {
			for _, off := range o.Offhands {
				look := Look{Weapon: w, Offhand: off, Emblem: Emblems[0]}
				kit := StarterKitFor(c, look)
				if kit[0] != (ItemRecord{Equipped: true, Slot: SlotWeapon, ItemID: w, Count: 1}) {
					t.Errorf("%s %+v: kit %+v", c, look, kit)
				}
				shields := 0
				for _, it := range kit {
					if Items[it.ItemID].Kind == KindShield {
						shields++
						if it.Equipped != (off != "") {
							t.Errorf("%s %+v: shield placement %+v", c, look, it)
						}
					}
				}
				if want := map[bool]int{true: 1, false: 0}[o.Offhands[0] != ""]; shields != want {
					t.Errorf("%s %+v: %d shields", c, look, shields)
				}
			}
		}
	}
	bad := []Look{
		{Weapon: "axe_common", Offhand: "shield_badge", Emblem: "#d6b46e"},   // machado não é do cavaleiro
		{Weapon: "sword_good", Offhand: "shield_badge", Emblem: "#d6b46e"},   // não é opção inicial
		{Weapon: "sword_common", Offhand: "shield_good", Emblem: "#d6b46e"},  // escudo melhor não
		{Weapon: "sword_common", Offhand: "shield_badge", Emblem: "#000000"}, // cor fora da lista
		{Weapon: "sword_common", Offhand: "potion_hp_s", Emblem: "#d6b46e"},
	}
	for _, l := range bad {
		if ValidLook("knight", l) {
			t.Errorf("accepted %+v", l)
		}
	}
	if ValidLook("mage", Look{Weapon: "staff_common", Offhand: "shield_badge", Emblem: "#8c6fd6"}) {
		t.Error("mage with shield accepted")
	}
	if EmblemOr("rogue", "") != "#5fae7d" || EmblemOr("rogue", "#d04050") != "#d04050" || EmblemOr("rogue", "red") != "#5fae7d" {
		t.Error("EmblemOr")
	}
}

func TestEquipmentBonusStats(t *testing.T) {
	base := ClassStats("rogue", 10)
	base = ApplyAttrs(base, "rogue", 10, ClassBaseAttrs["rogue"])
	var bonus Stats
	for _, id := range []string{"gloves_epic", "boots_epic", "cape_epic", "ring_epic", "ring_rare", "ear_epic"} {
		bonus = bonus.AddBonus(Items[id].Bonus)
	}
	if bonus.AtkInterval != 0 || bonus.AtkSpeed != 0.08 || bonus.Speed != 0.5 {
		t.Fatalf("summing bonuses: %+v", bonus)
	}
	s := base.AddBonus(bonus)
	s = FinishRegen(s, "rogue", ClassBaseAttrs["rogue"])
	plain := FinishRegen(base, "rogue", ClassBaseAttrs["rogue"])
	near := func(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }
	switch {
	case !near(1/s.AtkInterval, (1/base.AtkInterval)*1.08):
		t.Errorf("attack speed %.4f vs %.4f", 1/s.AtkInterval, 1/base.AtkInterval)
	case !near(s.Speed, base.Speed+0.5):
		t.Errorf("speed %.2f", s.Speed)
	case !near(s.CastSpeed, base.CastSpeed+0.08+0.06):
		t.Errorf("cast speed %.2f", s.CastSpeed)
	case !near(s.CritDamage, base.CritDamage+0.3):
		t.Errorf("crit damage %.2f", s.CritDamage)
	case !near(s.HPRegen, plain.HPRegen+2+60*0.008): // +2/s da capa e a vida extra dela
		t.Errorf("hp regen %.3f vs %.3f", s.HPRegen, plain.HPRegen)
	case s.Evasion != base.Evasion+15:
		t.Errorf("evasion %.1f", s.Evasion)
	}
}

func TestSlotsOfKinds(t *testing.T) {
	ring := Items["ring_common"]
	if !ring.Fits(SlotRing1) || !ring.Fits(SlotRing2) || ring.Fits(SlotEar1) || ring.Slot() != SlotRing1 {
		t.Fatal("ring slots")
	}
	if Items["potion_hp_s"].Equippable() || Items["potion_hp_s"].Slot() != NoSlot {
		t.Fatal("potion equippable")
	}
	if Items["cape_rare"].Slots()[0] != SlotCape || len(SlotNames) != EquipSlots || EquipSlots != 13 {
		t.Fatal("cape / names")
	}
}

func TestRollLoot(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	gold, bones := 0, 0
	const n = 2000
	for i := 0; i < n; i++ {
		for _, d := range RollLoot(Monsters["skeleton_minion"], 3, rng) {
			switch d.Item.ID {
			case GoldID:
				gold++
				if d.Count < 6 || d.Count > 12 {
					t.Fatalf("gold %d outside 2–4 per level", d.Count)
				}
			case "bone_old":
				bones++
			}
		}
	}
	if gold < n*6/10 || gold > n*8/10 || bones < n*3/10 || bones > n*5/10 {
		t.Fatalf("gold %d bones %d of %d", gold, bones, n)
	}

	// O chefe sempre deixa a coroa e um equipamento raro.
	drops := RollLoot(Monsters["bone_lord"], 22, rng)
	crown, rare := false, false
	for _, d := range drops {
		crown = crown || d.Item.ID == "broken_crown"
		rare = rare || (d.Item.Equippable() && d.Item.Grade == GradeRare)
	}
	if !crown || !rare {
		t.Fatalf("boss drops = %+v", drops)
	}
}

func TestAppearance(t *testing.T) {
	good := []Appearance{
		{},
		{Skin: len(SkinTones), Hair: len(HairColors), Outfit: len(OutfitColors), Head: "ranger", HideHat: true},
		{Skin: 1, Hair: 1, Outfit: 1, Head: "knight"},
	}
	for _, a := range good {
		if !ValidAppearance(a) || !ValidLook("knight", Look{Weapon: "sword_common", Offhand: "", Emblem: Emblems[0], Appearance: a}) {
			t.Errorf("rejected %+v", a)
		}
	}
	bad := []Appearance{
		{Skin: -1}, {Skin: len(SkinTones) + 1}, {Hair: len(HairColors) + 1}, {Outfit: len(OutfitColors) + 1},
		{Outfit: -2}, {Head: "skeleton_mage"}, {Head: "Knight"},
	}
	for _, a := range bad {
		if ValidAppearance(a) {
			t.Errorf("accepted %+v", a)
		}
	}
	// A cabeça da própria classe vira "" e quem não tem chapéu não o esconde.
	if n := NormalAppearance("mage", Appearance{Head: "mage", HideHat: true}); n != (Appearance{HideHat: true}) {
		t.Errorf("mage: %+v", n)
	}
	if n := NormalAppearance("rogue", Appearance{Head: "knight", HideHat: true}); n != (Appearance{Head: "knight"}) {
		t.Errorf("rogue: %+v", n)
	}
	for c := range Classes {
		if HatClasses[c] == (c == "rogue") {
			t.Errorf("%s: hat %v", c, HatClasses[c])
		}
	}
}
