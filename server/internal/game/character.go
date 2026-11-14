package game

import "math"

// Atributos de um personagem montado: as contas puras que a zona faz ao recalcular os
// atributos, cobrar uma skill e regenerar. Ficam aqui para o simulador de balanceamento
// (cmd/balance) usar exatamente os mesmos números, sem servidor.

// Regeneração fora de combate.
const (
	OutOfCombatAfter = 5.0 // segundos sem lutar para regenerar rápido
	CalmHPRegenMult  = 5.0 // fora de combate, a vida regenera 5× mais rápido
	CalmMPRegenMult  = 3.3 // e a mana, 3,3×
)

// CharacterStats: classe no nível → atributos (base da classe + pontos + bônus da
// especialização) → equipamento (gear: a soma dos bônus, ver AddItemBonus) → passivas →
// regeneração (com a Mente Clara por cima). weaponType é o tipo da arma equipada ("" =
// nenhuma; a Esgrima só vale com espada).
func CharacterStats(class, spec string, level int, spent Attrs, gear Stats, ranks map[uint32]int, weaponType string) Stats {
	total := ClassBaseAttrs[class].Add(spent).Add(SpecBonus(class, spec))
	s := ClassStats(class, level)
	s = ApplyAttrs(s, class, level, total)
	s = s.AddBonus(gear)
	s = ApplyPassives(s, ranks, weaponType)
	s = FinishRegen(s, class, total)
	if r := ranks[SkillClearMind]; r > 0 {
		s.MPRegen *= 1 + Skills[SkillClearMind].PValue.At(r)
	}
	return s
}

// ApplyPassives aplica as passivas de atributo aprendidas (ranks: skill → rank).
func ApplyPassives(s Stats, ranks map[uint32]int, weaponType string) Stats {
	for id, r := range ranks {
		sk := Skills[id]
		if sk == nil || r < 1 || !sk.IsPassive() {
			continue
		}
		v := sk.PValue.At(r)
		switch sk.Passive {
		case PassivePDefPct:
			s.PDef *= 1 + v
		case PassiveSwordPAtk:
			if weaponType == WeaponSword {
				s.PAtk *= 1 + v
			}
		case PassiveCritDamage:
			s.CritDamage += v
		case PassiveMaxHPPct:
			s.MaxHP *= 1 + v
		case PassiveOverload:
			s.MAtk *= 1 + v
		case PassiveCrit:
			s.Crit += v
		case PassiveEvasion:
			s.Evasion += v
		}
	}
	return s
}

// AddItemBonus soma a b o que um item equipado dá: o bônus do item e o do encantamento.
func AddItemBonus(b Stats, d *ItemDef, enchant int) Stats {
	b = b.AddBonus(d.Bonus)
	if enchant > 0 {
		b = b.AddBonus(EnchantBonus(d, enchant))
	}
	return b
}

// SkillMPCost: custo de mana no rank, arredondado (a Sobrecarga encarece 10%).
func SkillMPCost(s *SkillDef, rank int, overload bool) float64 {
	cost := s.MPCost(rank)
	if overload {
		cost *= OverloadMPCost
	}
	return math.Round(cost)
}

// SkillCooldownFor: recarga no rank, dividida pela velocidade de conjuração (≤ 0 vale 1).
func SkillCooldownFor(s *SkillDef, rank int, castSpeed float64) float64 {
	if castSpeed <= 0 {
		castSpeed = 1
	}
	return s.Cooldown(rank) / castSpeed
}
