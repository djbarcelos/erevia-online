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
