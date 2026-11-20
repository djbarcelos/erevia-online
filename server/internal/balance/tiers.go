package balance

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"erevia/server/internal/game"
)

// ---------------------------------------------------------------------------------------
// Graus de equipamento (tunnel#153) no relatório
// ---------------------------------------------------------------------------------------

// TierSwaps: os níveis em que o equipamento típico troca de grau (o D no 10, o C no 20, o
// B no 30), com o nível de antes e os dois de depois.
var TierSwaps = []int{game.TierD.MinLevel(), game.TierC.MinLevel(), game.TierB.MinLevel()}

// NewGearByLevel: o equipamento que passa a existir em cada nível do 10 ao máximo (só o que
// pede exatamente aquele nível), em ordem de id.
func NewGearByLevel() map[int][]*game.ItemDef {
	out := map[int][]*game.ItemDef{}
	for _, d := range Gear() {
		if d.ReqLevel >= game.TierD.MinLevel() && !d.RepOnly {
			out[d.ReqLevel] = append(out[d.ReqLevel], d)
		}
	}
	return out
}

// writeTiers: a seção dos graus no relatório.
func writeTiers(w io.Writer) {
	pf := func(format string, a ...any) { fmt.Fprintf(w, format, a...) }
	pf("\n## Graus de equipamento\n\n")
	pf("Grau pelo nível exigido: sem grau 1–9, D %d–%d, C %d–%d, B %d–%d. Abaixo do nível mínimo do grau a peça não se veste; do mínimo até o nível da peça ela vale com a penalidade de nível (%s, uma vez).\n\n",
		game.TierD.MinLevel(), game.TierC.MinLevel()-1, game.TierC.MinLevel(), game.TierB.MinLevel()-1, game.TierB.MinLevel(), game.MaxLevel,
		penaltyText())
	pf("### Equipamento novo por nível\n\n| Nível | Grau | Peças novas |\n|---:|---|---|\n")
	by := NewGearByLevel()
	var levels []int
	for l := range by {
		levels = append(levels, l)
	}
	sort.Ints(levels)
	for _, l := range levels {
		groups := map[string]int{}
		var order []string
		for _, d := range by[l] {
			g := gearGroup(d)
			if groups[g] == 0 {
				order = append(order, g)
			}
			groups[g]++
		}
		var parts []string
		for _, g := range order {
			parts = append(parts, fmt.Sprintf("%s (%d)", g, groups[g]))
		}
		pf("| %d | %s | %s |\n", l, game.TierFor(l).Letter(), strings.Join(parts, " · "))
	}

	pf("\n### Conjuntos\n\nCinco peças (peito, cabeça, pernas, mãos e pés), comum e Exaltada (épica); a Exaltada conta como a comum. **Poder**: o conjunto inteiro +0 com os dois bônus, contra a soma dos orçamentos das peças (tolerância +%s).\n\n",
		pct(ItemTolerance)[1:])
	pf("| Conjunto | Grau | Peso | Nível | 2 peças | 4 peças | Poder (comum) | Poder (Exaltada) |\n|---|---|---|---:|---|---|---:|---:|\n")
	for _, s := range game.ItemSetList {
		pc, bc := SetPower(s, game.GradeCommon)
		pe, be := SetPower(s, game.GradeEpic)
		lvl := game.Items[s.Pieces[0]].ReqLevel
		pf("| %s | %s | %s | %d | %s | %s | %s / %s (%s) | %s / %s (%s) |\n", s.Name, s.Tier.Letter(), s.Weight.Name(), lvl,
			statText(s.Bonuses[0].Bonus), statText(s.Bonuses[1].Bonus), num(pc, 1), num(bc, 1), pct(pc/bc-1), num(pe, 1), num(be, 1), pct(pe/be-1))
	}

	pf("\n### O que cada troca de equipamento rende\n\nNo nível em que o equipamento típico muda: o tempo para matar com o equipamento novo contra o do nível de antes (o mesmo personagem, o mesmo monstro). É o salto de poder de cada troca, sem a mudança de caça no meio.\n\n")
	swaps := GearSwapLevels()
	pf("| Caminho |")
	for _, l := range swaps {
		pf(" %d |", l)
	}
	pf("\n|---|")
	for range swaps {
		pf("---:|")
	}
	pf("\n")
	for _, p := range Paths() {
		pf("| %s |", p.Name())
		for _, l := range swaps {
			pf(" %s |", pct(GearSwapGain(p, l)))
		}
		pf("\n")
	}

	pf("\n### Tempo para matar na troca de grau (regime, +0)\n\nO nível de antes do grau e os dois primeiros dele, com o equipamento típico de cada um. No 20 e no 30 a caça também muda (os golens do Vale; as elites do Fundo do Vale): ali o tempo sobe por causa do monstro, não do equipamento (a tabela de cima separa os dois).\n\n")
	pf("| Caminho |")
	for _, l := range TierSwaps {
		pf(" %d | %d | %d |", l-1, l, l+1)
	}
	pf("\n|---|")
	for range TierSwaps {
		pf("---:|---:|---:|")
	}
	pf("\n")
	for _, p := range Paths() {
		pf("| %s |", p.Name())
		for _, l := range TierSwaps {
			for _, x := range []int{l - 1, l, l + 1} {
				pf(" %s |", num(RegimeTTK(p, x, 0), 1))
			}
		}
		pf("\n")
	}
}

