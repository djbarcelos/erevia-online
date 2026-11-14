package balance

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"erevia/server/internal/game"
)

// ---------------------------------------------------------------------------------------
// Saída: tabelas em markdown ou CSV
// ---------------------------------------------------------------------------------------

// Report são todas as linhas: por caminho, os níveis 1–30, com +0 e +3.
type Report struct {
	Night bool
	Rows  map[int]map[Path][]Row // encantamento → caminho → níveis 1..30
}

// Enchants: os dois equipamentos típicos (sem encantamento e no máximo seguro).
var Enchants = []int{0, game.EnchantSafe}

// BuildReport calcula o relatório (determinístico).
func BuildReport(night bool) *Report {
	r := &Report{Night: night, Rows: map[int]map[Path][]Row{}}
	for _, e := range Enchants {
		r.Rows[e] = map[Path][]Row{}
		// Antes do 20 os dois caminhos da classe são o mesmo personagem: calcula uma vez.
		base := map[string][]Row{}
		for _, c := range Classes() {
			for l := 1; l < game.SpecLevel; l++ {
				base[c] = append(base[c], Simulate(Path{Class: c}, l, e, night))
			}
		}
		for _, p := range Paths() {
			rows := append([]Row{}, base[p.Class]...)
			for l := game.SpecLevel; l <= game.MaxLevel; l++ {
				rows = append(rows, Simulate(p, l, e, night))
			}
			r.Rows[e][p] = rows
		}
	}
	return r
}

// num: número com vírgula decimal.
func num(v float64, prec int) string {
	if math.IsInf(v, 1) {
		return "∞"
	}
	if math.IsNaN(v) {
		return "—"
	}
	return strings.Replace(strconv.FormatFloat(v, 'f', prec, 64), ".", ",", 1)
}

