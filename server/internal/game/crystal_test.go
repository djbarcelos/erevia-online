package game

import (
	"slices"
	"testing"
)

// Os pergaminhos antigos (sem grau no nome) são os do D: os ids não mudaram, então o que os
// jogadores já tinham na bolsa, no armazém, no correio e no leilão virou o do D sem migração.
func TestLegacyEnchantScrollsAreGradeD(t *testing.T) {
	for _, id := range []string{"scroll_ench_weapon", "scroll_ench_armor", "scroll_ench_weapon_b", "scroll_ench_armor_b"} {
		d := Items[id]
		if d == nil || !d.IsEnchantScroll() || d.ScrollTier != TierD {
			t.Fatalf("%s: pergaminho do D (%+v)", id, d)
		}
	}
	for _, tc := range []struct {
		scroll, item string
		ok           bool
	}{
		{"scroll_ench_weapon", "sword_common", true}, // sem grau
		{"scroll_ench_weapon", "sword_d16", true},
		{"scroll_ench_weapon", "sword_c26", false},
		{"scroll_ench_weapon_tc", "sword_c26", true},
		{"scroll_ench_weapon_tc", "sword_d16", false},
		{"scroll_ench_weapon_tc", "sword_common", false},
		{"scroll_ench_armor_tb_bl", "ring_b", true},
		{"scroll_ench_weapon_tb", "ring_b", false}, // de arma numa joia
	} {
		if got := Items[tc.item].EnchantableBy(Items[tc.scroll]); got != tc.ok {
			t.Errorf("%s em %s: %v, quer %v", tc.scroll, tc.item, got, tc.ok)
		}
	}
}

func TestEveryTierHasItsScrolls(t *testing.T) {
	for _, tr := range Tiers {
		for _, weapon := range []bool{true, false} {
			for _, bl := range []bool{true, false} {
				id := EnchantScrollID(tr, weapon, bl)
				d := Items[id]
				want := tr
				if want == TierNone {
					want = TierD
				}
				if d == nil || d.ScrollTier != want || d.Blessed != bl || (d.Scroll == ScrollEnchantWeapon) != weapon {
					t.Errorf("%s (arma %v, abençoado %v): %s %+v", tr.Label(), weapon, bl, id, d)
				}
				if !bl && !slices.Contains(SmithStock, id) {
					t.Errorf("o Ferreiro vende %s", id)
				}
			}
		}
	}
	for _, id := range []string{"scroll_ench_weapon_tc_bl", "scroll_ench_armor_tc_bl", "scroll_ench_weapon_tb_bl", "scroll_ench_armor_tb_bl"} {
		found := false
		for _, r := range RecipeList {
			found = found || r.Result == id
		}
		if !found {
			t.Errorf("%s sai de receita", id)
		}
	}
}

// Cristalizar nunca dá mais moedas que vender a peça, em qualquer encantamento.
func TestCrystallizeNeverBeatsSelling(t *testing.T) {
	n := 0
	for _, id := range ItemIDs() {
		d := Items[id]
		if !d.Crystallizable() {
			if d.Equippable() && d.Tier >= TierD && d.Price > 0 {
				t.Errorf("%s (%s) deveria cristalizar", id, d.Tier.Label())
			}
			continue
		}
		n++
		for e := 0; e <= MaxEnchant; e++ {
			c, k := CrystalYield(d, e)
			if Items[c] == nil || Items[c].CrystalOf != d.Tier {
				t.Fatalf("%s: cristal %q do grau errado", id, c)
			}
			if e > 0 {
				if _, prev := CrystalYield(d, e-1); k < prev {
					t.Errorf("%s +%d rende menos que +%d", id, e, e-1)
				}
			}
			if got := Items[c].SellPrice() * int64(k); got >= d.SellPrice() {
				t.Errorf("%s +%d: %d× %s vendem %d, a peça %d", id, e, k, c, got, d.SellPrice())
			}
		}
	}
	if n < 100 {
		t.Fatalf("só %d peças cristalizam", n)
	}
	for _, id := range []string{"sword_common", "potion_hp_s", "scroll_ench_weapon", CrystalD} {
		if _, k := CrystalYield(Items[id], 0); k != 0 {
			t.Errorf("%s não cristaliza", id)
		}
	}
}

