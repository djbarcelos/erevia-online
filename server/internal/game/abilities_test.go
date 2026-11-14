package game

import "testing"

func TestEveryMonsterAbilityIsValid(t *testing.T) {
	n := 0
	for id, m := range Monsters {
		if err := ValidateAbilities(m); err != nil {
			t.Errorf("%s: %v", id, err)
		}
		n += len(m.Abilities)
	}
	if n < 5 {
		t.Fatalf("Ozrhal (3), golem and bone lord have abilities: got %d", n)
	}
}

func TestAbilityMinimumDelay(t *testing.T) {
	slam := func(delay float64) []AbilityDef {
		return []AbilityDef{{ID: "x", Shape: ShapeCircle, Radius: 3, Delay: delay, Cooldown: 8, Mult: 1}}
	}
	cases := []struct {
		name string
		def  MonsterDef
		ok   bool
	}{
		{"comum com 1,0 s", MonsterDef{ID: "a", Abilities: slam(1.0)}, true},
		{"comum com 0,9 s", MonsterDef{ID: "a", Abilities: slam(0.9)}, false},
		{"elite com 1,4 s", MonsterDef{ID: "a", Elite: true, Abilities: slam(1.4)}, false},
		{"elite com 1,5 s", MonsterDef{ID: "a", Elite: true, Abilities: slam(1.5)}, true},
		{"chefe com 1,2 s", MonsterDef{ID: "a", Boss: true, Abilities: slam(1.2)}, false},
		{"raid com 1,5 s", MonsterDef{ID: "a", Raid: true, Abilities: slam(1.5)}, true},
	}
	for _, c := range cases {
		if err := ValidateAbilities(&c.def); (err == nil) != c.ok {
			t.Errorf("%s: err = %v", c.name, err)
		}
	}
}

func TestAbilityValidationCatchesBadShapes(t *testing.T) {
	base := AbilityDef{ID: "x", Delay: 1.5, Cooldown: 8, Mult: 1}
	with := func(f func(a *AbilityDef)) *MonsterDef {
		a := base
		f(&a)
		return &MonsterDef{ID: "m", Abilities: []AbilityDef{a}}
	}
	bad := map[string]*MonsterDef{
		"recarga menor que o atraso": with(func(a *AbilityDef) { a.Shape, a.Radius, a.Cooldown = ShapeCircle, 3, 1 }),
		"cone sem abertura":          with(func(a *AbilityDef) { a.Shape, a.Radius = ShapeCone, 5 }),
		"linha sem largura":          with(func(a *AbilityDef) { a.Shape, a.Length = ShapeLine, 6 }),
		"anel com buraco maior":      with(func(a *AbilityDef) { a.Shape, a.Inner, a.Radius = ShapeDonut, 6, 4 }),
		"marca em cone":              with(func(a *AbilityDef) { a.Shape, a.Radius, a.Arc, a.Target = ShapeCone, 5, 60, AimMark }),
		"longe demais":               with(func(a *AbilityDef) { a.Shape, a.Length, a.Width = ShapeLine, 40, 2 }),
		"id repetido": {ID: "m", Abilities: []AbilityDef{
			{ID: "x", Shape: ShapeCircle, Radius: 2, Delay: 1, Cooldown: 5}, {ID: "x", Shape: ShapeCircle, Radius: 2, Delay: 1, Cooldown: 5}}},
	}
	for name, m := range bad {
		if ValidateAbilities(m) == nil {
			t.Errorf("%s: should be rejected", name)
		}
	}
}
