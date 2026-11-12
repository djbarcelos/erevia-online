package game

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestClassStatsGrowWithLevel(t *testing.T) {
	for id := range Classes {
		l1, l10 := ClassStats(id, 1), ClassStats(id, 10)
		if l10.MaxHP <= l1.MaxHP || l10.PDef <= l1.PDef {
			t.Errorf("%s should grow: %+v → %+v", id, l1, l10)
		}
	}
	if ClassStats("mage", 1).Range <= ClassStats("knight", 1).Range {
		t.Error("mage should have longer range")
	}
	if !ClassStats("mage", 1).Magic {
		t.Error("mage basic attack should be magic")
	}
}

func TestEveryClassHasTwoSkills(t *testing.T) {
	for id := range Classes {
		s := ClassSkills(id)
		if len(s) != 2 || s[0] == nil || s[1] == nil || s[0].Class != id || s[0].Key != 1 || s[1].Key != 2 {
			t.Errorf("%s skills wrong: %+v", id, s)
		}
	}
}

func TestRollHit(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	var misses, crits int
	var sum float64
	n := 20000
	for i := 0; i < n; i++ {
		h := RollHit(rng, 100, 100, 1, 0.2)
		switch {
		case h.Miss:
			misses++
		case h.Crit:
			crits++
		default:
			sum += h.Damage
			if h.Damage < 44 || h.Damage > 56 {
				t.Fatalf("damage out of range: %v", h.Damage)
			}
		}
	}
	if r := float64(misses) / float64(n); math.Abs(r-MissChance) > 0.01 {
		t.Errorf("miss rate %.3f", r)
	}
	if r := float64(crits) / float64(n-misses); math.Abs(r-0.2) > 0.02 {
		t.Errorf("crit rate %.3f", r)
	}
	if h := RollHit(rng, 1, 10000, 1, 0); !h.Miss && h.Damage < 1 {
		t.Error("damage should be at least 1")
	}
}

func TestXPCurve(t *testing.T) {
	if XPLevelStart(1) != 0 || XPLevelStart(2) != XPToNext(1) {
		t.Fatal("level start wrong")
	}
	if XPToNext(MaxLevel) != 0 {
		t.Fatal("no XP needed past max level")
	}
	for l := 1; l <= MaxLevel; l++ {
		if got := LevelForXP(XPLevelStart(l)); got != l {
			t.Errorf("LevelForXP(start of %d) = %d", l, got)
		}
	}
	if LevelForXP(math.MaxInt64/2) != MaxLevel {
		t.Error("huge xp should cap at max level")
	}
	// Ritmo: matar monstros do próprio nível sobe de nível com ~5–40 abates.
	for l := 1; l < MaxLevel; l++ {
		kills := float64(XPToNext(l)) / float64(MonsterXP(Monsters["skeleton_warrior"], l, l))
		if kills < 4 || kills > 45 {
			t.Errorf("level %d needs %.0f kills", l, kills)
		}
	}
}

func TestMonsterXPPenalty(t *testing.T) {
	m := Monsters["skeleton_minion"]
	full := MonsterXP(m, 3, 3)
	if MonsterXP(m, 3, 8) != full {
		t.Error("5 levels above should still give full XP")
	}
	if MonsterXP(m, 3, 15) >= full {
		t.Error("much stronger player should get less XP")
	}
	if MonsterXP(m, 1, 20) < 1 {
		t.Error("XP should never be zero")
	}
}

func TestDeathNeverDropsLevel(t *testing.T) {
	start := XPLevelStart(5)
	if got := XPAfterDeath(start+1, 5); got != start {
		t.Fatalf("should clamp to level start: %d", got)
	}
	mid := start + XPToNext(5)/2
	want := mid - int64(math.Round(float64(XPToNext(5))*DeathXPLoss))
	if got := XPAfterDeath(mid, 5); got != want {
		t.Fatalf("XPAfterDeath = %d, want %d", got, want)
	}
}

func TestExpandSpawnsDeterministicAndInRing(t *testing.T) {
	a := ExpandSpawns(DefaultSpawns(), rand.New(rand.NewPCG(7, 7)))
	b := ExpandSpawns(DefaultSpawns(), rand.New(rand.NewPCG(7, 7)))
	if len(a) != len(b) || len(a) < 50 {
		t.Fatalf("spawns: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("same seed must give same spawns")
		}
		r := math.Hypot(a[i].X, a[i].Z)
		if r < 44 || r > 145 { // fora da cidade (raio 40) e dentro do mapa (±150)
			t.Errorf("spawn outside hunting area: r=%.1f", r)
		}
		if a[i].Level < 1 || a[i].Level > 22 {
			t.Errorf("bad level %d", a[i].Level)
		}
	}
}
