package game

import (
	"math/rand/v2"
	"slices"
	"testing"
)

func TestTierForLevel(t *testing.T) {
	cases := map[int]Tier{1: TierNone, 9: TierNone, 10: TierD, 19: TierD, 20: TierC, 29: TierC, 30: TierB, 35: TierB, MaxLevel: TierB}
	for l, want := range cases {
		if got := TierFor(l); got != want {
			t.Errorf("nível %d: %s, quero %s", l, got.Label(), want.Label())
		}
	}
	if TierD.MinLevel() != 10 || TierC.MinLevel() != 20 || TierB.MinLevel() != 30 || TierNone.MinLevel() != 1 {
		t.Fatal("níveis mínimos dos graus")
	}
	if TierD.Label() != "Grau D" || TierNone.Label() != "Sem grau" {
		t.Fatal("nomes dos graus")
	}
	// Todo equipamento tem o grau do nível exigido (o antigo também).
	for id, d := range Items {
		if d.Equippable() && d.Tier != TierFor(d.ReqLevel) {
			t.Errorf("%s (nível %d) é do %s", id, d.ReqLevel, d.Tier.Label())
		}
	}
	if Items["sword_rare"].Tier != TierD || Items["sword_legend"].Tier != TierC || Items["sword_b30"].Tier != TierB {
		t.Fatal("o equipamento que já existia ganha o grau do nível")
	}
}

// Cada grau tem armas para as 4 classes, um conjunto de cinco peças em cada peso (comum e
// épica, com bônus de 2 e de 4 peças) e escudo, capa e joias; as épicas e lendárias ficam
// acima das comuns do mesmo grau.
func TestEveryTierHasWeaponsSetsAndJewels(t *testing.T) {
	for _, tier := range []Tier{TierD, TierC, TierB} {
		for _, c := range tierClasses {
			common, epic := 0, 0
			for _, d := range tierGear.items {
				if d.Kind == KindWeapon && d.Tier == tier && d.UsableBy(c) && len(d.Classes) == 1 {
					switch d.Grade {
					case GradeCommon:
						common++
					case GradeEpic:
						epic++
					}
					if d.WeaponType != weaponTypeOf(c) || d.Model == "" {
						t.Errorf("%s: arma sem tipo ou modelo", d.ID)
					}
				}
			}
			if common == 0 || epic != 1 {
				t.Errorf("%s, %s: %d armas comuns e %d épicas", tier.Label(), c, common, epic)
			}
		}
		for _, w := range Weights {
			var set *ItemSet
			for _, s := range ItemSetList {
				if s.Tier == tier && s.Weight == w {
					set = s
				}
			}
			if set == nil {
				t.Fatalf("%s: falta o conjunto %s", tier.Label(), w.Name())
			}
			if len(set.Pieces) != len(setKinds) || len(set.Bonuses) != 2 || set.Bonuses[0].Pieces != 2 || set.Bonuses[1].Pieces != 4 {
				t.Errorf("%s: %+v", set.ID, set)
			}
			for i, id := range set.Pieces {
				d, e := Items[id], Items[id+"_epic"]
				if d == nil || e == nil || d.Kind != setKinds[i] || e.Kind != d.Kind || d.Set != set.ID || e.Set != set.ID ||
					d.Weight != w || e.Weight != w || d.Tier != tier || e.Tier != tier || d.Grade != GradeCommon || e.Grade != GradeEpic {
					t.Errorf("%s: peça %s errada", set.ID, id)
				}
			}
			if set.Bonus(1) != (Stats{}) || set.Bonus(3) != set.Bonuses[0].Bonus || set.Bonus(5) != set.Bonus(4) {
				t.Errorf("%s: degraus do bônus", set.ID)
			}
		}
		for _, k := range accKinds {
			var common, top *ItemDef
			for _, d := range tierGear.items {
				if d.Kind == k && d.Tier == tier {
					if d.Grade == GradeCommon {
						common = d
					} else {
						top = d
					}
				}
			}
			if common == nil || top == nil || top.ReqLevel < common.ReqLevel {
				t.Errorf("%s, %s: comum %v, épica/lendária %v", tier.Label(), accNouns[k], common, top)
			}
		}
	}
}

