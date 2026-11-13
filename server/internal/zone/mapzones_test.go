package zone

import (
	"io"
	"log/slog"
	"testing"

	"erevia/server/internal/game"
)

func TestMapZonesMergesRingsAndKeepsBoss(t *testing.T) {
	zones := mapZones(game.DefaultSpawns())
	var rings, bosses, raids, areas int
	for _, z := range zones {
		if z.Area {
			areas++
			if z.Region != "Vale das Pedras Vivas" || z.Radius <= 0 || z.Z > -150 || z.Name != "Golem de Pedra" || z.LevelMin != 20 || z.LevelMax != 30 {
				t.Errorf("área inesperada: %+v", z)
			}
			continue
		}
		if z.Fixed && z.Raid {
			raids++
			if !z.Boss || z.Name != "Ozrhal, o Rei Ossário" || z.X != game.RaidLairX || z.Z != game.RaidLairZ {
				t.Errorf("raid boss inesperado: %+v", z)
			}
			continue
		}
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
	// 5 spawns em anel, mas só 3 faixas de raio diferentes (e o vale é uma área à parte).
	if rings != 3 || bosses != 1 || raids != 1 || areas != 1 {
		t.Fatalf("esperava 3 anéis, 1 chefe, 1 raid e 1 área, veio %d, %d, %d e %d", rings, bosses, raids, areas)
	}
	if zones[1].Name != "Esqueleto Guerreiro, Esqueleto Ladino" {
		t.Errorf("monstros do anel do meio não foram juntados: %q", zones[1].Name)
	}
}

func TestBossFlagOnEntity(t *testing.T) {
	z := New(testConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
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
