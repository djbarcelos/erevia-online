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
			switch z.Region {
			case "Vale das Pedras Vivas":
				if z.Radius <= 0 || z.Z > -150 || z.Name != "Golem de Pedra" || z.LevelMin != 20 || z.LevelMax != 30 {
					t.Errorf("área inesperada: %+v", z)
				}
			case "Vigília Ossária":
				// Os dois monstros do mesmo disco viram uma área só.
				if z.Name != "Cavaleiro Ossário, Devoto do Ossário" || z.LevelMin != 33 || z.LevelMax != 38 {
					t.Errorf("área inesperada: %+v", z)
				}
			case "Fundo do Vale", "Confins Partidos":
				if z.LevelMin < 30 || z.LevelMax > game.MaxLevel {
					t.Errorf("área inesperada: %+v", z)
				}
			default:
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
	// 5 spawns em anel, mas só 3 faixas de raio diferentes. Áreas: o Vale, o Fundo do Vale,
	// os 2 discos da Vigília e os 3 cantos dos Confins.
	if rings != 3 || bosses != 1 || raids != 1 || areas != 7 {
		t.Fatalf("esperava 3 anéis, 1 chefe, 1 raid e 7 áreas, veio %d, %d, %d e %d", rings, bosses, raids, areas)
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

// As elites do 30 ao 40 vão com a cor da variante; os outros monstros, sem.
func TestEntityCarriesVariantTint(t *testing.T) {
	z := newCombatZone(fixed("ancient_golem", 32, 20, 0), fixed("stone_golem", 25, 30, 0))
	for _, m := range z.monsters {
		got := z.entityPB(m).Tint
		if got != m.mon.def.Tint || (m.mon.def.ID == "ancient_golem") != (got != "") {
			t.Errorf("%s: cor %q", m.mon.def.ID, got)
		}
	}
}
