package game

import "sort"

// Armas com identidade própria: o alcance, o intervalo do ataque básico, se ele é mágico e a
// empunhadura vêm do tipo da arma equipada, não da classe. Sem arma, valem os números da
// base da classe (ClassDef.Base).

// Tipos de arma novos (os de espada, machado, cajado e adagas estão em items.go). axe2h, bow,
// crossbow e crossbow2h ainda não têm item.
const (
	WeaponWand       = "wand"
	WeaponAxe2H      = "axe2h"
	WeaponBow        = "bow"
	WeaponCrossbow   = "crossbow"
	WeaponCrossbow2H = "crossbow2h"
)

// Tipos de mão secundária.
const (
	OffhandShield   = "shield"
	OffhandGrimoire = "grimoire"
)

// Animações e projéteis (nomes para o cliente; ver WeaponInfo).
const (
	weaponAnimSlash  = "slash"
	weaponAnimChop   = "chop"
	weaponAnimHeavy  = "heavy"
	weaponAnimCast   = "cast"
	weaponAnimStab   = "stab"
	weaponAnimShoot  = "shoot"
	projectileArcane = "arcane_bolt"
	projectileArrow  = "arrow"
	projectileBolt   = "bolt"
)

// WeaponDef descreve um tipo de arma.
type WeaponDef struct {
	Type, Name string
	Range      float64 // alcance do ataque básico (m)
	Interval   float64 // segundos entre ataques básicos (antes da Destreza)
	Magic      bool    // o ataque básico usa MAtk/MDef
	Hands      int     // 1 ou 2; a de 2 mãos ocupa a mão secundária
	Dual       bool    // arma dupla: também ocupa a mão secundária
	Anim       string  // animação do ataque
	Projectile string  // projétil ("" = corpo a corpo)
}

// Weapons: os tipos de arma. Os números de sword, axe, staff e daggers são EXATAMENTE os de
// ClassDef.Base da classe que os usa (o TestWeaponsMatchClassBase confere) e a varinha
// repete os do cajado. axe2h, bow, crossbow e crossbow2h são PROVISÓRIOS: nenhum item os usa
// ainda, e os números serão balanceados quando entrarem.
var Weapons = map[string]*WeaponDef{
	WeaponSword:      {Type: WeaponSword, Name: "Espada", Range: 2.2, Interval: 1.2, Hands: 1, Anim: weaponAnimSlash},
	WeaponAxe:        {Type: WeaponAxe, Name: "Machado", Range: 2.2, Interval: 1.4, Hands: 1, Anim: weaponAnimChop},
	WeaponAxe2H:      {Type: WeaponAxe2H, Name: "Machado de Duas Mãos", Range: 2.4, Interval: 1.8, Hands: 2, Anim: weaponAnimHeavy}, // provisório
	WeaponStaff:      {Type: WeaponStaff, Name: "Cajado", Range: 12, Interval: 1.6, Magic: true, Hands: 1, Anim: weaponAnimCast, Projectile: projectileArcane},
	WeaponWand:       {Type: WeaponWand, Name: "Varinha", Range: 12, Interval: 1.6, Magic: true, Hands: 1, Anim: weaponAnimCast, Projectile: projectileArcane},
	WeaponDaggers:    {Type: WeaponDaggers, Name: "Adagas", Range: 2.2, Interval: 0.8, Hands: 1, Dual: true, Anim: weaponAnimStab},
	WeaponBow:        {Type: WeaponBow, Name: "Arco", Range: 14, Interval: 1.5, Hands: 2, Anim: weaponAnimShoot, Projectile: projectileArrow},             // provisório
	WeaponCrossbow:   {Type: WeaponCrossbow, Name: "Besta", Range: 13, Interval: 1.7, Hands: 1, Anim: weaponAnimShoot, Projectile: projectileBolt},         // provisório
	WeaponCrossbow2H: {Type: WeaponCrossbow2H, Name: "Besta Pesada", Range: 15, Interval: 2.2, Hands: 2, Anim: weaponAnimShoot, Projectile: projectileBolt}, // provisório
}

// WeaponTypes: os tipos em ordem estável (o Welcome vai sempre igual).
func WeaponTypes() []string {
	out := make([]string, 0, len(Weapons))
	for t := range Weapons {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// TwoHanded: a arma ocupa a mão secundária (2 mãos ou dupla).
func (w *WeaponDef) TwoHanded() bool { return w != nil && (w.Hands >= 2 || w.Dual) }

// ApplyWeapon sobrescreve alcance, intervalo e "mágico" com os da arma. Tipo vazio ou
// desconhecido (sem arma) deixa a base da classe. Roda antes de ApplyAttrs: a Destreza
// escala o intervalo.
func ApplyWeapon(s Stats, wt string) Stats {
	if w := Weapons[wt]; w != nil {
		s.Range, s.AtkInterval, s.Magic = w.Range, w.Interval, w.Magic
	}
	return s
}

// Armas e mãos secundárias permitidas: da especialização, ou da classe quando a
// especialização não define as dela.
func allowed(class, spec string, pick func(weapons, offhands []string) []string) []string {
	c := Classes[class]
	if c == nil {
		return nil
	}
	if sp := SpecFor(class, spec); sp != nil {
		if l := pick(sp.Weapons, sp.Offhands); len(l) > 0 {
			return l
		}
	}
	return pick(c.Weapons, c.Offhands)
}

func has(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// CanWield informa se a classe (com a especialização) usa o tipo de arma.
func CanWield(class, spec, weaponType string) bool {
	return has(allowed(class, spec, func(w, _ []string) []string { return w }), weaponType)
}

// CanOffhand informa se a classe (com a especialização) usa o tipo de mão secundária
// (OffhandShield, OffhandGrimoire).
func CanOffhand(class, spec, kind string) bool {
	return has(allowed(class, spec, func(_, o []string) []string { return o }), kind)
}

// OffhandType: o tipo de mão secundária do item ("" = não vai na mão secundária).
func (d *ItemDef) OffhandType() string {
	if d.Kind == KindShield {
		return OffhandShield
	}
	return ""
}

// Hands: quantas mãos a arma do item ocupa (0 = não é arma). A empunhadura vem do tipo da
// arma, nunca do modelo.
func (d *ItemDef) Hands() int {
	if d.Kind != KindWeapon {
		return 0
	}
	if w := Weapons[d.WeaponType]; w != nil {
		return w.Hands
	}
	return 1
}

// TwoHanded: o item é uma arma que ocupa a mão secundária.
func (d *ItemDef) TwoHanded() bool {
	return d.Kind == KindWeapon && Weapons[d.WeaponType].TwoHanded()
}

// generatedWeaponType: o tipo das armas geradas (graus e ponte): o da classe, e varinha
// quando o modelo é a varinha.
func generatedWeaponType(class, model string) string {
	if model == "wand" {
		return WeaponWand
	}
	switch class {
	case "barbarian":
		return WeaponAxe
	case "mage":
		return WeaponStaff
	case "rogue":
		return WeaponDaggers
	}
	return WeaponSword
}

// shieldClasses: as classes que usam escudo (as peças de escudo do catálogo levam essa lista
// em Classes; a fonte é ClassDef.Offhands).
var shieldClasses = classesWithOffhand(OffhandShield)

func classesWithOffhand(kind string) []string {
	var out []string
	for _, id := range []string{"knight", "barbarian", "mage", "rogue"} {
		if has(Classes[id].Offhands, kind) {
			out = append(out, id)
		}
	}
	return out
}
