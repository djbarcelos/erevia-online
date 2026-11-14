package zone

import (
	"math"
	"testing"

	"erevia/server/internal/game"
)

// As contas de atributos, custo e recarga de skill saíram da zona para o pacote game
// (game.CharacterStats, game.AddItemBonus, game.SkillMPCost, game.SkillCooldownFor) para o
// simulador de balanceamento usar os mesmos números. Este teste guarda a versão antiga, tal
// como estava na zona, e confere que a nova dá exatamente o mesmo número.

func oldItemsBonus(inv *inventory) game.Stats {
	var b game.Stats
	for _, s := range inv.equip {
		if !s.empty() {
			b = b.AddBonus(s.def.Bonus)
			if s.enchant > 0 {
				b = b.AddBonus(game.EnchantBonus(s.def, s.enchant))
			}
		}
	}
	return b
}

func oldComputeStats(p *entity, spent game.Attrs) game.Stats {
	cls := p.pl.class
	total := game.ClassBaseAttrs[cls].Add(spent).Add(game.SpecBonus(cls, p.pl.spec))
	s := game.ClassStats(cls, p.level)
	s = game.ApplyAttrs(s, cls, p.level, total)
	s = s.AddBonus(oldItemsBonus(&p.pl.inv))
	s = oldApplyPassives(s, p)
	s = game.FinishRegen(s, cls, total)
	if r := p.pl.rank(game.SkillClearMind); r > 0 {
		s.MPRegen *= 1 + game.Skills[game.SkillClearMind].PValue.At(r)
	}
	return s
}

func oldApplyPassives(s game.Stats, p *entity) game.Stats {
	for id, r := range p.pl.ranks {
		sk := game.Skills[id]
		if sk == nil || r < 1 || !sk.IsPassive() {
			continue
		}
		v := sk.PValue.At(r)
		switch sk.Passive {
		case game.PassivePDefPct:
			s.PDef *= 1 + v
		case game.PassiveSwordPAtk:
			if p.pl.inv.weaponType() == game.WeaponSword {
				s.PAtk *= 1 + v
			}
		case game.PassiveCritDamage:
			s.CritDamage += v
		case game.PassiveMaxHPPct:
			s.MaxHP *= 1 + v
		case game.PassiveOverload:
			s.MAtk *= 1 + v
		case game.PassiveCrit:
			s.Crit += v
		case game.PassiveEvasion:
			s.Evasion += v
		}
	}
	return s
}

func oldSkillMP(p *entity, s *game.SkillDef) float64 {
	cost := s.MPCost(p.pl.rank(s.ID))
	if p.pl.rank(game.SkillOverload) > 0 {
		cost *= game.OverloadMPCost
	}
	return math.Round(cost)
}

func oldSkillCD(p *entity, s *game.SkillDef) float64 {
	cs := p.stats.CastSpeed
	if cs <= 0 {
		cs = 1
	}
	return s.Cooldown(p.pl.rank(s.ID)) / cs
}

func TestExtractedStatsMatchZone(t *testing.T) {
	z := newCombatZone()
	gear := map[string][]string{
		"knight":    {"sword_epic", "shield_rare"},
		"barbarian": {"axe_legend", "shield_good"},
		"mage":      {"staff_rare"},
		"rogue":     {"daggers_good"},
	}
	common := []string{"armor_epic", "helm_good", "legs_rare", "gloves_epic", "boots_rare", "cape_good",
		"neck_legend", "ear_rare", "ear_good", "ring_epic", "ring_legend"}
	id := uint32(1)
	checked := 0
	for class := range game.Classes {
		for _, spec := range append([]string{""}, specIDs(class)...) {
			for _, level := range []int{1, 7, 20, 30} {
				for _, enchant := range []int{0, 3, 10} {
					p, _ := joinAt(z, id, class, 30, 0, 0)
					id++
					p.pl.spec = spec
					atLevel(z, p, level)
					// Todas as skills da classe e do caminho no rank máximo (todas as passivas).
					for _, s := range game.Skills {
						if s.Class == class && (s.Spec == "" || s.Spec == spec) {
							p.pl.ranks[s.ID] = s.MaxRank
						}
					}
					for _, it := range append(append([]string{}, gear[class]...), common...) {
						d := game.Items[it]
						slot := d.Slot()
						if slot == game.SlotEar1 && !p.pl.inv.equip[slot].empty() {
							slot = game.SlotEar2
						}
						if slot == game.SlotRing1 && !p.pl.inv.equip[slot].empty() {
							slot = game.SlotRing2
						}
						p.pl.inv.equip[slot] = stack{def: d, count: 1, enchant: enchant}
					}
					spent := game.Attrs{Str: level, Dex: level / 2, Con: 2, Int: level, Wis: 3, Spi: 1}
					if got, want := p.pl.inv.bonus(), oldItemsBonus(&p.pl.inv); got != want {
						t.Fatalf("%s +%d: bônus do equipamento %+v, antes %+v", class, enchant, got, want)
					}
					if got, want := z.computeStats(p, spent), oldComputeStats(p, spent); got != want {
						t.Fatalf("%s/%s nv %d +%d: atributos\n%+v\nantes\n%+v", class, spec, level, enchant, got, want)
					}
					z.refreshStats(p)
					for _, s := range game.Skills {
						if s.Class != class || s.IsPassive() {
							continue
						}
						if got, want := skillMP(p, s), oldSkillMP(p, s); got != want {
							t.Fatalf("%s: mana %v, antes %v", s.Name, got, want)
						}
						if got, want := skillCD(p, s), oldSkillCD(p, s); got != want {
							t.Fatalf("%s: recarga %v, antes %v", s.Name, got, want)
						}
					}
					checked++
				}
			}
		}
	}
	// Sobrecarga e velocidade de conjuração mexem no custo e na recarga.
	p, _ := joinAt(z, id, "mage", 30, 0, 0)
	atLevel(z, p, 20)
	learn(z, p, map[uint32]int{game.SkillOverload: 1, game.SkillFireball: 3})
	p.stats.CastSpeed = 1.37
	s := game.Skills[game.SkillFireball]
	if skillMP(p, s) != oldSkillMP(p, s) || skillCD(p, s) != oldSkillCD(p, s) || skillMP(p, s) == s.MPCost(3) {
		t.Errorf("mago com Sobrecarga: mana %v/%v, recarga %v/%v", skillMP(p, s), oldSkillMP(p, s), skillCD(p, s), oldSkillCD(p, s))
	}
	p.stats.CastSpeed = 0
	if skillCD(p, s) != oldSkillCD(p, s) {
		t.Error("velocidade de conjuração zero deveria valer 1")
	}
	if checked < 100 {
		t.Fatalf("só %d combinações conferidas", checked)
	}
	// As constantes de regeneração continuam as mesmas.
	if outOfCombatAfter != 5.0 || calmRegenMult != 5.0 || calmMPRegenMult != 3.3 {
		t.Errorf("regeneração fora de combate mudou: %v %v %v", outOfCombatAfter, calmRegenMult, calmMPRegenMult)
	}
}

func specIDs(class string) []string {
	var out []string
	for _, s := range game.ClassSpecs(class) {
		out = append(out, s.ID)
	}
	return out
}
