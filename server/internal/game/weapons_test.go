package game

import (
	"math"
	"testing"
)

// Golden: alcance, intervalo, "mágico" e PAtk de cada classe com a arma inicial (a do tipo da
// classe), sem arma e, no mago, com varinha. Os valores são os de antes de o alcance e o
// intervalo passarem a vir da arma (ClassDef.Base): a mudança não pode mexer neles.
// dexInterval é o intervalo com +10 de Destreza sobre a base da classe.
var goldenWeaponStats = []struct {
	class       string
	level       int
	weapon      string
	rng, intv   float64
	magic       bool
	patk        float64
	dexInterval float64
}{
	{"knight", 1, "sword", 2.2, 1.2, false, 8, 1.0169491525423728},
	{"knight", 1, "", 2.2, 1.2, false, 8, 1.0169491525423728},
	{"knight", 20, "sword", 2.2, 1.2, false, 49.800000000000004, 1.0169491525423728},
	{"knight", 20, "", 2.2, 1.2, false, 49.800000000000004, 1.0169491525423728},
	{"knight", 40, "sword", 2.2, 1.2, false, 93.80000000000001, 1.0169491525423728},
	{"knight", 40, "", 2.2, 1.2, false, 93.80000000000001, 1.0169491525423728},
	{"barbarian", 1, "axe", 2.2, 1.4, false, 10, 1.1666666666666667},
	{"barbarian", 1, "", 2.2, 1.4, false, 10, 1.1666666666666667},
	{"barbarian", 20, "axe", 2.2, 1.4, false, 67, 1.1666666666666667},
	{"barbarian", 20, "", 2.2, 1.4, false, 67, 1.1666666666666667},
	{"barbarian", 40, "axe", 2.2, 1.4, false, 127, 1.1666666666666667},
	{"barbarian", 40, "", 2.2, 1.4, false, 127, 1.1666666666666667},
	{"mage", 1, "staff", 12, 1.6, true, 6, 1.3445378151260505},
	{"mage", 1, "", 12, 1.6, true, 6, 1.3445378151260505},
	{"mage", 20, "staff", 12, 1.6, true, 17.4, 1.3445378151260505},
	{"mage", 20, "", 12, 1.6, true, 17.4, 1.3445378151260505},
	{"mage", 40, "staff", 12, 1.6, true, 29.4, 1.3445378151260505},
	{"mage", 40, "", 12, 1.6, true, 29.4, 1.3445378151260505},
	{"rogue", 1, "daggers", 2.2, 0.8, false, 7, 0.634920634920635},
	{"rogue", 1, "", 2.2, 0.8, false, 7, 0.634920634920635},
	{"rogue", 20, "daggers", 2.2, 0.8, false, 45, 0.634920634920635},
	{"rogue", 20, "", 2.2, 0.8, false, 45, 0.634920634920635},
	{"rogue", 40, "daggers", 2.2, 0.8, false, 85, 0.634920634920635},
	{"rogue", 40, "", 2.2, 0.8, false, 85, 0.634920634920635},
	{"mage", 1, "wand", 12, 1.6, true, 6, 1.3445378151260505},
	{"mage", 20, "wand", 12, 1.6, true, 17.4, 1.3445378151260505},
	{"mage", 40, "wand", 12, 1.6, true, 29.4, 1.3445378151260505},
}

func TestWeaponStatsGolden(t *testing.T) {
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	for _, g := range goldenWeaponStats {
		s := CharacterStats(g.class, "", g.level, Attrs{}, Stats{}, nil, g.weapon)
		d := CharacterStats(g.class, "", g.level, Attrs{Dex: ClassBaseAttrs[g.class].Dex + 10}, Stats{}, nil, g.weapon)
		if !near(s.Range, g.rng) || !near(s.AtkInterval, g.intv) || s.Magic != g.magic || !near(s.PAtk, g.patk) || !near(d.AtkInterval, g.dexInterval) {
			t.Errorf("%s nv%d %q: alcance %v intervalo %v mágico %v PAtk %v (Dex+10: %v), esperado %v %v %v %v (%v)",
				g.class, g.level, g.weapon, s.Range, s.AtkInterval, s.Magic, s.PAtk, d.AtkInterval,
				g.rng, g.intv, g.magic, g.patk, g.dexInterval)
		}
	}
}

// Os números de cada arma original são os da base da classe que a usa.
func TestWeaponsMatchClassBase(t *testing.T) {
	for class, wt := range map[string]string{"knight": WeaponSword, "barbarian": WeaponAxe, "mage": WeaponStaff, "rogue": WeaponDaggers} {
		b, w := Classes[class].Base, Weapons[wt]
		if b.Range != w.Range || b.AtkInterval != w.Interval || b.Magic != w.Magic {
			t.Errorf("%s/%s: arma %v %v %v, base %v %v %v", class, wt, w.Range, w.Interval, w.Magic, b.Range, b.AtkInterval, b.Magic)
		}
	}
	if s, w := Weapons[WeaponStaff], Weapons[WeaponWand]; s.Range != w.Range || s.Interval != w.Interval || s.Magic != w.Magic {
		t.Error("a varinha tem os números do cajado")
	}
}

func TestCanWieldAndOffhand(t *testing.T) {
	cases := []struct {
		class, wt string
		want      bool
	}{
		{"knight", WeaponSword, true}, {"knight", WeaponAxe, false}, {"knight", WeaponWand, false},
		{"barbarian", WeaponAxe, true}, {"barbarian", WeaponSword, false},
		{"mage", WeaponStaff, true}, {"mage", WeaponWand, true}, {"mage", WeaponDaggers, false},
		{"rogue", WeaponDaggers, true}, {"rogue", WeaponBow, false}, {"nope", WeaponSword, false},
	}
	for _, c := range cases {
		if got := CanWield(c.class, "", c.wt); got != c.want {
			t.Errorf("CanWield(%s, %s) = %v", c.class, c.wt, got)
		}
	}
	for class, want := range map[string]bool{"knight": true, "barbarian": true, "mage": false, "rogue": false} {
		if CanOffhand(class, "", OffhandShield) != want {
			t.Errorf("CanOffhand(%s, shield) = %v", class, !want)
		}
	}
	if CanOffhand("mage", "", OffhandGrimoire) {
		t.Error("o grimório ainda não é de ninguém")
	}
	// A lista da especialização, quando existe, vale no lugar da classe.
	spec := SpecList[0]
	saved := spec.Weapons
	defer func() { spec.Weapons = saved }()
	spec.Weapons = []string{WeaponBow}
	if !CanWield(spec.Class, spec.ID, WeaponBow) || CanWield(spec.Class, spec.ID, Classes[spec.Class].Weapons[0]) {
		t.Error("a especialização com lista própria troca a da classe")
	}
}

func TestShieldClassesFollowOffhands(t *testing.T) {
	if len(shieldClasses) != 2 || shieldClasses[0] != "knight" || shieldClasses[1] != "barbarian" {
		t.Errorf("shieldClasses = %v", shieldClasses)
	}
}
