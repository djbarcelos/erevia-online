package game

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// Progressão teórica (docs/progressao.md, tunnel#147): quantos abates e quantas horas cada
// nível pede, pela curva de XP (XPToNext), pela XP dos monstros (MonsterXP) e pelos
// monstros que existem em cada faixa (DefaultSpawns). O teste TestProgressionDocUpToDate
// confere que a tabela do documento é a que sai daqui; mudou a curva ou um monstro, é só
// rodar EREVIA_UPDATE_DOCS=1 go test ./internal/game -run TestProgressionDoc.
//
// Premissas (as mesmas do documento):
//   - o jogador caça sozinho, de dia, monstros do próprio nível (XP cheia, sem noite, sem
//     grupo e sem XP em dobro) e não morre;
//   - em cada nível, caça no grupo de monstros mais avançado que cobre aquele nível (no 20,
//     o Vale dos golens, não o anel dos magos); grupos do mesmo começo entram na média pela
//     quantidade de cada um no mapa. Chefes e o raid boss ficam de fora;
//   - a XP das missões entra no nível em que a missão chega (o tutorial, no 1; as de
//     Unlock, no nível delas) e o que sobra passa para os níveis seguintes;
//   - cada abate custa secPerKill segundos de jogo (luta + deslocar e procurar o próximo).

// SecondsPerKill: a premissa do documento, 10 s de luta + 15 s para andar e achar o próximo.
const SecondsPerKill = 25.0

// ProgressionRow é um nível da tabela teórica: o que custa sair de Level para Level+1.
type ProgressionRow struct {
	Level     int
	XPToNext  int64
	Monsters  string  // nomes dos monstros caçados
	XPPerKill float64 // XP média por abate (monstros do nível Level)
	QuestXP   int64   // XP de missões usada neste nível
	Kills     int
	Hours     float64
}

// huntGroup: os spawns caçados no nível l (o grupo de começo mais alto que cobre o nível,
// sem chefes nem pontos fixos).
func huntGroup(l int) []Spawn {
	best := -1
	var group []Spawn
	for _, s := range DefaultSpawns() {
		def := Monsters[s.Monster]
		if def == nil || def.Boss || def.Raid || s.Fixed {
			continue
		}
		hi := max(s.LevelMax, s.LevelMin)
		if l < s.LevelMin || l > hi {
			continue
		}
		switch {
		case s.LevelMin > best:
			best, group = s.LevelMin, []Spawn{s}
		case s.LevelMin == best:
			group = append(group, s)
		}
	}
	return group
}

// huntMonsters: os ids dos monstros caçados no nível l, sem repetir, na ordem do mapa.
func huntMonsters(l int) []string {
	var ids []string
	for _, s := range huntGroup(l) {
		if !slices.Contains(ids, s.Monster) {
			ids = append(ids, s.Monster)
		}
	}
	return ids
}

// huntXP é a XP média por abate no nível l e os nomes dos monstros caçados (regra acima).
func huntXP(l int) (float64, string) {
	group := huntGroup(l)
	if len(group) == 0 {
		return 0, ""
	}
	var xp float64
	var count int
	var names []string
	for _, s := range group {
		def := Monsters[s.Monster]
		xp += float64(MonsterXP(def, l, l) * int64(s.Count))
		count += s.Count
		if !slices.Contains(names, def.Name) {
			names = append(names, def.Name)
		}
	}
	return xp / float64(count), strings.Join(names, ", ")
}

// TheoreticalProgression monta a tabela do nível 1 ao MaxLevel−1.
func TheoreticalProgression(secPerKill float64) []ProgressionRow {
	questAt := map[int]int64{}
	for _, q := range questList {
		questAt[max(1, q.Unlock)] += q.Reward.XP
	}
	var left int64
	out := make([]ProgressionRow, 0, MaxLevel-1)
	for l := 1; l < MaxLevel; l++ {
		left += questAt[l]
		need := XPToNext(l)
		used := min(left, need)
		left -= used
		perKill, names := huntXP(l)
		kills := 0
		if perKill > 0 {
			kills = int(math.Ceil(float64(need-used) / perKill))
		}
		out = append(out, ProgressionRow{
			Level: l, XPToNext: need, Monsters: names, XPPerKill: perKill, QuestXP: used,
			Kills: kills, Hours: float64(kills) * secPerKill / 3600,
		})
	}
	return out
}

