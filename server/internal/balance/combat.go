package balance

import (
	"fmt"
	"io"
	"math"
	"sort"

	"erevia/server/internal/game"
)

// ---------------------------------------------------------------------------------------
// Combate do 20 ao 30 por caminho (tunnel#152): as metas de tempo para matar e para morrer
// ---------------------------------------------------------------------------------------
//
// Metas propostas na tunnel#152 (a confirmar com o Daniel):
//   - monstro comum do nível, sozinho: cai em ~8–12 s, e o jogador perde ~30–50% da vida;
//   - elite: ~30 s;
//   - chefe de campo em grupo de 3: ~2–4 min;
//   - PvP do mesmo nível e equipamento: ninguém morre em menos de ~6 s, nem com todas as
//     recargas prontas.
//
// Tudo com o equipamento típico +combatEnchant, de dia, a partir de vida e mana cheias e
// nada em recarga (a abertura de uma luta, KillFight).

// Metas (segundos e fração da vida).
const (
	TargetCommonMin, TargetCommonMax = 8.0, 12.0
	TargetLossMin, TargetLossMax     = 0.30, 0.50
	TargetElite                      = 30.0
	TargetBossMin, TargetBossMax     = 120.0, 240.0
	TargetPvPMin                     = 6.0
)

const (
	combatEnchant = game.EnchantSafe
	combatElite   = "ossario_cavaleiro" // a elite das Terras Mortas (20–30)
)

// CombatRow é um caminho num nível.
type CombatRow struct {
	Path  Path
	Level int

	Common     float64 // segundos para matar o monstro comum da região (média pelo número de cada um)
	CommonLoss float64 // fração da vida perdida nessa luta
	CommonDied bool
	Elite      float64 // segundos para matar a elite do nível sozinho (+Inf = morreu ou não caiu)
	EliteLoss  float64

	// PvP: o menor tempo para este caminho matar outro do mesmo nível, com todas as recargas
	// prontas e o alvo parado, sem revidar nem beber poção (o pior caso), e quem.
	PvPKill   float64
	PvPVictim Path
}

// CombatTable: todas as linhas do 20 ao 29.
func CombatTable() []CombatRow {
	var out []CombatRow
	for l := game.SpecLevel; l < game.SpecLevel+10 && l < game.MaxLevel; l++ {
		builds := map[Path]*Build{}
		for _, p := range Paths() {
			builds[p] = NewBuild(p, l, combatEnchant)
		}
		for _, p := range Paths() {
			out = append(out, combatRow(builds, p, l))
		}
	}
	return out
}

func combatRow(builds map[Path]*Build, p Path, level int) CombatRow {
	b := builds[p]
	row := CombatRow{Path: p, Level: level, PvPKill: math.Inf(1)}
	h := HuntFor(level)
	var w, t, loss float64
	for _, hm := range h.Monsters {
		f := KillFight(b, hm.Def, level, false)
		if f.Died {
			row.CommonDied = true
		}
		k := float64(hm.Count)
		w += k
		t += k * f.Time
		loss += k * (1 - f.HP/b.Stats.MaxHP)
	}
	if w > 0 {
		row.Common, row.CommonLoss = t/w, loss/w
	}
	if def := game.Monsters[combatElite]; def != nil {
		f := KillFight(b, def, level, false)
		row.Elite, row.EliteLoss = f.Time, 1-f.HP/b.Stats.MaxHP
		if f.Died || f.Capped {
			row.Elite = math.Inf(1)
		}
	}
	for _, q := range Paths() {
		if t := PvPKillTime(b, builds[q]); t < row.PvPKill {
			row.PvPKill, row.PvPVictim = t, q
		}
	}
	return row
}

// pvpDummy: o alvo do PvP no simulador (um "monstro" sem ataque, com os atributos do outro
// jogador).
var pvpDummy = &game.MonsterDef{ID: "pvp_alvo", Name: "Alvo", BodyRadius: 0.5}

// PvPKillTime: quanto o atacante leva para matar o alvo (mesmo nível), abrindo com tudo
// pronto. O alvo não revida, não se cura e não bebe poção: é o pior caso para ele.
func PvPKillTime(att, target *Build) float64 {
	f := newFight(att, false, false)
	f.engage(pvpDummy, att.Level)
	s := target.Stats
	f.m = game.Stats{MaxHP: s.MaxHP, PDef: s.PDef, MDef: s.MDef, Evasion: s.Evasion, Range: s.Range, AtkInterval: 99}
	f.mhp, f.mRange = s.MaxHP, 0
	out := f.run()
	if out.Capped || out.Died {
		return math.Inf(1)
	}
	return out.Time
}

// BossRow: um chefe de campo contra um grupo de 3 (tanque, mago e ladino).
type BossRow struct {
	Boss  string
	Level int
	Group []Path
	Time  float64 // segundos (+Inf = o tanque morreu ou passou do limite)
	Died  bool
}

