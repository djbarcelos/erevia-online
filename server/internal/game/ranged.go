package game

import (
	"math"
	"strings"
)

// Armas de longe do Patrulheiro (docs/evolucoes/hunter.md, Fase 3): arco, besta e besta
// pesada. Cada arma do Ladino a partir do nível 16 ganha as três versões, com o mesmo grau,
// nível e bônus; o ataque da arma muda pelo tipo (o doc: arco igual às adagas, besta +15%,
// besta pesada +45%). Alcance e intervalo vêm de Weapons (ApplyWeapon).

// rangedKind: um tipo de arma de longe e como a versão dele é feita.
type rangedKind struct {
	weapon, prefix, noun, model string
	feminine                    bool
	atk                         float64
}

var rangedKinds = []rangedKind{
	{WeaponBow, "bow", "Arco", "bow_withString", false, 1},
	{WeaponCrossbow, "crossbow", "Besta", "crossbow_1handed", true, 1.15},
	{WeaponCrossbow2H, "crossbow2h", "Besta Pesada", "crossbow_2handed", true, 1.45},
}

// RangedFromLevel: só as armas do Ladino deste nível em diante ganham versões de longe (o
// Patrulheiro evolui no 20).
const RangedFromLevel = 16

// HunterStarterBow: o arco que o Patrulheiro ganha ao evoluir (uma vez).
const HunterStarterBow = "bow_hunter"

// rangedName: "Adagas de Sílex" → "Arco de Sílex"; "Adagas Rúnicas" → "Besta Rúnica".
func rangedName(base string, k rangedKind) string {
	rest := base
	if i := strings.IndexByte(base, ' '); i > 0 {
		rest = base[i:]
	}
	fix := map[string][2]string{"Rúnicas": {"Rúnico", "Rúnica"}, "Despertas": {"Desperto", "Desperta"}}
	words := strings.Fields(rest)
	for i, w := range words {
		if f, ok := fix[w]; ok {
			words[i] = f[0]
			if k.feminine {
				words[i] = f[1]
			}
		}
	}
	return strings.TrimSpace(k.noun + " " + strings.Join(words, " "))
}

// rangedItems: as versões de longe das armas do Ladino (de list) e o arco inicial.
func rangedItems(list []*ItemDef) []*ItemDef {
	var out []*ItemDef
	for _, d := range list {
		if d.WeaponType != WeaponDaggers || d.ReqLevel < RangedFromLevel {
			continue
		}
		suffix := strings.TrimPrefix(strings.TrimPrefix(d.ID, "daggers_"), "daggers")
		for _, k := range rangedKinds {
			c := *d
			c.ID = k.prefix + "_" + suffix
			c.Name = rangedName(d.Name, k)
			c.WeaponType, c.Model, c.OffModel = k.weapon, k.model, ""
			c.Bonus.PAtk = math.Round(d.Bonus.PAtk * k.atk)
			c.Classes = []string{"rogue"}
			out = append(out, &c)
		}
	}
	starter := weapon(WeaponBow, HunterStarterBow, "Arco do Patrulheiro", "rogue", GradeCommon, 20, Stats{PAtk: 23}, 2000,
		"bow_withString", "", "O arco que a Ysolde entrega a quem escolhe a caçada.")
	starter.Graded = true // na escada do grau C, com as armas do nível 20
	return append(out, starter)
}

// RangedAtkMult: quanto o tipo de arma de longe multiplica o ataque da arma (1 nos outros).
// O tiro mais lento bate mais forte: o orçamento de poder divide por isso.
func RangedAtkMult(weaponType string) float64 {
	for _, k := range rangedKinds {
		if k.weapon == weaponType {
			return k.atk
		}
	}
	return 1
}
