package game

import (
	"math"
	"sort"
)

// Cristalização e encantamento por grau (tunnel#154), como no Lineage 2.
//
//   - Cristalizar: uma peça de grau D ou maior vira cristais do próprio grau (CrystalYield). A
//     quantidade sai do preço da peça e sobe com o encantamento. Os cristais vendem por menos
//     que a peça (cristalizar não dá ouro): valem pelo que criam.
//   - Pergaminhos por grau: cada grau tem os seus (de arma, de armadura e os abençoados). Os
//     pergaminhos antigos, sem grau no nome, são os do D: os ids não mudaram, então quem já
//     tinha pergaminhos fica com os do D sem migração. O do D também encanta as peças sem grau.
//   - Receitas: as peças comuns do C e do B saem do Ferreiro com cristais do grau, materiais
//     da área e moedas; os pergaminhos abençoados do C e do B, também.
//   - Saída de ouro: encantar cobra moedas (EnchantFee) e as receitas cobram mais que a loja
//     paga pela peça.

// Ids dos cristais.
const (
	CrystalD = "crystal_d"
	CrystalC = "crystal_c"
	CrystalB = "crystal_b"
)

// crystalIDs: o cristal de cada grau ("" = o grau não cristaliza).
var crystalIDs = [...]string{TierNone: "", TierD: CrystalD, TierC: CrystalC, TierB: CrystalB}

// CrystalID: o cristal do grau ("" = sem grau, não cristaliza).
func CrystalID(t Tier) string {
	if int(t) >= len(crystalIDs) {
		return ""
	}
	return crystalIDs[t]
}

// CrystalShare: a fração do preço de loja da peça que volta em cristais (pelo preço de loja
// do cristal). A loja paga SellRate (40%) do preço da peça e também do cristal, então a
// venda dos cristais de uma peça +0 dá 25% do que a venda da própria peça daria, e a de um +10
// (o triplo de cristais), 75%: cristalizar nunca dá mais moedas que vender.
const CrystalShare = 0.25

// CrystalPerEnchant: cada nível de encantamento soma 1/5 dos cristais da peça +0.
const CrystalPerEnchant = 5

// Crystallizable: a peça vira cristais (equipamento de grau D ou maior).
func (d *ItemDef) Crystallizable() bool {
	return d != nil && d.Equippable() && d.Stack <= 1 && CrystalID(d.Tier) != "" && d.Price > 0
}

// CrystalBase: os cristais de uma peça +0 (0 = não cristaliza).
func CrystalBase(d *ItemDef) int {
	if !d.Crystallizable() {
		return 0
	}
	c := Items[CrystalID(d.Tier)]
	return max(1, int(math.Round(float64(d.Price)*CrystalShare/float64(c.Price))))
}

// CrystalYield: o cristal e quantos uma peça +enchant vira ("", 0 = não cristaliza).
func CrystalYield(d *ItemDef, enchant int) (string, int) {
	base := CrystalBase(d)
	if base == 0 {
		return "", 0
	}
	return CrystalID(d.Tier), base + base*max(0, enchant)/CrystalPerEnchant
}

// crystalItems: os cristais e os pergaminhos de encantar de cada grau (items.go junta ao
// catálogo).
func crystalItems() []*ItemDef {
	crystal := func(t Tier, id, name string, price int64, desc string) *ItemDef {
		d := stackable(id, name, KindMaterial, price, "crystal_"+toLower(t.Letter()), desc)
		d.CrystalOf = t
		return d
	}
	out := []*ItemDef{
		crystal(TierD, CrystalD, "Cristal D", 25,
			"O que sobra de uma peça de grau D depois de cristalizada. O Ferreiro usa para forjar."),
		crystal(TierC, CrystalC, "Cristal C", 60,
			"O que sobra de uma peça de grau C depois de cristalizada. Base das peças e dos pergaminhos abençoados do grau C."),
		crystal(TierB, CrystalB, "Cristal B", 150,
			"O que sobra de uma peça de grau B depois de cristalizada. Base das peças e dos pergaminhos abençoados do grau B."),
	}
	for _, s := range scrollSpecs {
		out = append(out, enchantScrollItems(s)...)
	}
	return out
}

// scrollSpec: os quatro pergaminhos de encantar de um grau.
type scrollSpec struct {
	tier                             Tier
	weapon, armor, weaponBl, armorBl string
	weaponPrice, armorPrice          int64
	weaponBlPrice, armorBlPrice      int64
}

// scrollSpecs: os pergaminhos de cada grau. Os do D mantêm os ids antigos (sem grau).
var scrollSpecs = []scrollSpec{
	{TierD, "scroll_ench_weapon", "scroll_ench_armor", "scroll_ench_weapon_b", "scroll_ench_armor_b", 1500, 600, 6000, 2500},
	{TierC, "scroll_ench_weapon_tc", "scroll_ench_armor_tc", "scroll_ench_weapon_tc_bl", "scroll_ench_armor_tc_bl", 3800, 1500, 15000, 6200},
	{TierB, "scroll_ench_weapon_tb", "scroll_ench_armor_tb", "scroll_ench_weapon_tb_bl", "scroll_ench_armor_tb_bl", 7500, 3000, 30000, 12500},
}

