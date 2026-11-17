package balance

import (
	"fmt"
	"math"

	"erevia/server/internal/game"
)

// ---------------------------------------------------------------------------------------
// Bestiário no orçamento de poder
// ---------------------------------------------------------------------------------------
//
// O último marco do Bestiário dá um bônus de dano contra aquele tipo de monstro. O veterano
// do caso pior tem o bônus no teto (game.BestiaryDamageCap) contra todos os monstros da
// região onde caça. O bônus é horizontal: ele pode deixar a caça um pouco mais rápida, mas
// não pode valer um degrau de equipamento (as regras em BestiaryViolations). Se furar, o que muda é o bônus (game.BestiaryDamageBonus/Cap), não
// a tolerância.

// BestiaryTolerance: o quanto o bônus inteiro pode somar ao dano por segundo da rotação.
const BestiaryTolerance = 0.03

// BestiaryVeteranBonus: o bônus do veterano no simulador (o mesmo da zona com 500 abates).
func BestiaryVeteranBonus() float64 {
	best := 0.0
	for _, e := range game.BestiaryCatalog() {
		best = math.Max(best, game.BestiaryBonus(e.Monster, game.BestiaryMarkBonus))
	}
	return math.Min(best, game.BestiaryDamageCap)
}

// BestiaryDPSGain: quanto o bônus soma ao dano por segundo da rotação numa luta longa
// (TimeToDie, sem a sobra do último golpe), na média dos níveis da faixa e dos monstros da
// região (0,01 = 1%).
func BestiaryDPSGain(b Band, p Path, bonus float64) float64 {
	var base, with float64
	for l := b.Lo; l <= b.Hi; l++ {
		b0, b1 := NewBuild(p, l, 0), NewBuild(p, l, 0)
		b1.Bestiary = bonus
		for _, hm := range HuntFor(l).Monsters {
			_, d0 := TimeToDie(b0, hm.Def, l, false, true)
			_, d1 := TimeToDie(b1, hm.Def, l, false, true)
			base += float64(hm.Count) * d0
			with += float64(hm.Count) * d1
		}
	}
	if base <= 0 {
		return 0
	}
	return with/base - 1
}

// BestiaryPower: o bônus do veterano em pontos de poder (o ataque a mais do bônus no teto,
// com o ataque do personagem típico) e a folga que o orçamento dá à arma típica do nível
// (ItemTolerance × orçamento da arma).
func BestiaryPower(p Path, level int, bonus float64) (power, slack float64) {
	b := NewBuild(p, level, 0)
	power = wAtk * bonus * math.Max(b.Stats.PAtk, b.Stats.MAtk)
	for _, pc := range b.Gear {
		if pc.Item.Kind == game.KindWeapon {
			slack = ItemTolerance * ItemBudget(pc.Item)
		}
	}
	return power, slack
}

// BestiaryViolations: o veterano do Bestiário (bônus no teto contra tudo o que caça) no
// orçamento de poder. Duas regras, por classe/caminho e faixa:
//
//   - poder: o bônus, em pontos de poder (o ataque a mais), não passa da folga que o
//     orçamento dá à arma típica do nível (ItemTolerance): vale menos que o que uma arma pode
//     passar do próprio orçamento, então não substitui um degrau de equipamento;
//   - dano: no simulador, o bônus soma no máximo BestiaryTolerance ao dano por segundo da
//     rotação (pega um bônus que vaze para veneno, roubo de vida ou outra conta).
//
// O equilíbrio entre as classes (ClassViolations) não é recalculado com o bônus: ele vale
// igual para todas, e o tempo para matar do simulador anda em degraus (sem o sorteio do dano,
// um golpe a menos por monstro muda a média da faixa de uma vez); medido em novembro de 2026,
// até um bônus de 0,5% já move as exceções conhecidas mais que a folga de 2 pontos.
func BestiaryViolations() []Violation {
	bonus := BestiaryVeteranBonus()
	if bonus <= 0 {
		return nil
	}
	var out []Violation
	for _, b := range Bands {
		for _, p := range BandEntities(b) {
			worst, at, wp, ws := 0.0, 0, 0.0, 0.0
			for l := b.Lo; l <= b.Hi; l++ {
				pw, sl := BestiaryPower(p, l, bonus)
				if sl > 0 && pw/sl > worst {
					worst, at, wp, ws = pw/sl, l, pw, sl
				}
			}
			if worst > 1 {
				out = append(out, Violation{"bestiary-power:" + b.String() + ":" + p.Name(), worst - 1,
					fmt.Sprintf("%s no nível %d: o Bestiário (+%.0f%% de dano) vale %s pontos de poder, a folga da arma é %s",
						p.Name(), at, bonus*100, num(wp, 1), num(ws, 1))})
			}
			if g := BestiaryDPSGain(b, p, bonus); g > BestiaryTolerance {
				out = append(out, Violation{"bestiary-dps:" + b.String() + ":" + p.Name(), g,
					fmt.Sprintf("%s no %s: o Bestiário (+%.0f%% de dano) soma %.1f%% ao dano por segundo (máximo %.0f%%)",
						p.Name(), b, bonus*100, g*100, BestiaryTolerance*100)})
			}
		}
	}
	return out
}
