package game

import (
	"math/rand/v2"
	"testing"
)

func TestCatalogIsConsistent(t *testing.T) {
	for id, d := range Items {
		if d.ID != id || d.Name == "" || d.Icon == "" {
			t.Errorf("%s: bad def", id)
		}
		if d.Equippable() && (d.Stack != 1 || d.Slot >= EquipSlots) {
			t.Errorf("%s: equipment must not stack", id)
		}
		if d.Kind == KindWeapon && d.Model == "" {
			t.Errorf("%s: weapon without model", id)
		}
	}
	for _, list := range [][]string{MerchantStock, SmithStock} {
		for _, id := range list {
			if Items[id] == nil || Items[id].Price <= 0 {
				t.Errorf("shop item %s missing or free", id)
			}
		}
	}
	for c := range Classes {
		for _, it := range StarterKit(c) {
			d := Items[it.ItemID]
			if d == nil || (it.Equipped && (d.Slot != it.Slot || !d.UsableBy(c))) {
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
