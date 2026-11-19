package game

import (
	"math"
	"strconv"
)

// Encantamento: pergaminhos sobem o item de +0 até +MaxEnchant. Até EnchantSafe não há
// risco; acima, uma falha quebra o item (o pergaminho abençoado só volta a +0).
const (
	EnchantSafe = 3
	MaxEnchant  = 10
)

// ScrollKind é o tipo de pergaminho (igual ao enum do protocolo).
type ScrollKind uint8

const (
	ScrollNone ScrollKind = iota
	ScrollReturn
	ScrollEnchantWeapon
	ScrollEnchantArmor
)

// enchantPerGrade: bônus por nível, por raridade (índice = Grade). Armas somam ataque;
// as outras peças somam metade disso em defesa.
var enchantPerGrade = [...]float64{0, 1, 2, 3, 4, 5}

// EnchantPerGrade: bônus de ataque por nível de encantamento da raridade.
func EnchantPerGrade(g Grade) float64 {
	if int(g) >= len(enchantPerGrade) {
		return 0
	}
	return enchantPerGrade[g]
}

// EnchantChance é a chance de o item sem grau ou do grau D chegar a +target.
func EnchantChance(target int) float64 { return EnchantChanceFor(TierD, target) }

// enchantRiskByTier: quanto o grau tira da chance de cada nível acima do seguro (tunnel#154).
// Até o seguro nada muda; o C fica 5 pontos abaixo do D e o B, 10.
var enchantRiskByTier = [...]float64{TierNone: 0, TierD: 0, TierC: 0.05, TierB: 0.10}

// EnchantChanceFor é a chance de uma peça do grau t chegar a +target.
func EnchantChanceFor(t Tier, target int) float64 {
	switch {
	case target <= EnchantSafe:
		return 1
	case target > MaxEnchant:
		return 0
	}
	risk := 0.0
	if int(t) < len(enchantRiskByTier) {
		risk = enchantRiskByTier[t]
	}
	return math.Round((0.70-0.05*float64(target-EnchantSafe-1)-risk)*100) / 100
}

// EnchantChances: chance[n] = de chegar a +n no grau D (para o cliente).
func EnchantChances() []float64 { return EnchantChancesFor(TierD) }

// EnchantChancesFor: chance[n] = de chegar a +n no grau t.
func EnchantChancesFor(t Tier) []float64 {
	out := make([]float64, MaxEnchant+1)
	for i := range out {
		out[i] = EnchantChanceFor(t, i)
	}
	return out
}

// enchantFeeByTier: moedas por nível de destino que o encantamento cobra, por grau (a
// saída de ouro da tunnel#154). Encantar a +n custa n × a taxa do grau, dê certo ou não.
// Os valores saem do modelo de ouro (internal/balance/gold.go, go run ./cmd/balance -ouro):
// em cada grau, o que sai fica entre 60% e 100% do que entra (TestGoldSinkPerTier). O salto
// do B acompanha o saque do 30 ao 40, que vende muito (o equipamento ponte da tunnel#149).
var enchantFeeByTier = [...]int64{TierNone: 10, TierD: 40, TierC: 650, TierB: 6500}

// EnchantFeePerLevel: as moedas por nível de destino no grau t.
func EnchantFeePerLevel(t Tier) int64 {
	if int(t) >= len(enchantFeeByTier) {
		return 0
	}
	return enchantFeeByTier[t]
}

// EnchantFee: o que custa tentar levar uma peça do grau t a +target.
func EnchantFee(t Tier, target int) int64 { return EnchantFeePerLevel(t) * int64(max(0, target)) }

// enchantSteps: cada nível acima do seguro vale o dobro.
func enchantSteps(level int) float64 {
	return float64(level + max(0, level-EnchantSafe))
}

// IsEnchantScroll: o item é um pergaminho de encantar (de arma ou de armadura).
func (d *ItemDef) IsEnchantScroll() bool {
	return d != nil && d.Kind == KindScroll && (d.Scroll == ScrollEnchantWeapon || d.Scroll == ScrollEnchantArmor)
}

// ScrollFits: o pergaminho é do grau da peça. O do grau D também serve nas peças sem grau.
func ScrollFits(scrollTier, itemTier Tier) bool {
	return scrollTier == itemTier || (scrollTier == TierD && itemTier == TierNone)
}

