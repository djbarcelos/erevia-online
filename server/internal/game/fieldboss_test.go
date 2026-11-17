package game

import "testing"

// Os chefes de campo existentes (tunnel#132 e #138): 3 fases, janela e habilidades por fase.
func TestFieldBossesHaveThreePhases(t *testing.T) {
	for id, want := range map[string]struct {
		lo, hi float64
		marks  [2]float64
	}{
		"mae_lodo": {60, 90, [2]float64{0.66, 0.33}},
		"bone_lord": {30, 45, [2]float64{0.60, 0.25}},
	} {
		m := Monsters[id]
		f := m.FieldBoss
		if f == nil || f.PhaseCount() != 3 || f.RespawnMin != want.lo || f.RespawnMax != want.hi {
			t.Fatalf("%s: chefe de campo de 3 fases e janela %v–%v min: %+v", id, want.lo, want.hi, f)
		}
		for i, mk := range want.marks {
			if f.Phases[i].AtHP != mk {
				t.Errorf("%s: fase %d começa em %.2f, esperava %.2f", id, i+2, f.Phases[i].AtHP, mk)
			}
		}
		later := 0
		for _, a := range m.Abilities {
			if a.MinPhase > 1 {
				later++
			}
		}
		if later == 0 {
			t.Errorf("%s: nenhuma habilidade de fase tardia", id)
		}
	}
	// Senhor dos Ossos: círculo de 4 m (2 s), linha de 12 m na fase 3 (1,5 s) e 3 Guerreiros na fase 2.
	b := Monsters["bone_lord"]
	if a := b.Abilities[0]; a.Shape != ShapeCircle || a.Radius != 4 || a.Delay != 2 {
		t.Errorf("Golpe Giratório: %+v", a)
	}
	if a := b.Abilities[1]; a.Shape != ShapeLine || a.Length != 12 || a.Delay != 1.5 || a.MinPhase != 3 {
		t.Errorf("Marcha dos Mortos: %+v", a)
	}
	if p := b.FieldBoss.Phases[0]; p.Summon != "skeleton_warrior" || p.SummonCount != 3 || !b.FieldBoss.Phases[1].Enrage {
		t.Errorf("fases do Senhor dos Ossos: %+v", b.FieldBoss.Phases)
	}
}

func TestValidateFieldBoss(t *testing.T) {
	ok := func() *MonsterDef {
		return &MonsterDef{ID: "x", Boss: true, FieldBoss: &FieldBossDef{RespawnMin: 10, RespawnMax: 20, Phases: []FieldPhase{{AtHP: 0.6}, {AtHP: 0.3}}}}
	}
	if err := ValidateFieldBoss(ok()); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*MonsterDef){
		"sem Boss":           func(m *MonsterDef) { m.Boss = false },
		"é Raid":             func(m *MonsterDef) { m.Raid = true },
		"janela invertida":   func(m *MonsterDef) { m.FieldBoss.RespawnMax = 5 },
		"sem renascimento":   func(m *MonsterDef) { m.FieldBoss.RespawnMin, m.FieldBoss.RespawnMax = 0, 0 },
		"fases fora de ordem": func(m *MonsterDef) { m.FieldBoss.Phases[1].AtHP = 0.7 },
		"fase em 100%":       func(m *MonsterDef) { m.FieldBoss.Phases[0].AtHP = 1 },
		"invoca quem não existe": func(m *MonsterDef) {
			m.FieldBoss.Phases[0].Summon, m.FieldBoss.Phases[0].SummonCount = "nao_existe", 1
		},
		"volta sem cura": func(m *MonsterDef) {
			p := &m.FieldBoss.Phases[0]
			p.Summon, p.SummonCount, p.ReturnAfter = "lodo_menor", 2, 30
		},
		"habilidade de fase 4": func(m *MonsterDef) { m.Abilities = []AbilityDef{{ID: "a", MinPhase: 4}} },
	} {
		m := ok()
		mutate(m)
		if ValidateFieldBoss(m) == nil {
			t.Errorf("%s: deveria recusar", name)
		}
	}
}
