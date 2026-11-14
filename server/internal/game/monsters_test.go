package game

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestRingMonstersStayOutOfRaidLair(t *testing.T) {
	for seed := uint64(1); seed <= 20; seed++ {
		for _, sp := range ExpandSpawns(DefaultSpawns(), rand.New(rand.NewPCG(seed, seed))) {
			if sp.Monster.Raid {
				continue
			}
			if d := math.Hypot(sp.X-RaidLairX, sp.Z-RaidLairZ); d <= RaidLairRadius+6 {
				t.Fatalf("seed %d: %s spawned inside the raid lair (%.1f m)", seed, sp.Monster.ID, d)
			}
		}
	}
}

func TestRaidLootIsGuaranteed(t *testing.T) {
	def := Monsters["ozrhal"]
	rng := rand.New(rand.NewPCG(7, 7))
	legend := 0
	for range 200 {
		epics, frag := 0, false
		for _, d := range RollLoot(def, 25, rng) {
			switch {
			case d.Item.Grade == GradeEpic && d.Item.Equippable():
				epics++
			case d.Item.Grade == GradeLegendary:
				legend++
			case d.Item.ID == "raid_fragment":
				frag = true
			}
		}
		if epics != RaidEpicDrops || !frag {
			t.Fatalf("every kill drops %d epics and the fragment: %d %v", RaidEpicDrops, epics, frag)
		}
	}
	if legend < 50 || legend > 90 { // ~35% de 200
		t.Fatalf("legendary rate off: %d/200", legend)
	}
	if Monsters["bone_guard"] == nil || len(RollLoot(Monsters["bone_guard"], 20, rng)) != 0 {
		t.Fatal("summoned guards drop nothing")
	}
}

// Áreas do 30 ao 40: dentro do mapa, fora dos anéis e longe do covil do raid (a coleira do
// Ozrhal é de 30 m), no orçamento do roadmap (até 45 monstros por área), com elites de
// aviso longo e cor própria.
func TestLateAreas(t *testing.T) {
	perRegion := map[string]int{}
	for _, s := range DefaultSpawns() {
		def := Monsters[s.Monster]
		if s.Monster == "ozrhal" && s.LevelMin != 35 {
			t.Errorf("Ozrhal no nível %d", s.LevelMin)
		}
		if s.LevelMin < 30 || !s.Area {
			continue
		}
		perRegion[s.Region] += s.Count
		if !def.Elite || def.Tint == "" || len(def.Abilities) == 0 || s.LevelMax > MaxLevel {
			t.Errorf("%s: elite %v, cor %q, %d habilidades, até o %d", def.ID, def.Elite, def.Tint, len(def.Abilities), s.LevelMax)
		}
		if s.X-s.Radius < -150 || s.X+s.Radius > 150 || s.Z-s.Radius < -330 || s.Z+s.Radius > 150 {
			t.Errorf("%s em (%.0f, %.0f) sai do mapa", def.ID, s.X, s.Z)
		}
		if d := math.Hypot(s.X-RaidLairX, s.Z-RaidLairZ) - s.Radius; d < 30+8 {
			t.Errorf("%s a %.0f m do covil", def.ID, d)
		}
		if s.Z > -150 && math.Hypot(s.X, s.Z)-s.Radius < 140 {
			t.Errorf("%s dentro dos anéis", def.ID)
		}
	}
	for _, r := range []string{"Fundo do Vale", "Vigília Ossária", "Confins Partidos"} {
		if perRegion[r] == 0 || perRegion[r] > 45 {
			t.Errorf("%s: %d monstros", r, perRegion[r])
		}
	}
	// Os Confins são os cantos dos Ermos Partidos: com os 20 do anel, cabem no orçamento.
	if n := perRegion["Confins Partidos"] + 20; n > 45 {
		t.Errorf("Ermos + Confins: %d", n)
	}
}
