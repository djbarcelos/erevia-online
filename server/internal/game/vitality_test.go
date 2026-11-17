package game

import (
	"math"
	"testing"
	"time"
)

func TestVitalityCapIsOneLevel(t *testing.T) {
	for _, l := range []int{1, 10, 20, 39} {
		if VitalityCap(l) != XPToNext(l) {
			t.Fatalf("teto no %d = %d, XPToNext = %d", l, VitalityCap(l), XPToNext(l))
		}
	}
	if VitalityCap(MaxLevel) != MasteryPointXP {
		t.Fatalf("no máximo o teto é uma barra de Maestria: %d", VitalityCap(MaxLevel))
	}
	if ClampVitality(-5, 10) != 0 || ClampVitality(XPToNext(10)*3, 10) != XPToNext(10) {
		t.Fatal("ClampVitality fora de [0, teto]")
	}
}

// Enche 1,5% do teto por hora fora do jogo: cheia em ~67 h (~3 dias), nunca passa do teto.
func TestVitalityOfflineFill(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	c := VitalityCap(30)
	cases := []struct {
		hours float64
		want  float64 // fração do teto
	}{{0, 0}, {1, 0.015}, {10, 0.15}, {24, 0.36}, {48, 0.72}, {66, 0.99}, {67, 1}, {72, 1}, {24 * 365, 1}}
	for _, tc := range cases {
		got := VitalityOffline(0, 30, now.Add(-time.Duration(tc.hours*float64(time.Hour))), now)
		if want := int64(math.Min(float64(c), float64(c)*tc.want)); math.Abs(float64(got-want)) > 1 {
			t.Fatalf("%v h fora: %d, esperava %d", tc.hours, got, want)
		}
	}
	// Quem já tinha um pouco soma; relógio voltando ou sem saída registrada não enchem.
	if got := VitalityOffline(c/2, 30, now.Add(-10*time.Hour), now); got != c/2+int64(float64(c)*0.15) {
		t.Fatalf("soma ao que havia: %d", got)
	}
	if VitalityOffline(100, 30, now.Add(time.Hour), now) != 100 || VitalityOffline(100, 30, time.Time{}, now) != 100 {
		t.Fatal("sem tempo fora, o estoque fica como está")
	}
	if hours := 1 / VitalityOfflinePerHour; hours < 60 || hours > 75 {
		t.Fatalf("cheia em %.0f h: a meta é ~3 dias", hours)
	}
}

// A recuperação entra primeiro e o teto ×3 corta a vitalidade, que então gasta menos.
func TestMonsterXPExtrasCap(t *testing.T) {
	const base = 1000
	cases := []struct {
		name    string
		mult    float64
		catchup bool
		stock   int64
		cu, vit int64
	}{
		{"só vitalidade", 1, false, 1 << 40, 0, 1000},
		{"vitalidade no fim", 1, false, 300, 0, 300},
		{"sem vitalidade", 1, false, 0, 0, 0},
		{"evento + vitalidade = ×3", 2, false, 1 << 40, 0, 1000},
		{"recuperação + vitalidade", 1, true, 1 << 40, 500, 1000},
		{"evento + recuperação + vitalidade, teto ×3", 2, true, 1 << 40, 500, 500},
		{"noite + evento (×2,5) + recuperação: só cabe ela", 2.5, true, 1 << 40, 500, 0},
		{"noite + evento + vitalidade", 2.5, false, 1 << 40, 0, 500},
	}
	for _, tc := range cases {
		xp := int64(base * tc.mult)
		cu, vit := MonsterXPExtras(xp, tc.mult, tc.catchup, tc.stock)
		if cu != tc.cu || vit != tc.vit {
			t.Fatalf("%s: recuperação %d, vitalidade %d (esperava %d, %d)", tc.name, cu, vit, tc.cu, tc.vit)
		}
		if total := float64(xp + cu + vit); total > MonsterXPMultCap*base+0.5 {
			t.Fatalf("%s: %v passa do teto ×%v", tc.name, total, MonsterXPMultCap)
		}
	}
	if cu, vit := MonsterXPExtras(0, 1, true, 100); cu != 0 || vit != 0 {
		t.Fatal("abate sem XP não ganha extra")
	}
}

func TestCatchupWindow(t *testing.T) {
	for _, tc := range []struct {
		level, top int
		want       bool
	}{{1, 40, true}, {34, 40, true}, {35, 40, false}, {40, 40, false}, {10, 39, false}, {10, 0, false}} {
		if got := CatchupActive(tc.level, tc.top); got != tc.want {
			t.Fatalf("nível %d com a conta no %d: %v", tc.level, tc.top, got)
		}
	}
	if QuestXPCatchup(1000, true) != 500 || QuestXPCatchup(1000, false) != 0 {
		t.Fatal("recuperação na missão: +50%")
	}
}
