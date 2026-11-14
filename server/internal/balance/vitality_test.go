package balance

import (
	"math"
	"testing"
)

// A meta do tunnel#150: quem joga 1 h por dia, com a vitalidade, leva no máximo ~60% do
// tempo de jogo de quem joga sem parar (e não menos que a metade: só a XP de monstro dobra).
func TestVitalityOneHourPerDay(t *testing.T) {
	nonstop := DailyPlay(DailyPlan{HoursPerDay: 24})
	if math.Abs(nonstop.PlayHours-148) > 2 {
		t.Fatalf("sem parar: %.1f h, a tabela do docs/progressao.md dá ~148 h", nonstop.PlayHours)
	}
	if plain := DailyPlay(DailyPlan{HoursPerDay: 1}); math.Abs(plain.PlayHours-nonstop.PlayHours) > 0.01 {
		t.Fatalf("sem vitalidade, 1 h por dia leva o mesmo tempo de jogo: %.1f h", plain.PlayHours)
	}
	daily := DailyPlay(DailyPlan{HoursPerDay: 1, Vitality: true})
	ratio := daily.PlayHours / nonstop.PlayHours
	if ratio > 0.60 || ratio < 0.5 {
		t.Fatalf("1 h por dia com vitalidade: %.1f h = %.0f%% de quem joga sem parar (meta: até ~60%%)", daily.PlayHours, 100*ratio)
	}
	// Mais horas por dia, menos a vitalidade ajuda; o 2º personagem chega antes.
	if four := DailyPlay(DailyPlan{HoursPerDay: 4, Vitality: true}); four.PlayHours <= daily.PlayHours {
		t.Fatalf("4 h por dia (%.1f h) não pode render mais que 1 h (%.1f h)", four.PlayHours, daily.PlayHours)
	}
	if alt := DailyPlay(DailyPlan{HoursPerDay: 24, Catchup: true}); alt.PlayHours >= nonstop.PlayHours {
		t.Fatalf("recuperação: %.1f h", alt.PlayHours)
	}
}