// Do 10 ao 40 há equipamento novo a cada 3 níveis no máximo.
func TestNewGearEveryThreeLevels(t *testing.T) {
	levels := map[int]bool{}
	for _, d := range Items {
		if d.Equippable() && !d.RepOnly {
			levels[d.ReqLevel] = true
		}
	}
	last := 10
	if !levels[last] {
		t.Fatal("nada novo no 10")
	}
	for l := 11; l <= MaxLevel; l++ {
		if levels[l] {
			if l-last > 3 {
				t.Errorf("do %d ao %d sem item novo", last, l)
			}
			last = l
		}
	}
	if MaxLevel-last > 3 {
		t.Errorf("do %d ao %d sem item novo", last, MaxLevel)
	}
}

func worn(ids ...string) []Worn {
	var out []Worn
	for _, id := range ids {
		out = append(out, Worn{Def: Items[id]})
	}
	return out
}

// EquipBonus soma as peças, o bônus do conjunto pelas peças vestidas (a épica conta) e a
// penalidade de nível uma vez só.
func TestEquipBonusSetsAndUnderLevel(t *testing.T) {
	set := ItemSets["set_heavy_d"]
	sum := func(ws []Worn) Stats {
		var b Stats
		for _, w := range ws {
			b = AddItemBonus(b, w.Def, w.Enchant)
		}
		return b
	}
	one := worn("heavy_d_armor")
	if got := EquipBonus(20, one); got != sum(one) {
		t.Fatalf("uma peça não dá bônus: %+v", got)
	}
	two := worn("heavy_d_armor", "heavy_d_helm_epic") // a épica conta como a peça
	if got, want := EquipBonus(20, two), sum(two).AddBonus(set.Bonus(2)); got != want {
		t.Fatalf("duas peças: %+v, quero %+v", got, want)
	}
	four := worn("heavy_d_armor", "heavy_d_helm", "heavy_d_legs", "heavy_d_gloves", "ring_d")
	if got, want := EquipBonus(20, four), sum(four).AddBonus(set.Bonus(4)); got != want || set.Bonus(4) == set.Bonus(2) {
		t.Fatalf("quatro peças: %+v, quero %+v", got, want)
	}
	// Dois conjuntos de pesos diferentes valem cada um pelo que tem (2 + 2 peças).
	mixed := worn("heavy_d_armor", "heavy_d_helm", "light_d_legs", "light_d_gloves")
	if got, want := EquipBonus(20, mixed), sum(mixed).AddBonus(set.Bonus(2).AddBonus(ItemSets["set_light_d"].Bonus(2))); got != want {
		t.Fatalf("2 + 2: %+v, quero %+v", got, want)
	}
	// Acima do nível: a penalidade uma vez, com uma ou com várias peças.
	if got, want := EquipBonus(12, one), sum(one).AddBonus(UnderLevelPenalty); got != want {
		t.Fatalf("uma peça acima do nível: %+v, quero %+v", got, want)
	}
	if got, want := EquipBonus(12, four), sum(four).AddBonus(set.Bonus(4)).AddBonus(UnderLevelPenalty); got != want {
		t.Fatalf("várias peças acima do nível: %+v, quero %+v", got, want)
	}
	if EquipBonus(13, one) != sum(one) {
		t.Fatal("no nível exigido a penalidade some")
	}
	// Pura: a ordem das peças não muda o resultado.
	rev := slices.Clone(four)
	slices.Reverse(rev)
	if EquipBonus(12, rev) != EquipBonus(12, four) {
		t.Fatal("a ordem das peças mudou o bônus")
	}
	// A penalidade derruba a precisão e deixa ataques e recargas mais lentos.
	base := CharacterStats("knight", "", 12, Attrs{}, EquipBonus(13, worn("sword_d13")), nil, WeaponSword)
	pen := CharacterStats("knight", "", 12, Attrs{}, EquipBonus(12, worn("sword_d13")), nil, WeaponSword)
	if pen.Accuracy != base.Accuracy-100 || pen.AtkInterval <= base.AtkInterval*1.6 || pen.CastSpeed >= base.CastSpeed-0.39 {
		t.Fatalf("penalidade: %+v contra %+v", pen, base)
	}
}

