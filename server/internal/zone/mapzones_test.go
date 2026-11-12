package zone

import (
	"io"
	"log/slog"
	"testing"

	"erevia/server/internal/game"
)

func TestMapZonesMergesRingsAndKeepsBoss(t *testing.T) {
	zones := mapZones(game.DefaultSpawns())
	var rings, bosses int
	for _, z := range zones {
		if z.Fixed {
			bosses++
			if !z.Boss || z.Name != "Senhor dos Ossos" || z.Z != -132 {
				t.Errorf("chefe inesperado: %+v", z)
			}
			continue
		}
		rings++
		if z.RMin >= z.RMax || z.LevelMin > z.LevelMax {
			t.Errorf("anel inválido: %+v", z)
		}
	}
	// 5 spawns em anel, mas só 3 faixas de raio diferentes.
	if rings != 3 || bosses != 1 {
		t.Fatalf("esperava 3 anéis e 1 chefe, veio %d e %d", rings, bosses)
	}
	if zones[1].Name != "Esqueleto Guerreiro, Esqueleto Ladino" {
		t.Errorf("monstros do anel do meio não foram juntados: %q", zones[1].Name)
	}
}

func TestBossFlagOnEntity(t *testing.T) {
	z := New(DefaultConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	var found bool
	for _, m := range z.monsters {
		if m.mon.def.Boss {
			found = true
			if !z.entityPB(m).Boss {
				t.Error("entidade do chefe sem a flag boss")
			}
		} else if z.entityPB(m).Boss {
			t.Errorf("%s marcado como chefe", m.name)
		}
	}
	if !found {
		t.Fatal("nenhum chefe na zona padrão")
	}
}
