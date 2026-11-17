package zone

import (
	"io"
	"log/slog"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

func TestMapZonesMergesRingsAndKeepsBoss(t *testing.T) {
	zones := mapZones(game.DefaultSpawns())
	var rings, bosses, raids, areas, brejo, bosque, mortas, sectors int
	for _, z := range zones {
		if z.Area {
			areas++
			switch z.Region {
			case "Vale das Pedras Vivas":
				if z.Radius <= 0 || z.Z > -150 || z.Name != "Golem de Pedra, Golem de Musgo, Golem de Cascalho" || z.LevelMin != 20 || z.LevelMax != 30 {
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
			case "":
				// O acampamento dos Saqueadores (matilha, tunnel#129): um disco pequeno nos Campos.
				if z.Name != "Saqueador Ossudo" || z.Radius > 8 || z.LevelMin != 8 || z.LevelMax != 10 {
					t.Errorf("área inesperada: %+v", z)
				}
			case game.BrejoRegion:
				// Os grupos do Brejo (tunnel#139): discos de 12 m (6 m o acampamento) dentro do bioma, do 14 ao 20.
				if z.Radius <= 0 || z.Radius > 12 || z.X < game.BrejoMinX || z.X > game.BrejoMaxX || z.Z < game.BrejoMinZ || z.Z > game.BrejoMaxZ || z.LevelMin < 14 || z.LevelMax > 20 {
					t.Errorf("área inesperada: %+v", z)
				}
				brejo++
			case game.BosqueRegion:
				// Os grupos do Bosque Gris (tunnel#137): discos de 12 m (6 m a matilha dos lobos), do 7 ao 15.
				if z.Radius <= 0 || z.Radius > 12 || z.X > -10 || z.Z < 10 || z.LevelMin < 7 || z.LevelMax > 15 {
					t.Errorf("área inesperada: %+v", z)
				}
				bosque++
			case game.TerrasMortasRegion:
				// Os grupos das Terras Mortas: discos de 12 m (6 m a matilha dos Carniçais), do 24 ao 28.
				if z.Radius <= 0 || z.Radius > 12 || z.X < 100 || z.LevelMin < 24 || z.LevelMax > 28 {
					t.Errorf("área inesperada: %+v", z)
				}
				mortas++
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
			if !z.Boss || (z.Name != "Senhor dos Ossos" || z.Z != -132) && (z.Name != "Mãe-Lodo" || z.X != game.MotherX || z.Z != game.MotherZ) &&
				(z.Name != "Corvo Rubro" || z.X != game.CrowDenX || z.Z != game.CrowDenZ) {
				t.Errorf("chefe inesperado: %+v", z)
			}
			continue
		}
		if z.AngleMax > z.AngleMin { // os setores do Bosque Gris e das Terras Mortas (com dono, sem monstros)
			sectors++
			if z.Region != game.BosqueRegion && z.Region != game.TerrasMortasRegion {
				t.Errorf("setor inesperado: %+v", z)
			}
			continue
		}
		rings++
		if z.RMin >= z.RMax || z.LevelMin > z.LevelMax {
			t.Errorf("anel inválido: %+v", z)
		}
	}
	// 5 spawns em anel, mas só 3 faixas de raio diferentes. Áreas: o Vale, o Fundo do Vale,
	// os 2 discos da Vigília, os 3 cantos dos Confins e o acampamento dos Saqueadores. O Brejo traz 9 discos (os 7 grupos de 12 m e os 2
	// acampamentos de 6 m dos Lanceiros, um deles no mesmo lugar de um grupo) e a Mãe-Lodo.
	// O Bosque Gris traz 5 discos e o Corvo Rubro; as Terras Mortas, 6 (um dos grupos tem a matilha dos Carniçais à parte); os dois
	// setores (um de cada) só dão dono ao pedaço do anel.
	if rings != 3 || bosses != 3 || raids != 1 || areas != 8+brejo+bosque+mortas || brejo != 9 || bosque != 5 || mortas != 6 || sectors != 2 {
		t.Fatalf("esperava 3 anéis, 3 chefes, 1 raid, 2 setores e 8 áreas + 9 do Brejo + 5 do Bosque + 6 das Terras Mortas, veio %d, %d, %d, %d, %d (%d, %d, %d)", rings, bosses, raids, sectors, areas, brejo, bosque, mortas)
	}
	if zones[1].Name != "Esqueleto Guerreiro, Esqueleto Ladino, Esqueleto Lançador, Esqueleto Espreitador, Ossada Instável" {
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

func TestMapZonesDrawSectors(t *testing.T) {
	east := game.Spawn{Monster: "skeleton_minion", Count: 3, RMin: 46, RMax: 74, LevelMin: 2, LevelMax: 4,
		AngleMin: 340, AngleMax: 20, Region: "Leste de Teste"}
	spawns := append(game.DefaultSpawns(), periodSector("skeleton_rogue", game.WhenDay), periodSector("skeleton_mage", game.WhenNight), east)
	var rings int
	sectors := map[string]*pb.MapZone{}
	for _, z := range mapZones(spawns) {
		switch {
		case z.AngleMax > z.AngleMin:
			sectors[z.Region] = z
		case !z.Fixed && !z.Area:
			rings++
		}
	}
	if rings != 3 {
		t.Errorf("os setores não podem virar anéis nem se juntar a eles: %d anéis", rings)
	}
	b := sectors["Bosque de Teste"]
	if b == nil || b.AngleMin != 100 || b.AngleMax != 170 || b.RMin != 80 || b.RMax != 150 || b.LevelMin != 7 || b.LevelMax != 15 {
		t.Fatalf("setor do bosque: %+v", b)
	}
	if b.Name != "Esqueleto Ladino (de dia), Esqueleto Mago (à noite)" {
		t.Errorf("monstros do setor (com o período): %q", b.Name)
	}
	e := sectors["Leste de Teste"]
	if e == nil || e.AngleMin != 340 || e.AngleMax != 380 || e.Name != "Esqueleto Lacaio" {
		t.Fatalf("o setor que cruza o 0° vai de 340 a 380: %+v", e)
	}
}
