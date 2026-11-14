package game

import (
	"fmt"
	"math"
)

// Equipamento ponte do 30 ao 40 (tunnel#149), até os graus de equipamento (tunnel#153):
// para cada classe, uma arma e uma armadura de nível 30 (Rúnicas, das ruínas dos Ilvarin)
// e de nível 35 (da Partição), em duas raridades. Usa os modelos de hoje.
//
// Não entram no sorteio de equipamento por raridade (gearByGrade): só caem das elites do 30
// ao 40 e do Ozrhal, pela lista de saque deles (bridgeLoot). O salto de poder é suave: o
// conjunto de 35 tem BridgeStep a mais que o de 30, e o raro, BridgeRareStep a mais que o
// comum do mesmo nível. O comum de 30 fica perto do lendário do raid (nível 20).

const (
	BridgeStep     = 1.12 // 35 sobre 30
	BridgeRareStep = 1.15 // raro sobre comum
)

// BridgeLevels: os níveis dos conjuntos.
var BridgeLevels = [2]int{30, 35}

// bridgeClass: a peça de cada classe no nível 30, comum. O resto sai desta.
type bridgeClass struct {
	class         string
	weapon, armor string // nomes no nível 30
	weapon35      string // e no 35
	armor35       string
	model, off    string
	weaponStats   Stats
	armorStats    Stats
	weaponDesc    string
	armorDesc     string
	weaponDesc35  string
	armorDesc35   string
	weaponPrefix  string
	// Raro: "Desperta", "Desperto" ou "Despertas", na concordância de cada peça.
	weaponRare, armorRare string
}

var bridgeClasses = []bridgeClass{
	{class: "knight", weaponPrefix: "sword", weapon: "Espada Rúnica", armor: "Couraça Rúnica",
		weapon35: "Espada da Partição", armor35: "Couraça da Partição", model: "sword_2handed", weaponRare: "Desperta", armorRare: "Desperta",
		weaponStats: Stats{PAtk: 46, MaxHP: 60, Crit: 0.02}, armorStats: Stats{PDef: 40, MDef: 22, MaxHP: 150},
		weaponDesc: "Runas dos Ilvarin correm pela lâmina como água.", armorDesc: "Placas de pedra rúnica, leves como aço.",
		weaponDesc35: "Forjada com uma lasca da rachadura da Partição.", armorDesc35: "Estala como pedra quando aguenta um golpe."},
	{class: "barbarian", weaponPrefix: "axe", weapon: "Machado Rúnico", armor: "Peitoral Rúnico",
		weapon35: "Machado da Partição", armor35: "Peitoral da Partição", model: "axe_2handed", weaponRare: "Desperto", armorRare: "Desperto",
		weaponStats: Stats{PAtk: 51, Crit: 0.04}, armorStats: Stats{PDef: 34, MDef: 20, MaxHP: 170},
		weaponDesc: "O fio brilha verde a cada pancada.", armorDesc: "Couro grosso costurado com fios de runa.",
		weaponDesc35: "Parte a pedra pela mesma linha em que a Partição partiu.", armorDesc35: "Tem cicatrizes de golpes que ninguém sobreviveu para dar de novo."},
	{class: "mage", weaponPrefix: "staff", weapon: "Cajado Rúnico", armor: "Túnica Rúnica",
		weapon35: "Cajado da Partição", armor35: "Túnica da Partição", model: "staff", weaponRare: "Desperto", armorRare: "Desperta",
		weaponStats: Stats{MAtk: 53, MaxMP: 90, Crit: 0.02}, armorStats: Stats{PDef: 22, MDef: 38, MaxHP: 90, MaxMP: 90},
		weaponDesc: "Uma pedra de golem no topo, ainda morna.", armorDesc: "Tecida pelos Ilvarin para os seus tecelões de runas.",
		weaponDesc35: "Zumbe no tom da rachadura que corta Erevia.", armorDesc35: "O tecido muda de cor quando a magia passa por ele."},
	{class: "rogue", weaponPrefix: "daggers", weapon: "Adagas Rúnicas", armor: "Gibão Rúnico",
		weapon35: "Adagas da Partição", armor35: "Gibão da Partição", model: "dagger", off: "dagger", weaponRare: "Despertas", armorRare: "Desperto",
		weaponStats: Stats{PAtk: 41, Crit: 0.05}, armorStats: Stats{PDef: 30, MDef: 24, MaxHP: 120, Evasion: 5},
		weaponDesc: "Duas lascas de runa, afiadas até sumirem de lado.", armorDesc: "Couro fino com runas que abafam os passos.",
		weaponDesc35: "Cortam por onde a terra já estava rachada.", armorDesc35: "Escuro como o fundo da Partição."},
}

