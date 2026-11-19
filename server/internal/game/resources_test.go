package game

import (
	"math"
	"testing"
)

func TestResourceStateRules(t *testing.T) {
	d := newResource("Teste", "#ffffff", map[GenEvent]float64{GenBasicHit: 10})
	var s ResourceState
	if got := s.Gain(d, 20, 0); got != 20 {
		t.Fatalf("ganho: %v", got)
	}
	if got := s.Gain(d, 20, 0.5); got != 5 || s.Value != 25 {
		t.Errorf("teto de 25/s: ganhou %v, barra %v", got, s.Value)
	}
	s.Gain(d, 25, 1.5)
	s.Gain(d, 25, 2.6)
	s.Gain(d, 25, 3.7)
	if s.Value != d.Max {
		t.Errorf("máximo: %v", s.Value)
	}
	if s.Spend(101) || !s.Spend(40) || s.Value != 60 {
		t.Errorf("gastar: %v", s.Value)
	}
	if s.Gain(d, math.NaN(), 10) != 0 || s.Gain(d, -5, 10) != 0 {
		t.Error("NaN ou negativo entrou")
	}
	// Fora de combate (5 s): cai 10/s.
	s.Tick(d, 20, 18, 1)
	if s.Value != 60 {
		t.Errorf("caiu em combate: %v", s.Value)
	}
	s.Tick(d, 20, 10, 1)
	if s.Value != 50 {
		t.Errorf("queda fora de combate: %v", s.Value)
	}
	if v := s.SpendAll(); v != 50 || s.Value != 0 {
		t.Errorf("gastar tudo: %v", v)
	}
}

func TestEveryEvolutionHasAResource(t *testing.T) {
	for _, sp := range SpecList {
		r := sp.Resource
		if r == nil || r.Name == "" || len(r.Color) != 7 || r.Max != 100 || r.PerSecondCap != 25 || r.TargetCap != 3 {
			t.Errorf("%s: recurso %+v", sp.ID, r)
			continue
		}
		// Alguma coisa enche a barra (evento geral, distância ou skill da árvore).
		fills := len(r.Ranged) > 0
		for _, v := range r.Gens {
			fills = fills || v > 0
		}
		for _, s := range EvolutionTree(sp.ID) {
			fills = fills || s.ResOnCast > 0 || s.ResOnHit > 0
		}
		if !fills {
			t.Errorf("%s: nada gera %s", sp.ID, r.Name)
		}
		// As 2 iniciais custam só mana (o ciclo básico funciona sem recurso).
		for _, id := range sp.StarterSkills {
			if SkillResCost(Skills[id]) > 0 {
				t.Errorf("%s: a inicial %s custa recurso", sp.ID, Skills[id].Name)
			}
		}
	}
	if ResourceOf("knight", "") != nil || ResourceOf("knight", "bastion").Name != "Determinação" {
		t.Error("ResourceOf")
	}
}

func TestBackstabCone(t *testing.T) {
	// Alvo olhando para +Z (yaw 0): atrás é -Z.
	if !Backstab(0, 0, -2) || !Backstab(0, 0.5, -2) {
		t.Error("atrás deveria contar")
	}
	if Backstab(0, 0, 2) || Backstab(0, 2, 0) {
		t.Error("frente e lado não contam")
	}
	// Olhando para +X (yaw π/2): atrás é -X.
	if !Backstab(math.Pi/2, -2, 0) || Backstab(math.Pi/2, 2, 0) {
		t.Error("yaw π/2")
	}
}