// GearSwapLevels: os níveis do 10 ao máximo em que o equipamento típico de algum caminho
// muda (entrou peça nova).
func GearSwapLevels() []int {
	var out []int
	for l := game.TierD.MinLevel(); l <= game.MaxLevel; l++ {
		for _, c := range Classes() {
			if !samePieces(TypicalGear(c, l, 0), TypicalGear(c, l-1, 0)) {
				out = append(out, l)
				break
			}
		}
	}
	return out
}

func samePieces(a, b []Piece) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// GearSwapGain: no nível, o tempo para matar com o equipamento típico do nível contra o do
// nível de antes (o resto do personagem igual): −0,10 = a troca mata 10% mais rápido.
func GearSwapGain(p Path, level int) float64 {
	now := NewBuild(p, level, 0)
	old := NewBuild(p, level, 0)
	old.Gear = TypicalGearFor(p.Class, old.Path.Spec, level-1, 0)
	old.Stats = game.CharacterStats(p.Class, old.Path.Spec, level, old.Attrs, GearBonus(level, old.Gear), old.Ranks, weaponType(old.Gear))
	return buildRegimeTTK(now, level)/buildRegimeTTK(old, level) - 1
}

// gearGroup: como a peça aparece na tabela por nível ("armas comuns", "conjunto Leve D"…).
func gearGroup(d *game.ItemDef) string {
	switch {
	case d.Bridge:
		return "ponte " + gradeName(d.Grade)
	case d.Set != "":
		return fmt.Sprintf("%s (%s)", game.ItemSets[d.Set].Name, gradeName(d.Grade))
	case d.Kind == game.KindWeapon:
		return "armas " + gradePlural(d.Grade)
	case d.Graded:
		return "escudo, capa e joias " + gradePlural(d.Grade)
	}
	return kindName(d.Kind) + " (" + gradeName(d.Grade) + ")"
}

func gradePlural(g game.Grade) string {
	switch g {
	case game.GradeCommon:
		return "comuns"
	case game.GradeEpic:
		return "épicas"
	case game.GradeLegendary:
		return "lendárias"
	}
	return gradeName(g) + "s"
}

// statText: "+4 def. física, +30 vida" (o bônus de conjunto na tabela).
func statText(b game.Stats) string {
	var parts []string
	add := func(v float64, label string, percent bool) {
		if v == 0 {
			return
		}
		if percent {
			parts = append(parts, fmt.Sprintf("+%s%% %s", num(v*100, 0), label))
			return
		}
		parts = append(parts, fmt.Sprintf("+%s %s", num(v, numPrec(v)), label))
	}
	add(b.PAtk, "atq. físico", false)
	add(b.MAtk, "atq. mágico", false)
	add(b.PDef, "def. física", false)
	add(b.MDef, "def. mágica", false)
	add(b.MaxHP, "vida", false)
	add(b.MaxMP, "mana", false)
	add(b.Crit, "crítico", true)
	add(b.CritDamage, "dano crítico", true)
	add(b.Accuracy, "precisão", false)
	add(b.Evasion, "esquiva", false)
	add(b.AtkSpeed, "vel. de ataque", true)
	add(b.CastSpeed, "conjuração", true)
	add(b.HPRegen, "vida/s", false)
	add(b.MPRegen, "mana/s", false)
	return strings.Join(parts, ", ")
}

func numPrec(v float64) int {
	if v != float64(int(v)) {
		return 1
	}
	return 0
}

func penaltyText() string {
	u := game.UnderLevelPenalty
	return fmt.Sprintf("−%s de precisão e −%s%% de velocidade de ataque e de conjuração", num(-u.Accuracy, 0), num(-u.AtkSpeed*100, 0))
}