// As comuns do grau caem dos monstros do grau (até o nível do monstro + 2), as épicas e
// lendárias só dos chefes.
func TestTierDrops(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 8))
	roll := func(id string, level, n int) map[*ItemDef]int {
		got := map[*ItemDef]int{}
		for range n {
			for _, d := range RollLoot(Monsters[id], level, rng) {
				if d.Item.Graded {
					got[d.Item]++
				}
			}
		}
		return got
	}
	if got := roll("skeleton_warrior", 9, 3000); len(got) != 0 {
		t.Fatalf("abaixo do 10 não cai grau: %v", got)
	}
	for _, c := range []struct {
		monster string
		level   int
		tier    Tier
	}{{"skeleton_warrior", 11, TierD}, {"skeleton_mage", 18, TierD}, {"stone_golem", 25, TierC}, {"ancient_golem", 32, TierB}, {"broken_reaper", 39, TierB}} {
		got := roll(c.monster, c.level, 5000)
		total := 0
		for d, n := range got {
			total += n
			if d.Tier != c.tier || d.Grade != GradeCommon || d.ReqLevel > c.level+tierDropAhead {
				t.Errorf("%s nv %d deixou cair %s (%s, nível %d)", c.monster, c.level, d.ID, d.Tier.Label(), d.ReqLevel)
			}
		}
		if rate := float64(total) / 5000; rate < TierDropChance*0.6 || rate > TierDropChance*1.4 {
			t.Errorf("%s nv %d: %.3f peças do grau por abate", c.monster, c.level, rate)
		}
	}
	tops := func(id string, level int) map[Tier]map[Grade]bool {
		out := map[Tier]map[Grade]bool{}
		for d := range roll(id, level, 600) {
			if out[d.Tier] == nil {
				out[d.Tier] = map[Grade]bool{}
			}
			out[d.Tier][d.Grade] = true
		}
		return out
	}
	if got := tops("bone_lord", 22); !got[TierC][GradeEpic] || !got[TierD][GradeEpic] || got[TierC][GradeCommon] {
		t.Errorf("Senhor dos Ossos: %v", got)
	}
	if got := tops("rare_captain", 16); !got[TierD][GradeEpic] {
		t.Errorf("Capitão Ossudo: %v", got)
	}
	if got := tops("ozrhal", 35); !got[TierB][GradeEpic] || !got[TierB][GradeLegendary] || !got[TierC][GradeEpic] {
		t.Errorf("Ozrhal: %v", got)
	}
	// Fora do sorteio por raridade.
	for _, pool := range gearByGrade {
		for _, d := range pool {
			if d.Graded {
				t.Fatalf("%s no sorteio por raridade", d.ID)
			}
		}
	}
}

// O Ferreiro faz as comuns do D, por mais moedas do que a loja paga por elas.
func TestTierDRecipes(t *testing.T) {
	for _, d := range TierPieces(TierD, GradeCommon) {
		r := Recipes["craft_"+d.ID]
		if r == nil || r.Result != d.ID {
			t.Errorf("%s sem receita", d.ID)
			continue
		}
		if r.Gold <= d.SellPrice() {
			t.Errorf("%s: a receita custa %d e a loja paga %d", r.ID, r.Gold, d.SellPrice())
		}
	}
}
