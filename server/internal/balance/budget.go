package balance

import (
	"fmt"
	"math"
	"sort"

	"erevia/server/internal/game"
)

// ---------------------------------------------------------------------------------------
// Orçamento de poder: o que o teste (budget_test.go) cobra
// ---------------------------------------------------------------------------------------

// Tolerâncias.
const (
	// Uma peça +0 pode passar o próprio orçamento (ExpectedPower do nível exigido e da
	// raridade) em até 20%.
	ItemTolerance = 0.20
	// Um +10 pode passar a peça +0 da raridade seguinte (o próximo degrau da mesma família:
	// comum → boa → rara → épica → lendária) em até 25%.
	EnchantTolerance = 0.25
	// O tempo para matar de cada classe (até o 19) ou caminho (do 20 em diante) pode ficar até
	// 15% acima ou abaixo da média da faixa. Conta o equipamento típico +0, em regime.
	ClassTolerance = 0.15
)

// Band é uma faixa de níveis.
type Band struct{ Lo, Hi int }

func (b Band) String() string { return fmt.Sprintf("%d–%d", b.Lo, b.Hi) }

// Bands: as faixas de caça (as regiões dos spawns; o vale dividido no 24/25, depois do 2º
// degrau dos caminhos). Do 30 ao 40 (tunnel#149): o Fundo do Vale e a Vigília Ossária
// (30–34) e os Confins Partidos (35–40).
var Bands = []Band{{1, 5}, {6, 12}, {13, 19}, {20, 24}, {25, 29}, {30, 34}, {35, game.MaxLevel}}

// Violation é uma regra do orçamento furada.
type Violation struct {
	Key    string  // identifica o caso na lista de exceções conhecidas
	Excess float64 // quanto passou (0,3 = 30% acima; negativo = abaixo)
	Detail string
}

// ItemViolations: peças +0 acima do orçamento.
func ItemViolations() []Violation {
	var out []Violation
	for _, d := range Gear() {
		budget := ItemBudget(d)
		if budget <= 0 {
			continue
		}
		if ex := ItemPower(d, 0)/budget - 1; ex > ItemTolerance {
			out = append(out, Violation{"item:" + d.ID, ex,
				fmt.Sprintf("%s (%s, nível %d): poder %s, orçamento %s", d.Name, gradeName(d.Grade), d.ReqLevel, num(ItemPower(d, 0), 1), num(budget, 1))})
		}
	}
	return out
}

// EnchantViolations: peças +10 que passam a próxima raridade +0 além da tolerância.
func EnchantViolations() []Violation {
	var out []Violation
	for _, fam := range Families() {
		ladder := Ladder(fam)
		for i := 0; i+1 < len(ladder); i++ {
			d, next := ladder[i], ladder[i+1]
			p10, p0 := ItemPower(d, game.MaxEnchant), ItemPower(next, 0)
			if ex := p10/p0 - 1; ex > EnchantTolerance {
				out = append(out, Violation{"enchant:" + d.ID, ex,
					fmt.Sprintf("+%d %s (%s) contra %s +0 (%s)", game.MaxEnchant, d.Name, num(p10, 1), next.Name, num(p0, 1))})
			}
		}
	}
	return out
}

// BandEntities: quem se compara na faixa (as classes antes do 20, os caminhos depois).
func BandEntities(b Band) []Path {
	if b.Lo >= game.SpecLevel {
		return Paths()
	}
	var out []Path
	for _, c := range Classes() {
		out = append(out, Path{Class: c})
	}
	return out
}

// BandTTK: tempo médio para matar (regime) de cada classe/caminho na faixa.
func BandTTK(b Band, enchant int) map[Path]float64 {
	out := map[Path]float64{}
	for _, p := range BandEntities(b) {
		sum := 0.0
		for l := b.Lo; l <= b.Hi; l++ {
			sum += RegimeTTK(p, l, enchant)
		}
		out[p] = sum / float64(b.Hi-b.Lo+1)
	}
	return out
}

// Mean: a média dos tempos da faixa.
func Mean(m map[Path]float64) float64 {
	s := 0.0
	for _, v := range m {
		s += v
	}
	return s / float64(len(m))
}

// ClassViolations: classes/caminhos fora da média da faixa em tempo para matar.
func ClassViolations() []Violation {
	var out []Violation
	for _, b := range Bands {
		ttk := BandTTK(b, 0)
		mean := Mean(ttk)
		for _, p := range BandEntities(b) {
			if ex := ttk[p]/mean - 1; math.Abs(ex) > ClassTolerance {
				out = append(out, Violation{"ttk:" + b.String() + ":" + p.Name(), ex,
					fmt.Sprintf("%s no %s: %s s para matar, média da faixa %s s", p.Name(), b, num(ttk[p], 1), num(mean, 1))})
			}
		}
	}
	return out
}

// AllViolations: as três listas, em ordem de chave.
func AllViolations() []Violation {
	out := append(append(ItemViolations(), EnchantViolations()...), ClassViolations()...)
	out = append(out, BestiaryViolations()...) // bestiary.go
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func gradeName(g game.Grade) string {
	switch g {
	case game.GradeCommon:
		return "comum"
	case game.GradeGood:
		return "boa"
	case game.GradeRare:
		return "rara"
	case game.GradeEpic:
		return "épica"
	case game.GradeLegendary:
		return "lendária"
	}
	return "?"
}
