package game

import "time"

// Ciclo dia/noite. A fase (0–1) vem do relógio de parede, então todos os jogadores veem a
// mesma hora e o ciclo não recomeça quando o servidor reinicia.
//
//	0.00        nascer do sol (06:00)
//	0.00–0.75   dia: o sol cruza o céu de leste a oeste (06:00–18:00)
//	0.75–1.00   noite (18:00–06:00), ~10 minutos reais
const (
	DayCycle   = 40 * time.Minute
	NightStart = 0.75

	NightMonsterAtk = 1.15 // monstros batem 15% mais forte à noite
	NightXP         = 1.25 // e dão 25% mais XP
	NightAggroRange = 11.0 // e enxergam mais longe (de dia: 8 m)
)

// DayPhase devolve a fase do ciclo (0 ≤ fase < 1) no instante t.
func DayPhase(t time.Time) float64 {
	cycle := DayCycle.Milliseconds()
	ms := t.UnixMilli() % cycle
	if ms < 0 {
		ms += cycle
	}
	return float64(ms) / float64(cycle)
}

// IsNight informa se a fase é noite.
func IsNight(phase float64) bool { return phase >= NightStart }
