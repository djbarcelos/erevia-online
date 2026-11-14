package game

import (
	"math/rand/v2"
	"testing"
)

// O equipamento ponte existe para as 4 classes, nos níveis 30 e 35, comum e raro, e o salto
// de poder é suave: o 35 fica acima do 30 e no máximo ~15% acima (o mesmo do raro).
func TestBridgeGearStepIsSmooth(t *testing.T) {
	power := func(s Stats) float64 {
		return s.PAtk + s.MAtk + s.PDef + s.MDef + s.MaxHP/5 + s.MaxMP/5 + s.Crit*400 + s.Evasion
	}
	epic := map[string]string{"knight": "sword_epic", "barbarian": "axe_epic", "mage": "staff_epic", "rogue": "daggers_epic"}
	for _, c := range []string{"knight", "barbarian", "mage", "rogue"} {
		for _, pick := range []func(string, int, Grade) string{BridgeWeaponID, BridgeArmorID} {
			var prev *ItemDef
			for _, lvl := range BridgeLevels {
				common, rare := Items[pick(c, lvl, GradeCommon)], Items[pick(c, lvl, GradeRare)]
				if common == nil || rare == nil {
					t.Fatalf("%s: falta a peça de nível %d", c, lvl)
				}
				for _, d := range []*ItemDef{common, rare} {
					if !d.Bridge || d.ReqLevel != lvl || !d.UsableBy(c) || len(d.Classes) != 1 {
						t.Fatalf("%s: %+v", d.ID, d)
					}
				}
				if r := power(rare.Bonus) / power(common.Bonus); r < 1.05 || r > 1.18 { // 15% mais o arredondamento
					t.Errorf("%s: o raro vale %.2f× o comum", rare.ID, r)
				}
				if prev != nil {
					if r := power(common.Bonus) / power(prev.Bonus); r < 1.05 || r > 1.15 {
						t.Errorf("%s: o comum de 35 vale %.2f× o de 30", common.ID, r)
					}
				}
				prev = common
			}
		}
		// O comum de 30 não fica abaixo do épico de 18 (senão ninguém usa).
		if w, e := Items[BridgeWeaponID(c, 30, GradeCommon)], Items[epic[c]]; power(w.Bonus) < power(e.Bonus) {
			t.Errorf("%s: %s fica abaixo de %s", c, w.ID, e.ID)
		}
	}
}

// Fora do sorteio por raridade: o equipamento ponte só cai de quem o tem no saque.
func TestBridgeGearOnlyFromItsMonsters(t *testing.T) {
	for _, pool := range gearByGrade {
		for _, d := range pool {
			if d.Bridge {
				t.Fatalf("%s no sorteio por raridade", d.ID)
			}
		}
	}
	rng := rand.New(rand.NewPCG(3, 3))
	for _, id := range []string{"skeleton_mage", "stone_golem", "bone_lord"} {
		for range 3000 {
			for _, d := range RollLoot(Monsters[id], 25, rng) {
				if d.Item.Bridge {
					t.Fatalf("%s deixou cair %s", id, d.Item.ID)
				}
			}
		}
	}
	got := map[int]bool{}
	for _, id := range []string{"ancient_golem", "ossuary_knight", "broken_reaper", "ozrhal"} {
		for range 3000 {
			for _, d := range RollLoot(Monsters[id], 35, rng) {
				if d.Item.Bridge {
					got[d.Item.ReqLevel] = true
				}
			}
		}
	}
	if !got[30] || !got[35] {
		t.Fatalf("as elites não deixam cair os conjuntos: %v", got)
	}
}