// thousands: inteiro com ponto de milhar.
func thousands(v float64) string {
	if math.IsInf(v, 1) {
		return "∞"
	}
	s := strconv.FormatInt(int64(math.Round(v)), 10)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// dur: duração legível (s, min, h).
func dur(s float64) string {
	switch {
	case math.IsInf(s, 1):
		return "∞"
	case s < 60:
		return num(s, 0) + " s"
	case s < 3600:
		return fmt.Sprintf("%d min %02d s", int(s)/60, int(s)%60)
	}
	m := int(math.Round(s / 60))
	return fmt.Sprintf("%d h %02d min", m/60, m%60)
}

func pct(v float64) string {
	sign := "+"
	if v < 0 {
		sign = "−"
	}
	return sign + num(math.Abs(v)*100, 0) + "%"
}

// Assumptions: as suposições do modelo, com os valores das constantes.
func Assumptions(night bool) []string {
	period := "de dia"
	if night {
		period = fmt.Sprintf("de noite (monstros ×%s de ataque, ×%s de XP)", num(game.NightMonsterAtk, 2), num(game.NightXP, 2))
	}
	return []string{
		"Fórmulas do pacote `game`: atributos (`CharacterStats`: classe, pontos, caminho, equipamento, passivas e regeneração), dano médio do golpe (`ExpectedHit`: acerto × ataque × 100/(100+defesa) × crítico), custo e recarga das skills (`SkillMPCost`, `SkillCooldownFor`), XP (`MonsterXP`, `XPToNext`) e monstros reais (`MonsterStats`, `DefaultSpawns`). Média por fórmula: sem sorteio.",
		fmt.Sprintf("Caça sozinho, %s, sem XP em dobro. A luta anda no passo do tick da zona (%s s).", period, num(tickDT, 2)),
		"Pontos de atributo: 2 no atributo de dano e 1 no de apoio a cada nível (cavaleiro Força/Constituição, bárbaro Força/Constituição, mago Inteligência/Sabedoria, ladino Destreza/Força). O caminho escolhido no 20 soma o bônus dele.",
		"Árvore: dano primeiro (ordem em `build.go`), o 5º degrau e as 3 skills do caminho do 20 em diante, o resto em defesa. Nenhuma pontuação fora das regras (`ValidateRanks`).",
		fmt.Sprintf("Equipamento típico: em cada encaixe a melhor peça comum ou boa que a classe usa no nível (pelo poder do item), tudo +0 ou tudo +%d (o encantamento seguro). Raras, épicas e lendárias ficam fora (são drop ou criação).", game.EnchantSafe),
		fmt.Sprintf("Rotação como a auto-caça do cliente: uma skill a cada %s s, buffs que faltam primeiro, depois enfraquecer/marcar o alvo, depois a skill de mais dano pronta e com mana; o ataque básico corre junto. Poção de vida abaixo de %s e de mana abaixo de %s (as menores até o nível %d, as maiores depois), sem limite de estoque. Fuga e utilidade (Piscar, Passo Sombrio, Sumir, Escudo Arcano, Imortal, Provocar) ficam de fora.", num(skillGap, 0), pct(potionHPBelow)[1:], pct(potionMPBelow)[1:], largePotionFrom-1),
		"Monstro: o do nível do jogador, na região onde o nível caça (o grupo de `DefaultSpawns` que cobre o nível; empate, a região mais nova: do 20 em diante, o Vale das Pedras Vivas). Monstros que dividem a região entram na proporção da quantidade. A luta abre no alcance do ataque básico; o monstro anda até o dele (quem bate de longe ganha esse tempo).",
		fmt.Sprintf("**Matar**: tempo médio em regime (as recargas seguem de uma luta para a outra); **abertura** é a primeira luta, com tudo pronto. **DPS**: dano por segundo da rotação numa luta longa contra o mesmo monstro. **Morrer**: lutando sem parar contra o monstro (que não cai), com regeneração de combate, com e sem poções; ∞ = aguenta mais de %d min (ou volta à vida cheia no meio da luta).", fightCap/60),
		fmt.Sprintf("**XP/h**: abate + deslocamento até o próximo (%s × a distância média ao vizinho mais perto, 0,5 × √(área por monstro)) ou descanso até %s da vida e %s da mana (regeneração de combate nos primeiros %s s e depois ×%s na vida e ×%s na mana), o que for maior. Ninguém mata mais rápido do que os monstros do seu nível (±%d) renascem (%s× o tempo base, em média); onde isso limita, a linha leva **R**.",
			num(travelFactor, 1), pct(restHPTarget)[1:], pct(restMPTarget)[1:], num(game.OutOfCombatAfter, 0), num(game.CalmHPRegenMult, 1), num(game.CalmMPRegenMult, 1), huntLevelSpread, num(respawnMean, 1)),
		"**Tempo no nível**: XP do nível ÷ XP/h. O acumulado soma do 1 até o nível. Sem mortes (quem morre caçando aparece como ∞).",
	}
}

// WriteMarkdown escreve o relatório.
func (r *Report) WriteMarkdown(w io.Writer) {
	pf := func(format string, a ...any) { fmt.Fprintf(w, format, a...) }
	pf("# Simulador de balanceamento\n\n")
	pf("Gerado por `go run ./cmd/balance` (em `server/`). Cada número sai das fórmulas do pacote `game`; mudou curva, item, skill ou monstro, rode de novo.\n\n")
	pf("## Suposições\n\n")
	for _, a := range Assumptions(r.Night) {
		pf("- %s\n", a)
	}

	pf("\n## Tempo até o 30\n\n")
	pf("| Caminho | +0 | +%d | +0 sem limite de renascimento | +%d sem limite |\n|---|---:|---:|---:|---:|\n", game.EnchantSafe, game.EnchantSafe)
	for _, p := range Paths() {
		a, b := r.Rows[0][p], r.Rows[game.EnchantSafe][p]
		pf("| %s | %s | %s | %s | %s |\n", p.Name(), dur(TimeTo30(a)), dur(TimeTo30(b)), dur(TimeTo30Free(a)), dur(TimeTo30Free(b)))
	}

	pf("\n## Tempo para matar por faixa (regime, +0)\n\n")
	pf("Média dos níveis da faixa; entre parênteses, a distância da média da faixa (tolerância do teste: ±%s).\n\n", pct(ClassTolerance)[1:])
	for _, b := range Bands {
		ttk := BandTTK(b, 0)
		mean := Mean(ttk)
		pf("- **%s** (média %s s): ", b, num(mean, 1))
		var parts []string
		for _, p := range BandEntities(b) {
			parts = append(parts, fmt.Sprintf("%s %s s (%s)", p.Name(), num(ttk[p], 1), pct(ttk[p]/mean-1)))
		}
		pf("%s\n", strings.Join(parts, " · "))
	}

	pf("\n## Orçamento de poder dos itens\n\n")
	pf("Poder = ataque + defesas + vida/10 + mana/10 + 1 por 1%% de crítico e de velocidade + … (pesos em `power.go`). Orçamento = (%s + %s × (nível − 1)) × raridade (%s) × peso do encaixe. **%% orç.**: poder +0 ÷ orçamento (tolerância +%s). **+10 ÷ próxima**: o +10 contra a peça seguinte da escada +0 (tolerância +%s).\n\n",
		num(budgetBase, 0), num(budgetPerLevel, 1), gradeList(), pct(ItemTolerance)[1:], pct(EnchantTolerance)[1:])
	pf("| Família | Peça | Raridade | Nível | Poder +0 | +3 | +10 | Orçamento | %% orç. | +10 ÷ próxima |\n|---|---|---|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, fam := range Families() {
		ladder := Ladder(fam)
		for i, d := range ladder {
			next := "—"
			if i+1 < len(ladder) {
				next = pct(ItemPower(d, game.MaxEnchant)/ItemPower(ladder[i+1], 0) - 1)
			}
			pf("| %s | %s | %s | %d | %s | %s | %s | %s | %s | %s |\n", famName(fam), d.Name, gradeName(d.Grade), d.ReqLevel,
				num(ItemPower(d, 0), 1), num(ItemPower(d, 3), 1), num(ItemPower(d, game.MaxEnchant), 1), num(ItemBudget(d), 1),
				pct(ItemPower(d, 0)/ItemBudget(d)-1), next)
		}
	}

	pf("\n## Fora do orçamento hoje (exceções conhecidas do teste)\n\n")
	for _, v := range AllViolations() {
		pf("- `%s` %s: %s\n", v.Key, pct(v.Excess), v.Detail)
	}

	pf("\n## Por nível\n\n")
	pf("Pares **a / b** = equipamento +0 / +%d; o resto é +0. **R** = o renascimento limita o ritmo.\n", game.EnchantSafe)
	header := "\n| Nv | Região | DPS | Matar (s) | Abertura (s) | Morrer | Morrer sem poção | Vida/luta | XP/abate | Abates/h | XP/h | Poções/h | No nível | Acumulado |\n|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n"
	for _, c := range Classes() {
		paths := []Path{}
		for _, p := range Paths() {
			if p.Class == c {
				paths = append(paths, p)
			}
		}
		pf("\n### %s (1–%d)\n", game.Classes[c].Name, game.SpecLevel-1)
		pf("%s", header)
		r.writeRows(w, paths[0], 1, game.SpecLevel-1)
		for _, p := range paths {
			pf("\n### %s (%d–%d)\n", p.Name(), game.SpecLevel, game.MaxLevel)
			pf("%s", header)
			r.writeRows(w, p, game.SpecLevel, game.MaxLevel)
		}
	}
}

func (r *Report) writeRows(w io.Writer, p Path, lo, hi int) {
	a, b := r.Rows[0][p], r.Rows[game.EnchantSafe][p]
	var cumA, cumB float64
	for i := range a {
		ra, rb := a[i], b[i]
		if ra.Level < lo {
			cumA += ra.LevelTime
			cumB += rb.LevelTime
			continue
		}
		if ra.Level > hi {
			break
		}
		cumA += ra.LevelTime
		cumB += rb.LevelTime
		region := ra.Hunt.Region
		if ra.RespawnLimited {
			region += " **R**"
		}
		fmt.Fprintf(w, "| %d | %s | %s / %s | %s / %s | %s | %s | %s | %s | %s | %s | %s / %s | %s | %s / %s | %s / %s |\n",
			ra.Level, region, num(ra.DPS, 0), num(rb.DPS, 0), num(ra.TTK, 1), num(rb.TTK, 1), num(ra.TTKOpen, 1),
			durShort(ra.TTD), durShort(ra.TTDNoPot), pct(ra.HPLoss)[1:], thousands(ra.XPKill), num(ra.KillsH, 0),
			thousands(ra.XPH), thousands(rb.XPH), num(ra.PotsH, 0), dur(ra.LevelTime), dur(rb.LevelTime), dur(cumA), dur(cumB))
	}
}

func durShort(s float64) string {
	if math.IsInf(s, 1) {
		return "∞"
	}
	return dur(s)
}

func gradeList() string {
	var parts []string
	for g := game.GradeCommon; g <= game.GradeLegendary; g++ {
		parts = append(parts, gradeName(g)+" ×"+num(gradeBudget[g], 1))
	}
	return strings.Join(parts, ", ")
}

func famName(f string) string {
	if c, ok := strings.CutPrefix(f, "weapon:"); ok {
		return "arma (" + strings.ToLower(game.Classes[c].Name) + ")"
	}
	return strings.TrimPrefix(f, "kind:")
}

// WriteCSV escreve uma linha por caminho, nível e encantamento (ponto decimal).
func (r *Report) WriteCSV(w io.Writer) error {
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"classe", "caminho", "nivel", "encantamento", "regiao", "dps", "matar_s", "abertura_s",
		"morrer_s", "morrer_sem_pocao_s", "vida_por_luta", "xp_abate", "abates_h", "xp_h", "pocoes_h",
		"limitado_renascimento", "nivel_s", "acumulado_s", "nivel_sem_limite_s"})
	f := func(v float64) string {
		if math.IsInf(v, 1) {
			return "inf"
		}
		return strconv.FormatFloat(v, 'f', 3, 64)
	}
	for _, e := range Enchants {
		seen := map[string]bool{}
		for _, p := range Paths() {
			cum := 0.0
			first := !seen[p.Class] // antes do 20 os caminhos da classe são a mesma linha
			seen[p.Class] = true
			for _, row := range r.Rows[e][p] {
				cum += row.LevelTime
				spec := ""
				if row.Level >= game.SpecLevel {
					spec = p.Spec
				} else if !first {
					continue
				}
				_ = cw.Write([]string{p.Class, spec, strconv.Itoa(row.Level), strconv.Itoa(e), row.Hunt.Region,
					f(row.DPS), f(row.TTK), f(row.TTKOpen), f(row.TTD), f(row.TTDNoPot), f(row.HPLoss), f(row.XPKill),
					f(row.KillsH), f(row.XPH), f(row.PotsH), strconv.FormatBool(row.RespawnLimited), f(row.LevelTime), f(cum),
					f(row.LevelTimeFree)})
			}
		}
	}
	cw.Flush()
	return cw.Error()
}