// BridgeWeaponID e BridgeArmorID: os ids das peças (sword_b30, armor_knight_b35_rare…).
func BridgeWeaponID(class string, level int, g Grade) string {
	for _, c := range bridgeClasses {
		if c.class == class {
			return fmt.Sprintf("%s_b%d%s", c.weaponPrefix, level, bridgeSuffix(g))
		}
	}
	return ""
}

func BridgeArmorID(class string, level int, g Grade) string {
	return fmt.Sprintf("armor_%s_b%d%s", class, level, bridgeSuffix(g))
}

func bridgeSuffix(g Grade) string {
	if g == GradeRare {
		return "_rare"
	}
	return ""
}

// scaleBonus multiplica um bônus: atributos planos em inteiros, crítico em 0,005.
func scaleBonus(s Stats, f float64) Stats {
	r := func(v float64) float64 { return math.Round(v * f) }
	s.MaxHP, s.MaxMP, s.PAtk, s.MAtk, s.PDef, s.MDef = r(s.MaxHP), r(s.MaxMP), r(s.PAtk), r(s.MAtk), r(s.PDef), r(s.MDef)
	s.Evasion = r(s.Evasion)
	s.Crit = math.Round(s.Crit*f*200) / 200
	return s
}

// bridgeItems: as 32 peças (4 classes × arma e armadura × 30 e 35 × comum e raro).
func bridgeItems() []*ItemDef {
	var out []*ItemDef
	for _, c := range bridgeClasses {
		for i, lvl := range BridgeLevels {
			step := math.Pow(BridgeStep, float64(i))
			wName, aName, wDesc, aDesc := c.weapon, c.armor, c.weaponDesc, c.armorDesc
			if lvl == 35 {
				wName, aName, wDesc, aDesc = c.weapon35, c.armor35, c.weaponDesc35, c.armorDesc35
			}
			for _, g := range []Grade{GradeCommon, GradeRare} {
				f, name, aname := step, wName, aName
				var price int64 = int64(18000 * step)
				if g == GradeRare {
					f *= BridgeRareStep
					name, aname = name+" "+c.weaponRare, aname+" "+c.armorRare
					price *= 3
				}
				w := weapon(BridgeWeaponID(c.class, lvl, g), name, c.class, g, lvl, scaleBonus(c.weaponStats, f), price, c.model, c.off, wDesc)
				a := gear(BridgeArmorID(c.class, lvl, g), aname, KindArmor, []string{c.class}, g, lvl, scaleBonus(c.armorStats, f), price, "", "armor", aDesc)
				w.Bridge, a.Bridge = true, true
				out = append(out, w, a)
			}
		}
	}
	return out
}

// bridgeLoot: as peças de um nível nas duas raridades (cada uma com a sua chance), para a
// lista de saque de um monstro.
func bridgeLoot(level int, common, rare float64) []LootEntry {
	return append(bridgeLootGrade(level, GradeCommon, common), bridgeLootGrade(level, GradeRare, rare)...)
}

func bridgeLootGrade(level int, g Grade, chance float64) []LootEntry {
	var out []LootEntry
	for _, c := range bridgeClasses {
		out = append(out, LootEntry{BridgeWeaponID(c.class, level, g), chance, 1, 1}, LootEntry{BridgeArmorID(c.class, level, g), chance, 1, 1})
	}
	return out
}