// Criar e cristalizar de novo perde cristais e moedas (nem cristal nem ouro do nada), e as
// peças comuns do C e do B saem do Ferreiro.
func TestCrystalRecipesNeverGainCrystals(t *testing.T) {
	made := map[string]bool{}
	for _, r := range RecipeList {
		res := Items[r.Result]
		made[r.Result] = true
		usesCrystal := slices.ContainsFunc(r.Materials, func(m QuestItem) bool { return Items[m.ID].CrystalOf != TierNone })
		if usesCrystal && r.Gold < res.SellPrice()*int64(r.Count) {
			t.Errorf("%s: %d moedas, menos que a venda (%d)", r.ID, r.Gold, res.SellPrice()*int64(r.Count))
		}
		c, back := CrystalYield(res, MaxEnchant)
		for _, m := range r.Materials {
			if m.ID == c && m.Count*r.Count <= back {
				// até um +10 do resultado rende menos que a receita gasta
				t.Errorf("%s: gasta %d× %s e o resultado +%d rende %d", r.ID, m.Count, c, MaxEnchant, back)
			}
		}
	}
	for _, tr := range []Tier{TierC, TierB} {
		for _, d := range TierPieces(tr, GradeCommon) {
			if !made[d.ID] {
				t.Errorf("%s (%s) sai do Ferreiro", d.ID, tr.Label())
			}
		}
	}
}

func TestEnchantChanceAndFeeByTier(t *testing.T) {
	for n := 0; n <= MaxEnchant; n++ {
		d, c, b := EnchantChanceFor(TierD, n), EnchantChanceFor(TierC, n), EnchantChanceFor(TierB, n)
		if EnchantChanceFor(TierNone, n) != d || EnchantChance(n) != d {
			t.Errorf("+%d: sem grau e D têm a mesma chance", n)
		}
		switch {
		case n <= EnchantSafe:
			if d != 1 || c != 1 || b != 1 {
				t.Errorf("+%d é seguro em todo grau", n)
			}
		case !(d > c && c > b && b > 0):
			t.Errorf("+%d: D %.2f > C %.2f > B %.2f > 0", n, d, c, b)
		}
	}
	if EnchantChanceFor(TierB, MaxEnchant+1) != 0 {
		t.Error("acima do máximo não há chance")
	}
	for i := 1; i < len(Tiers); i++ {
		if EnchantFeePerLevel(Tiers[i]) <= EnchantFeePerLevel(Tiers[i-1]) {
			t.Errorf("a taxa sobe com o grau: %s", Tiers[i].Label())
		}
	}
	if EnchantFee(TierC, 4) != 4*EnchantFeePerLevel(TierC) {
		t.Error("a taxa é por nível de destino")
	}
}

// Nas peças dos graus, o encantamento é uma fração da própria peça.
func TestGradedEnchantScalesWithThePiece(t *testing.T) {
	lo, hi := Items["sword_d16"], Items["sword_c26"]
	b1, b2 := EnchantBonus(lo, 7).PAtk, EnchantBonus(hi, 7).PAtk
	if b1 <= 0 || b2 <= b1 {
		t.Fatalf("+7: D %.0f, C %.0f (o C ganha mais)", b1, b2)
	}
	want := lo.Bonus.PAtk * GradedEnchantStep * 11 // 7 níveis, 4 acima do seguro
	if b1 < want-0.5 || b1 > want+0.5 {
		t.Fatalf("+7 da D: %.1f, quer %.1f", b1, want)
	}
	if r := EnchantBonus(Items["ring_d"], 5); r.MDef <= 0 || r.PDef != 0 || r.PAtk != 0 {
		t.Fatalf("joia: só M.Def (%+v)", r)
	}
	// As sem grau próprio continuam no bônus fixo da raridade.
	if EnchantBonus(Items["sword_good"], 3).PAtk != EnchantPerGrade(GradeGood)*3 {
		t.Fatal("a Espada de Aço segue o bônus da raridade")
	}
}