// ProgressionBand é uma faixa do resumo (de From até chegar a To).
type ProgressionBand struct {
	From, To    int
	Kills       int
	Hours       float64
	TargetHours float64 // tempo-alvo do dono do jogo (tunnel#149)
	XP, QuestXP int64   // XP da faixa e quanto dela vem de missões
}

// ProgressionTargets: as faixas e o tempo-alvo de cada uma (tunnel#149, ~150 h até o 40).
var ProgressionTargets = []ProgressionBand{
	{From: 1, To: 10, TargetHours: 3}, {From: 10, To: 20, TargetHours: 15},
	{From: 20, To: 30, TargetHours: 40}, {From: 30, To: MaxLevel, TargetHours: 90},
}

// ProgressionBands resume a tabela nas faixas de ProgressionTargets.
func ProgressionBands(rows []ProgressionRow) []ProgressionBand {
	bands := slices.Clone(ProgressionTargets)
	for i := range bands {
		for _, r := range rows {
			if r.Level >= bands[i].From && r.Level < bands[i].To {
				bands[i].Kills += r.Kills
				bands[i].Hours += r.Hours
				bands[i].XP += r.XPToNext
				bands[i].QuestXP += r.QuestXP
			}
		}
	}
	return bands
}

// ProgressionMarkdown é a tabela do documento (entre os marcadores do docs/progressao.md).
func ProgressionMarkdown(secPerKill float64) string {
	rows := TheoreticalProgression(secPerKill)
	var b strings.Builder
	num := func(v int64) string { // 12345 → 12.345
		s := fmt.Sprint(v)
		for i := len(s) - 3; i > 0; i -= 3 {
			s = s[:i] + "." + s[i:]
		}
		return s
	}
	dec := func(v float64) string { return strings.Replace(fmt.Sprintf("%.1f", v), ".", ",", 1) }
	fmt.Fprintf(&b, "Premissa: %s s por abate. Gerada por `game.ProgressionMarkdown`.\n\n", strings.Replace(fmt.Sprint(secPerKill), ".", ",", 1))
	b.WriteString("| Nível | XP para o próximo | Caça (do mesmo nível) | XP por abate | XP de missões | Abates | Minutos | Horas acumuladas |\n")
	b.WriteString("|---:|---:|---|---:|---:|---:|---:|---:|\n")
	var total float64
	var kills int
	for _, r := range rows {
		total += r.Hours
		kills += r.Kills
		fmt.Fprintf(&b, "| %d → %d | %s | %s | %s | %s | %s | %s | %s |\n", r.Level, r.Level+1, num(r.XPToNext), r.Monsters,
			num(int64(math.Round(r.XPPerKill))), num(r.QuestXP), num(int64(r.Kills)), num(int64(math.Round(r.Hours*60))), dec(total))
	}
	fmt.Fprintf(&b, "| **1 → %d** | **%s** | | | | **%s** | **%s** | **%s h** |\n", MaxLevel, num(XPLevelStart(MaxLevel)), num(int64(kills)), num(int64(math.Round(total*60))), dec(total))
	b.WriteString("\n| Faixa | Abates | Horas | Tempo-alvo | Diferença | XP de missões |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|\n")
	for _, f := range ProgressionBands(rows) {
		diff := (f.Hours/f.TargetHours - 1) * 100
		fmt.Fprintf(&b, "| %d → %d | %s | %s | %s | %s%% | %s%% |\n", f.From, f.To, num(int64(f.Kills)), dec(f.Hours), dec(f.TargetHours),
			strings.Replace(fmt.Sprintf("%+.1f", diff), ".", ",", 1), strings.Replace(fmt.Sprintf("%.0f", 100*float64(f.QuestXP)/float64(f.XP)), ".", ",", 1))
	}
	return b.String()
}
