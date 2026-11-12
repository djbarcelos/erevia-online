package game

import (
	"testing"
	"time"
)

func TestDayPhaseWrapsAndMarksNight(t *testing.T) {
	start := time.UnixMilli(DayCycle.Milliseconds() * 1000) // início exato de um ciclo
	cases := []struct {
		offset time.Duration
		phase  float64
		night  bool
	}{
		{0, 0, false},
		{10 * time.Minute, 0.25, false},
		{29*time.Minute + 59*time.Second, 0.7496, false},
		{30 * time.Minute, 0.75, true},
		{39 * time.Minute, 0.975, true},
		{40 * time.Minute, 0, false}, // volta ao começo
	}
	for _, c := range cases {
		p := DayPhase(start.Add(c.offset))
		if p < c.phase-0.001 || p > c.phase+0.001 {
			t.Errorf("%v: fase %.4f, esperava %.4f", c.offset, p, c.phase)
		}
		if IsNight(p) != c.night {
			t.Errorf("%v: noite=%v, esperava %v", c.offset, IsNight(p), c.night)
		}
	}
	if p := DayPhase(time.UnixMilli(-1)); p < 0 || p >= 1 {
		t.Errorf("fase fora de [0,1) antes de 1970: %v", p)
	}
}