func enchantScrollItems(s scrollSpec) []*ItemDef {
	l := s.tier.Letter()
	what := "Grau " + l
	if s.tier == TierD {
		what = "Grau D e peças sem grau"
	}
	mk := func(d *ItemDef) *ItemDef { d.ScrollTier = s.tier; return d }
	return []*ItemDef{
		mk(scroll(s.weapon, "Pergaminho de Encantar Arma ("+l+")", ScrollEnchantWeapon, s.weaponPrice,
			"Sobe uma arma ("+what+") em +1. Até +3 é seguro; acima disso pode falhar, e a arma quebra. Encantar cobra moedas.")),
		mk(scroll(s.armor, "Pergaminho de Encantar Armadura ("+l+")", ScrollEnchantArmor, s.armorPrice,
			"Sobe armadura, elmo, escudo, roupa ou joia ("+what+") em +1. Até +3 é seguro; acima disso pode falhar, e o item quebra. Encantar cobra moedas.")),
		mk(blessed(scroll(s.weaponBl, "Pergaminho Abençoado de Arma ("+l+")", ScrollEnchantWeapon, s.weaponBlPrice,
			"Sobe uma arma ("+what+") em +1. Se falhar, a arma não quebra: volta a +0. Encantar cobra moedas."))),
		mk(blessed(scroll(s.armorBl, "Pergaminho Abençoado de Armadura ("+l+")", ScrollEnchantArmor, s.armorBlPrice,
			"Sobe a peça ("+what+") em +1. Se falhar, ela não quebra: volta a +0. Encantar cobra moedas."))),
	}
}

// EnchantScrollID: o pergaminho de encantar do grau (arma ou armadura, comum ou abençoado).
// Sem grau usa o do D.
func EnchantScrollID(t Tier, weapon, blessed bool) string {
	if t == TierNone {
		t = TierD
	}
	for _, s := range scrollSpecs {
		if s.tier != t {
			continue
		}
		switch {
		case weapon && blessed:
			return s.weaponBl
		case weapon:
			return s.weapon
		case blessed:
			return s.armorBl
		}
		return s.armor
	}
	return ""
}

// EnchantScrollShop: os pergaminhos comuns de cada grau que o Ferreiro vende (os abençoados
// só saem de receita, de chefe e de reputação).
func EnchantScrollShop() []string {
	var out []string
	for _, s := range scrollSpecs {
		out = append(out, s.weapon, s.armor)
	}
	return out
}

// crystalRecipes: as receitas que gastam cristais (recipes.go põe no fim da lista):
//   - as peças comuns do grau C e do B, com cristais, os materiais da área e moedas;
//   - os pergaminhos abençoados do C e do B.
//
// Uma peça pede 4× os cristais que ela mesma rende +0 (CrystalRecipeFactor; um +10 rende 3×): criar
// e cristalizar de novo sempre perde cristais. As moedas passam do que a loja paga pela peça
// (como as do D, em tierRecipes): criar e vender não dá lucro.
func crystalRecipes() []*Recipe {
	var out []*Recipe
	for _, t := range []Tier{TierC, TierB} {
		pieces := TierPieces(t, GradeCommon)
		sort.SliceStable(pieces, func(i, j int) bool { return pieces[i].ReqLevel < pieces[j].ReqLevel })
		for _, d := range pieces {
			big := d.Kind == KindWeapon || d.Kind == KindArmor
			mats := []QuestItem{mat(CrystalID(t), CrystalBase(d)*CrystalRecipeFactor)}
			switch {
			case t == TierC && big:
				mats = append(mats, mat("rune_core", 3), mat("grave_ash", 10))
			case t == TierC:
				mats = append(mats, mat("rune_core", 2), mat("grave_ash", 6))
			case big:
				mats = append(mats, mat("rune_core", 4), mat("shadow_essence", 4))
			default:
				mats = append(mats, mat("rune_core", 3), mat("shadow_essence", 3))
			}
			gold := int64(math.Ceil(float64(d.SellPrice())/100)*100) + 100
			out = append(out, recipe("craft_"+d.ID, d.ID, gold, mats...))
		}
	}
	out = append(out,
		recipe("craft_ench_weapon_tc_bl", "scroll_ench_weapon_tc_bl", 6000, mat(CrystalC, 60), mat("rune_core", 3), mat("profane_tome", 1)),
		recipe("craft_ench_armor_tc_bl", "scroll_ench_armor_tc_bl", 2500, mat(CrystalC, 30), mat("rune_core", 2), mat("grave_ash", 10)),
		recipe("craft_ench_weapon_tb_bl", "scroll_ench_weapon_tb_bl", 12000, mat(CrystalB, 60), mat("rune_core", 4), mat("shadow_essence", 6)),
		recipe("craft_ench_armor_tb_bl", "scroll_ench_armor_tb_bl", 5000, mat(CrystalB, 30), mat("rune_core", 3), mat("shadow_essence", 4)),
	)
	return out
}

// CrystalRecipeFactor: quantas vezes os cristais da peça +0 a receita dela pede.
const CrystalRecipeFactor = 4
