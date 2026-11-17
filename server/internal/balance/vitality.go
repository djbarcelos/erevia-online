package balance

import (
	"fmt"
	"io"
	"math"
	"strings"

	"erevia/server/internal/game"
)

// ---------------------------------------------------------------------------------------
// Vitalidade (tunnel#150): quanto tempo de jogo quem joga pouco leva até o máximo
// ---------------------------------------------------------------------------------------
//
// Usa a progressão teórica do docs/progressao.md (game.TheoreticalProgression: abates do
// próprio nível, game.SecondsPerKill por abate, ~30% da XP de cada faixa em missões) e
// simula dia a dia: joga HoursPerDay horas caçando e passa o resto fora do jogo, enchendo a
// vitalidade (game.VitalityOfflinePerHour do teto por hora). Enquanto há vitalidade, a XP
// de monstro sai em dobro e o bônus sai do estoque; a das missões não muda. A meta da task:
// quem joga 1 h por dia leva no máximo ~60% do tempo de jogo de quem joga sem parar.

// DailyPlan: como o personagem joga.
type DailyPlan struct {
	HoursPerDay float64 // horas de caça por dia (24 = sem parar, sem vitalidade)
	Vitality    bool    // enche a vitalidade nas horas fora do jogo
	Catchup     bool    // 2º personagem: a conta já tem alguém no máximo (+50% até o 35)
}

// DailyResult: tempo de jogo e dias corridos até o nível máximo.
type DailyResult struct {
	PlayHours float64
	Days      int
	// VitalityShare: fração da XP total que veio da vitalidade.
	VitalityShare float64
}

// DailyPlay simula o plano do nível 1 ao máximo.
func DailyPlay(plan DailyPlan) DailyResult {
	rows := game.TheoreticalProgression(game.SecondsPerKill)
	level, prog, vit := 1, 0.0, 0.0
	var res DailyResult
	var total, fromVit float64
	catchupOn := func() bool { return plan.Catchup && game.CatchupActive(level, game.MaxLevel) }
	need := func() float64 { // XP de monstro que o nível pede (o resto vem das missões)
		r := rows[level-1]
		quest := float64(r.QuestXP)
		if catchupOn() {
			quest = math.Min(float64(r.XPToNext), quest*game.CatchupMult)
		}
		return float64(r.XPToNext) - quest
	}
	for level < game.MaxLevel && res.Days < 100_000 {
		left := plan.HoursPerDay
		for left > 1e-9 && level < game.MaxLevel {
			r := rows[level-1]
			rate := r.XPPerKill * 3600 / game.SecondsPerKill // XP de monstro por hora, sem extras
			cu := 0.0
			if catchupOn() {
				cu = rate * (game.CatchupMult - 1)
			}
			bonus := 0.0
			if vit > 0 {
				bonus = rate * (game.VitalityMult - 1)
			}
			gain := rate + cu + bonus
			t := math.Min(left, (need()-prog)/gain)
			if bonus > 0 {
				t = math.Min(t, vit/bonus)
			}
			t = math.Max(t, 0)
			prog += gain * t
			vit = math.Max(0, vit-bonus*t)
			fromVit += bonus * t
			left -= t
			res.PlayHours += t
			if prog >= need()-1e-6 {
				total += float64(r.XPToNext)
				level++
				prog = 0
				vit = math.Min(vit, float64(game.VitalityCap(level)))
			}
		}
		res.Days++
		if plan.Vitality && level < game.MaxLevel {
			vit = float64(game.VitalityFill(int64(vit), level, 24-plan.HoursPerDay, game.VitalityOfflinePerHour))
		}
	}
	if total > 0 {
		res.VitalityShare = fromVit / total
	}
	return res
}

// WriteVitalityMarkdown: a tabela do PR (go run ./cmd/balance -vitalidade).
func WriteVitalityMarkdown(w io.Writer) {
	nonstop := DailyPlay(DailyPlan{HoursPerDay: 24})
	dec := func(v float64) string { return strings.Replace(fmt.Sprintf("%.1f", v), ".", ",", 1) }
	fmt.Fprintf(w, "Progressão teórica (%s s por abate), do 1 ao %d. Vitalidade: +%s%% do teto por hora fora do jogo; teto = XPToNext(nível).\n\n",
		dec(game.SecondsPerKill), game.MaxLevel, dec(game.VitalityOfflinePerHour*100))
	fmt.Fprintln(w, "| Plano | Horas de jogo | Dias corridos | % do tempo de quem joga sem parar | XP da vitalidade |")
	fmt.Fprintln(w, "|---|---:|---:|---:|---:|")
	rows := []struct {
		name string
		plan DailyPlan
	}{
		{"Sem parar (sem vitalidade)", DailyPlan{HoursPerDay: 24}},
		{"1 h por dia, sem vitalidade", DailyPlan{HoursPerDay: 1}},
		{"1 h por dia, com vitalidade", DailyPlan{HoursPerDay: 1, Vitality: true}},
		{"2 h por dia, com vitalidade", DailyPlan{HoursPerDay: 2, Vitality: true}},
		{"4 h por dia, com vitalidade", DailyPlan{HoursPerDay: 4, Vitality: true}},
		{"2º personagem, sem parar (recuperação)", DailyPlan{HoursPerDay: 24, Catchup: true}},
		{"2º personagem, 1 h por dia, com vitalidade e recuperação", DailyPlan{HoursPerDay: 1, Vitality: true, Catchup: true}},
	}
	for _, r := range rows {
		got := DailyPlay(r.plan)
		fmt.Fprintf(w, "| %s | %s | %d | %s%% | %s%% |\n", r.name, dec(got.PlayHours), got.Days,
			dec(100*got.PlayHours/nonstop.PlayHours), dec(100*got.VitalityShare))
	}
}
