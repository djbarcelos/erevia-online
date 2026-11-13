package game

import (
	"math"
	"strings"
	"testing"
)

func TestHuntPlaceRingPointsTowardThePlayer(t *testing.T) {
	sp := DefaultSpawns()
	for _, c := range []struct{ px, pz, ax, az float64 }{{10, 0, 1, 0}, {0, -10, 0, -1}, {-3, 4, -0.6, 0.8}} {
		pl, ok := HuntPlace(sp, []string{"skeleton_minion"}, 1, c.px, c.pz)
		if !ok || pl.Kind != PlaceHunt || pl.Name != "Esqueleto Lacaio" {
			t.Fatalf("minion place = %+v, %v", pl, ok)
		}
		r := math.Hypot(pl.X, pl.Z)
		if r-pl.Radius < 46-1e-9 || r+pl.Radius > 74+1e-9 || pl.Radius <= 0 {
			t.Fatalf("circle r=%.1f radius=%.1f leaves the 46–74 ring", r, pl.Radius)
		}
		if math.Abs(pl.X/r-c.ax) > 1e-6 || math.Abs(pl.Z/r-c.az) > 1e-6 {
			t.Fatalf("player at (%v, %v): point (%.1f, %.1f) is not in the player's direction", c.px, c.pz, pl.X, pl.Z)
		}
	}
	// No meio da cidade: para o leste.
	if pl, _ := HuntPlace(sp, []string{"skeleton_minion"}, 1, 0, 0); pl.Z != 0 || pl.X <= 0 {
		t.Fatalf("from the town center the point should be east, got %+v", pl)
	}
}

func TestHuntPlaceFollowsPlayerLevel(t *testing.T) {
	sp := DefaultSpawns()
	pick := func(m []string, level int) QuestPlace {
		t.Helper()
		pl, ok := HuntPlace(sp, m, level, 50, 0)
		if !ok {
			t.Fatalf("%v at level %d: no place", m, level)
		}
		return pl
	}
	bones := []string{"skeleton_warrior", "skeleton_rogue"}
	low := pick(bones, 6)
	if r := math.Hypot(low.X, low.Z); r < 80 || r > 108 {
		t.Fatalf("warrior/rogue at level 6: r = %.1f, want the 80–108 ring", r)
	}
	if !strings.Contains(low.Name, "Esqueleto Guerreiro") || !strings.Contains(low.Name, "Esqueleto Ladino") {
		t.Fatalf("name = %q, want both monsters", low.Name)
	}
	if r := math.Hypot(pick(bones, 5).X, 0); r < 80 || r > 108 {
		t.Fatal("below the range, the closest range still wins")
	}
	mid := pick(bones, 10)
	if math.Hypot(mid.X, mid.Z) <= math.Hypot(low.X, low.Z) {
		t.Fatal("higher level should move the circle outward in the ring")
	}
	if r := math.Hypot(pick([]string{"skeleton_warrior"}, 16).X, 0); r < 116 || r > 140 {
		t.Fatalf("warrior at level 16: r = %.1f, want the 116–140 ring", r)
	}
	// Chegar a um nível: a melhor área para o nível atual (sem chefes).
	if r := math.Hypot(pick(nil, 3).X, 0); r < 46 || r > 74 {
		t.Fatalf("level goal at 3: r = %.1f, want the minion ring", r)
	}
	if g := pick(nil, 25); g.Name != "Vale das Pedras Vivas" {
		t.Fatalf("level 25 should hunt in the valley, got %+v", g)
	}
	g29 := pick(nil, 29)
	if g29.Name != "Vale das Pedras Vivas" || g29.Z >= pick(nil, 22).Z {
		t.Fatalf("deeper levels go south in the valley: 29 → %+v", g29)
	}
	if math.Hypot(g29.X-0, g29.Z+270)+g29.Radius > 52+1e-9 {
		t.Fatalf("valley circle leaves the area: %+v", g29)
	}
	if boss := pick([]string{"bone_lord"}, 10); boss.X != 0 || boss.Z != -132 {
		t.Fatalf("boss place = %+v", boss)
	}
	if _, ok := HuntPlace(sp, []string{"dragao"}, 5, 0, 0); ok {
		t.Fatal("unknown monster must have no place")
	}
}

func TestHuntPlaceAvoidsRaidLair(t *testing.T) {
	pl, ok := HuntPlace(DefaultSpawns(), []string{"skeleton_mage"}, 16, 50, 50)
	if !ok {
		t.Fatal("no place")
	}
	if d := math.Hypot(pl.X-RaidLairX, pl.Z-RaidLairZ); d <= RaidLairRadius+pl.Radius {
		t.Fatalf("hunt circle (%.1f, %.1f) overlaps the raid lair (d = %.1f)", pl.X, pl.Z, d)
	}
}