// combatGroup: o grupo de referência para os chefes de campo.
var combatGroup = []Path{{"knight", "bastion"}, {"mage", "elementalist"}, {"rogue", "cutthroat"}}

// BossTable: os chefes de campo da faixa.
func BossTable() []BossRow {
	var out []BossRow
	for _, c := range []struct {
		id    string
		level int
	}{{"mae_lodo", 21}, {"bone_lord", 22}} {
		def := game.Monsters[c.id]
		if def == nil {
			continue
		}
		var fs []*fight
		for _, p := range combatGroup {
			fs = append(fs, newFight(NewBuild(p, c.level, combatEnchant), false, true))
		}
		f := groupKill(fs, def, c.level)
		row := BossRow{Boss: def.Name, Level: c.level, Group: combatGroup, Time: f.Time, Died: f.Died}
		if f.Died || f.Capped {
			row.Time = math.Inf(1)
		}
		out = append(out, row)
	}
	return out
}

// WriteCombatMarkdown: as tabelas (go run ./cmd/balance -combate).
func WriteCombatMarkdown(w io.Writer) {
	pf := func(f string, a ...any) { fmt.Fprintf(w, f, a...) }
	sec := func(v float64) string {
		if math.IsInf(v, 1) {
			return "—"
		}
		return num(v, 1) + " s"
	}
	rows := CombatTable()
	pf("## Combate do 20 ao 30 por caminho (tunnel#152)\n\n")
	pf("Equipamento típico +%d, de dia, abrindo a luta com vida e mana cheias e tudo pronto. ", combatEnchant)
	pf("Metas: comum em %.0f–%.0f s perdendo %.0f–%.0f%% da vida; elite (%s) em ~%.0f s; chefe de campo em grupo de 3 em %.0f–%.0f min; ",
		TargetCommonMin, TargetCommonMax, TargetLossMin*100, TargetLossMax*100, game.Monsters[combatElite].Name, TargetElite, TargetBossMin/60, TargetBossMax/60)
	pf("PvP: ninguém morre em menos de %.0f s (o alvo parado, sem revidar nem beber poção).\n\n", TargetPvPMin)

	// Resumo por caminho: médias na faixa e o pior PvP.
	type agg struct {
		common, loss, elite float64
		n, eliteN           int
		pvp                 float64
		pvpAt               int
		victim              Path
	}
	by := map[Path]*agg{}
	for _, r := range rows {
		a := by[r.Path]
		if a == nil {
			a = &agg{pvp: math.Inf(1)}
			by[r.Path] = a
		}
		a.common += r.Common
		a.loss += r.CommonLoss
		a.n++
		if !math.IsInf(r.Elite, 1) {
			a.elite += r.Elite
			a.eliteN++
		}
		if r.PvPKill < a.pvp {
			a.pvp, a.pvpAt, a.victim = r.PvPKill, r.Level, r.PvPVictim
		}
	}
	var mean float64
	for _, a := range by {
		mean += a.common / float64(a.n)
	}
	mean /= float64(len(by))
	pf("### Resumo (média do 20 ao 29)\n\n")
	pf("| Caminho | Comum | vs média | Vida perdida | Elite | PvP mais rápido | Contra |\n|---|---:|---:|---:|---:|---:|---|\n")
	paths := Paths()
	sort.SliceStable(paths, func(i, j int) bool { return paths[i].Name() < paths[j].Name() })
	for _, p := range paths {
		a := by[p]
		c := a.common / float64(a.n)
		el := math.Inf(1)
		if a.eliteN > 0 {
			el = a.elite / float64(a.eliteN)
		}
		pf("| %s | %s | %+.0f%% | %.0f%% | %s | %s (nv %d) | %s |\n", p.Name(), sec(c), (c/mean-1)*100, a.loss/float64(a.n)*100,
			sec(el), sec(a.pvp), a.pvpAt, a.victim.Name())
	}
	pf("\n### Chefes de campo em grupo (%s)\n\n| Chefe | Nível | Tempo |\n|---|---:|---:|\n", groupNames(combatGroup))
	for _, b := range BossTable() {
		t := sec(b.Time)
		if b.Died {
			t = "o tanque morre"
		}
		pf("| %s | %d | %s |\n", b.Boss, b.Level, t)
	}
	pf("\n### Por nível\n\n| Nível | Caminho | Comum | Vida perdida | Elite | PvP mais rápido |\n|---:|---|---:|---:|---:|---:|\n")
	for _, r := range rows {
		pf("| %d | %s | %s | %.0f%% | %s | %s |\n", r.Level, r.Path.Name(), sec(r.Common), r.CommonLoss*100, sec(r.Elite), sec(r.PvPKill))
	}
}

func groupNames(ps []Path) string {
	s := ""
	for i, p := range ps {
		if i > 0 {
			s += ", "
		}
		s += p.Name()
	}
	return s
}