// Enchantable informa se o item aceita este tipo de pergaminho (sem olhar o grau: ver
// EnchantableBy).
func (d *ItemDef) Enchantable(scroll ScrollKind) bool {
	if d == nil || !d.Equippable() || d.Stack > 1 {
		return false
	}
	if d.Kind == KindWeapon {
		return scroll == ScrollEnchantWeapon
	}
	return scroll == ScrollEnchantArmor
}

// EnchantableBy informa se o pergaminho sc encanta o item: o tipo (arma ou armadura) e o
// grau (ScrollFits) precisam bater.
func (d *ItemDef) EnchantableBy(sc *ItemDef) bool {
	return sc.IsEnchantScroll() && d.Enchantable(sc.Scroll) && ScrollFits(sc.ScrollTier, d.Tier)
}

// GradedEnchantStep: nas peças feitas para os graus (tiers.go), cada nível de encantamento
// soma esta fração do ataque (armas) ou da defesa (as outras peças) que a peça já dá. Assim o
// encantamento cresce com a peça, e um +7 (11 degraus: acima do seguro cada nível vale o
// dobro) fica abaixo da mesma raridade +0 do grau seguinte (tunnel#154; o teste está em
// internal/balance).
const GradedEnchantStep = 0.028

// EnchantStep é o que um degrau de encantamento soma à peça (sem arredondar). O total de um
// +n é EnchantStep × degraus (n, mais 1 por nível acima do seguro), arredondado por atributo.
func EnchantStep(d *ItemDef) Stats {
	var b Stats
	if d == nil {
		return b
	}
	if d.Graded {
		switch d.Kind {
		case KindWeapon:
			b.PAtk = d.Bonus.PAtk * GradedEnchantStep
			b.MAtk = d.Bonus.MAtk * GradedEnchantStep
		case KindNecklace, KindEarring, KindRing:
			b.MDef = d.Bonus.MDef * GradedEnchantStep
		default:
			b.PDef = d.Bonus.PDef * GradedEnchantStep
			b.MDef = d.Bonus.MDef * GradedEnchantStep
		}
		return b
	}
	if int(d.Grade) >= len(enchantPerGrade) {
		return b
	}
	per := enchantPerGrade[d.Grade]
	switch d.Kind {
	case KindWeapon:
		if d.Bonus.MAtk > 0 {
			b.MAtk = per
		}
		if d.Bonus.PAtk > 0 || d.Bonus.MAtk <= 0 {
			b.PAtk = per
		}
	case KindNecklace, KindEarring, KindRing:
		b.MDef = per / 2
	default:
		b.PDef = per / 2
		b.MDef = per / 2
	}
	return b
}

// EnchantBonus é o que o encantamento soma ao item: armas ganham o ataque que já dão
// (físico e/ou mágico); as outras peças ganham defesa (joias, só a mágica). As peças sem
// grau próprio usam o bônus fixo da raridade; as dos graus, a fração da própria peça
// (EnchantStep).
func EnchantBonus(d *ItemDef, level int) Stats {
	var b Stats
	if d == nil || level <= 0 {
		return b
	}
	step, n := EnchantStep(d), enchantSteps(level)
	if d.Graded {
		b.PAtk, b.MAtk = math.Round(step.PAtk*n), math.Round(step.MAtk*n)
		b.PDef, b.MDef = math.Round(step.PDef*n), math.Round(step.MDef*n)
		return b
	}
	if int(d.Grade) >= len(enchantPerGrade) {
		return b
	}
	per := enchantPerGrade[d.Grade] * n
	switch d.Kind {
	case KindWeapon:
		if d.Bonus.MAtk > 0 {
			b.MAtk = per
		}
		if d.Bonus.PAtk > 0 || d.Bonus.MAtk <= 0 {
			b.PAtk = per
		}
	case KindNecklace, KindEarring, KindRing:
		b.MDef = math.Round(per / 2)
	default:
		b.PDef = math.Round(per / 2)
		b.MDef = math.Round(per / 2)
	}
	return b
}

// EnchantedName: "+5 Espada de Aço" (sem encantamento, só o nome).
func EnchantedName(d *ItemDef, level int) string {
	if level <= 0 {
		return d.Name
	}
	return "+" + strconv.Itoa(level) + " " + d.Name
}
