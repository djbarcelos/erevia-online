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

// EnchantChance é a chance de o item chegar a +target.
func EnchantChance(target int) float64 {
	switch {
	case target <= EnchantSafe:
		return 1
	case target > MaxEnchant:
		return 0
	}
	return 0.70 - 0.05*float64(target-EnchantSafe-1)
}

// EnchantChances: chance[n] = de chegar a +n (para o cliente).
func EnchantChances() []float64 {
	out := make([]float64, MaxEnchant+1)
	for i := range out {
		out[i] = EnchantChance(i)
	}
	return out
}

// enchantSteps: cada nível acima do seguro vale o dobro.
func enchantSteps(level int) float64 {
	return float64(level + max(0, level-EnchantSafe))
}

// Enchantable informa se o item aceita este tipo de pergaminho.
func (d *ItemDef) Enchantable(scroll ScrollKind) bool {
	if d == nil || !d.Equippable() || d.Stack > 1 {
		return false
	}
	if d.Kind == KindWeapon {
		return scroll == ScrollEnchantWeapon
	}
	return scroll == ScrollEnchantArmor
}

// EnchantBonus é o que o encantamento soma ao item: armas ganham o ataque que já dão
// (físico e/ou mágico); as outras peças ganham defesa (joias, só a mágica).
func EnchantBonus(d *ItemDef, level int) Stats {
	var b Stats
	if d == nil || level <= 0 || int(d.Grade) >= len(enchantPerGrade) {
		return b
	}
	per := enchantPerGrade[d.Grade] * enchantSteps(level)
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
