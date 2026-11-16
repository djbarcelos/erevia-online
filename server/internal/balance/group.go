package balance

import (
	"fmt"
	"io"
	"math"

	"erevia/server/internal/game"
)

// ---------------------------------------------------------------------------------------
// Modo grupo (tunnel#151): XP por hora de cada membro, caçando junto, contra caçar sozinho
// ---------------------------------------------------------------------------------------
//
// O grupo bate no mesmo monstro: a vida dele é uma só, e cada membro segue a própria
// rotação (o tempo para matar sai do dano somado). O monstro só bate no primeiro da lista
// (o tanque, que segura o ódio); os outros ficam no alcance do próprio ataque. Os efeitos no
// alvo de cada um (marca, enfraquecer, atordoar) valem só para o golpe dele: o modelo
// subestima o grupo, nunca o contrário.
//
// Entre um abate e outro o grupo anda junto (na velocidade do mais lento) e espera o mais
// cansado chegar às metas de vida e mana. O grupo é um caçador só para o renascimento:
// não mata mais rápido do que os monstros do nível voltam (Hunt.MinCycle).
//
// A XP do abate é a da zona: MonsterXP pelo maior nível, × game.PartyXPBonus(membros), ×
// game.PartyGapMult(diferença), dividida pelo nível de cada um.

// GroupMaxAbove: o grupo procura monstros até 5 níveis acima do próprio (onde rende mais).
const GroupMaxAbove = 5

// Share: quem mais caça na região (o renascimento é o gargalo do 13 em diante, então a
// comparação depende de quantos dividem os monstros).
type Share int

const (
	// ShareRegion: a mesma gente na região nos dois casos. Os N do grupo, caçando sozinhos,
	// ficam cada um com 1/N dos renascimentos; em grupo, o grupo fica com todos.
	ShareRegion Share = iota
	// ShareFree: monstros à vontade (sem o limite do renascimento).
	ShareFree
	// ShareAlone: a região inteira para um só, seja o grupo ou quem caça sozinho (o grupo
	// divide entre N o que um sozinho teria: o pior caso para o grupo).
	ShareAlone
)

// minCycle: o menor intervalo entre abates de um caçador (ou do grupo) de n pessoas.
func (s Share) minCycle(h Hunt, n int, group bool) float64 {
	switch {
	case s == ShareFree:
		return 0
	case s == ShareRegion && !group:
		return h.MinCycle() * float64(n)
	}
	return h.MinCycle()
}

// SoloXPH: XP/h de quem caça sozinho no próprio nível, com o renascimento dividido por n.
func SoloXPH(p Path, level int, share Share, n int) float64 {
	b := NewBuild(p, level, 0)
	h := HuntFor(level)
	var wsum, cycle, xp float64
	for _, hm := range h.Monsters {
		r := huntRegimeCap(b, h, hm.Def, level, false, share.minCycle(h, n, false))
		if r.died {
			return 0
		}
		w := float64(hm.Count)
		wsum += w
		cycle += w * r.cycle
		xp += w * float64(game.MonsterXP(hm.Def, level, level))
	}
	return xp / wsum * 3600 / (cycle / wsum)
}

// GroupTrio: o grupo de 3 típico: o tanque primeiro, depois dano mágico e físico.
var GroupTrio = []Path{{"knight", "bastion"}, {"mage", "elementalist"}, {"rogue", "cutthroat"}}

// GroupRow: o grupo num nível, caçando monstros Above níveis acima.
type GroupRow struct {
	Level, Above   int
	Region         string
	TTK            float64 // tempo para matar com o dano somado (regime)
	Cycle          float64 // segundos por abate
	XPKillMember   float64 // XP de cada membro por abate (todos do mesmo nível)
	XPHMember      float64
	SoloXPH        []float64 // XP/h de cada membro caçando sozinho no próprio nível
	SoloAvg        float64
	Died           bool
	RespawnLimited bool
}

// Ratio: XP/h de cada membro no grupo ÷ a média de quem caça sozinho.
func (g GroupRow) Ratio() float64 { return g.XPHMember / g.SoloAvg }

// MinRatio: contra a classe que mais rende sozinha.
func (g GroupRow) MinRatio() float64 {
	best := 0.0
	for _, v := range g.SoloXPH {
		best = math.Max(best, v)
	}
	return g.XPHMember / best
}

// groupKill: uma luta do grupo contra um monstro (a partir do instante comum dos membros).
func groupKill(fs []*fight, def *game.MonsterDef, level int) Fight {
	hp := game.MonsterStats(def, level).MaxHP
	for _, f := range fs {
		f.engage(def, level)
		f.shared = &hp
	}
	tank := fs[0]
	start := tank.now
	var out Fight
fight:
	for step := 1; ; step++ {
		now := start + float64(step)*tickDT
		for _, f := range fs {
			f.now = now
		}
		if now-start > fightCap {
			out.Capped = true
			break
		}
		for _, f := range fs {
			if f.tickDoTs(); tank.mDead() {
				break fight
			}
		}
		for _, f := range fs {
			if f.playerActs(); tank.mDead() {
				break fight
			}
		}
		if tank.monsterActs(); tank.hp <= 0 {
			out.Died = true
			break
		}
		for _, f := range fs {
			f.regen()
			f.potions()
		}
	}
	for _, f := range fs {
		o := f.finish(start)
		out.DamageDone += o.DamageDone
		out.PotsHP += o.PotsHP
		out.PotsMP += o.PotsMP
		f.shared = nil
	}
	out.Time = tank.out.Time
	return out
}

// groupRegime: o grupo caçando um tipo de monstro sem parar (como huntRegime).
func groupRegime(bs []*Build, h Hunt, def *game.MonsterDef, level int, minCycle float64) regime {
	fs := make([]*fight, len(bs))
	travel := 0.0
	for i, b := range bs {
		fs[i] = newFight(b, false, true)
		travel = math.Max(travel, h.TravelSeconds(b.Stats.Speed))
	}
	var r regime
	var fightTime float64
	for k := range warmupKills + measureKills {
		out := groupKill(fs, def, level)
		if out.Died || out.Capped {
			r.died = true
			return r
		}
		down := travel
		for _, f := range fs {
			down = math.Max(down, f.restNeeded())
		}
		if out.Time+down < minCycle {
			down = minCycle - out.Time
			r.respawnLimited = true
		}
		if k >= warmupKills {
			fightTime += out.Time
			r.cycle += out.Time + down
		}
		for _, f := range fs {
			f.rest(down)
		}
	}
	r.ttk = fightTime / measureKills
	r.cycle /= measureKills
	return r
}

// GroupXP: a parte de cada membro na XP de um abate (zona: shareMonsterXP).
func GroupXP(def *game.MonsterDef, monLevel int, levels []int) []float64 {
	top, low, sum := 0, math.MaxInt, 0
	for _, l := range levels {
		top, low, sum = max(top, l), min(low, l), sum+l
	}
	total := float64(game.MonsterXP(def, monLevel, top)) * game.PartyXPBonus(len(levels)) * game.PartyGapMult(top-low)
	out := make([]float64, len(levels))
	for i, l := range levels {
		out[i] = total * float64(l) / float64(sum)
	}
	return out
}

// SimulateGroup: o grupo de paths, todos no nível, caçando monstros above níveis acima.
func SimulateGroup(paths []Path, level, above int, share Share) GroupRow {
	mon := min(level+above, game.MaxLevel)
	row := GroupRow{Level: level, Above: mon - level}
	bs := make([]*Build, len(paths))
	levels := make([]int, len(paths))
	for i, p := range paths {
		bs[i] = NewBuild(p, level, 0)
		levels[i] = level
		s := SoloXPH(p, level, share, len(paths))
		row.SoloXPH = append(row.SoloXPH, s)
		row.SoloAvg += s / float64(len(paths))
	}
	h := HuntFor(mon)
	row.Region = h.Region
	var wsum, ttk, cycle, xp float64
	for _, hm := range h.Monsters {
		r := groupRegime(bs, h, hm.Def, mon, share.minCycle(h, len(paths), true))
		if r.died {
			row.Died = true
			return row
		}
		w := float64(hm.Count)
		row.RespawnLimited = row.RespawnLimited || r.respawnLimited
		wsum += w
		ttk += w * r.ttk
		cycle += w * r.cycle
		xp += w * GroupXP(hm.Def, mon, levels)[0]
	}
	row.TTK, row.Cycle, row.XPKillMember = ttk/wsum, cycle/wsum, xp/wsum
	row.XPHMember = row.XPKillMember * 3600 / row.Cycle
	return row
}

// BestGroup: o grupo no nível, nos monstros (0 a GroupMaxAbove acima) que mais rendem por
// membro sem ninguém morrer.
func BestGroup(paths []Path, level int, share Share) GroupRow {
	var best GroupRow
	for a := 0; a <= GroupMaxAbove && level+a <= game.MaxLevel; a++ {
		g := SimulateGroup(paths, level, a, share)
		if !g.Died && g.XPHMember > best.XPHMember {
			best = g
		}
	}
	return best
}

// CarryRow: um nível alto caça os monstros dele com um nível baixo no grupo (o baixo ajuda,
// então conta). Compara a XP/h do baixo com a dele caçando sozinho.
type CarryRow struct {
	High, Low            int
	GapMult              float64
	LowXPH, LowSoloXPH   float64
	HighXPH, HighSoloXPH float64
}

// SimulateCarry: cavaleiro no nível high e mago no low, nos monstros do high.
func SimulateCarry(high, low int) CarryRow {
	paths := []Path{{"knight", "bastion"}, {"mage", "elementalist"}}
	bs := []*Build{NewBuild(paths[0], high, 0), NewBuild(paths[1], low, 0)}
	levels := []int{high, low}
	h := HuntFor(high)
	row := CarryRow{High: high, Low: low, GapMult: game.PartyGapMult(high - low)}
	var wsum, cycle, xpHigh, xpLow float64
	for _, hm := range h.Monsters {
		r := groupRegime(bs, h, hm.Def, high, h.MinCycle())
		if r.died {
			return row
		}
		w := float64(hm.Count)
		share := GroupXP(hm.Def, high, levels)
		wsum += w
		cycle += w * r.cycle
		xpHigh += w * share[0]
		xpLow += w * share[1]
	}
	c := cycle / wsum
	row.HighXPH, row.LowXPH = xpHigh/wsum*3600/c, xpLow/wsum*3600/c
	row.HighSoloXPH = Simulate(paths[0], high, 0, false).XPH
	row.LowSoloXPH = Simulate(paths[1], low, 0, false).XPH
	return row
}

// WeakRow: um caçador do nível caçando monstros below níveis abaixo (o bot no monstro fraco).
type WeakRow struct {
	Level, Below   int
	XPMult         float64 // o corte de MonsterXP pela diferença (1 até 5 níveis)
	XPH            float64
	Ratio          float64 // ÷ a XP/h no próprio nível
	RespawnLimited bool
}

// SimulateWeak: o caminho no nível caçando a região dos monstros below níveis abaixo.
func SimulateWeak(p Path, level, below int) WeakRow {
	mon := max(1, level-below)
	b := NewBuild(p, level, 0)
	h := HuntFor(mon)
	row := WeakRow{Level: level, Below: level - mon}
	var wsum, cycle, xp float64
	for _, hm := range h.Monsters {
		r := huntRegime(b, h, hm.Def, mon, false, true)
		if r.died {
			return row
		}
		w := float64(hm.Count)
		row.RespawnLimited = row.RespawnLimited || r.respawnLimited
		wsum += w
		cycle += w * r.cycle
		xp += w * float64(game.MonsterXP(hm.Def, mon, level))
		row.XPMult = float64(game.MonsterXP(hm.Def, mon, level)) / float64(game.MonsterXP(hm.Def, mon, mon))
	}
	row.XPH = xp / wsum * 3600 / (cycle / wsum)
	row.Ratio = row.XPH / Simulate(p, level, 0, false).XPH
	return row
}

// GroupLevels: os níveis das tabelas do modo grupo.
var GroupLevels = []int{5, 10, 15, 20, 25, 30, 35, 39}

// WriteGroupMarkdown: as tabelas do modo grupo (go run ./cmd/balance -grupo).
func WriteGroupMarkdown(w io.Writer) {
	names := ""
	for i, p := range GroupTrio {
		if i > 0 {
			names += " + "
		}
		names += p.Name()
	}
	fmt.Fprintf(w, "Grupo de 3 do mesmo nível (%s), +0, de dia. Bônus: +%s%% por membro além do primeiro; a XP é dividida pelo nível (aqui, 1/3 para cada). O grupo caça os monstros de até %d níveis acima que mais rendem.\n\n",
		names, num(game.PartyBonusPerMember*100, 0), GroupMaxAbove)
	fmt.Fprintln(w, "Região dividida: os mesmos 3 na região nos dois casos (sozinhos, cada um fica com 1/3 dos renascimentos).")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Nível | Caça (acima) | Matar (grupo) | s/abate | XP/abate (membro) | XP/h (membro) | XP/h sozinho (média) | Grupo ÷ sozinho | ÷ melhor sozinho | Mesmos monstros do solo | Monstros à vontade | Região só sua |")
	fmt.Fprintln(w, "|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|")
	for _, l := range GroupLevels {
		g := BestGroup(GroupTrio, l, ShareRegion)
		same := SimulateGroup(GroupTrio, l, 0, ShareRegion)
		free := BestGroup(GroupTrio, l, ShareFree)
		alone := BestGroup(GroupTrio, l, ShareAlone)
		r := ""
		if g.RespawnLimited {
			r = " R"
		}
		fmt.Fprintf(w, "| %d | %s (+%d) | %s s | %s%s | %s | %s | %s | %s× | %s× | %s× | %s× | %s× |\n", l, g.Region, g.Above,
			num(g.TTK, 1), num(g.Cycle, 1), r, thousands(g.XPKillMember), thousands(g.XPHMember), thousands(g.SoloAvg),
			num(g.Ratio(), 2), num(g.MinRatio(), 2), num(same.Ratio(), 2), num(free.Ratio(), 2), num(alone.Ratio(), 2))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Carregar: cavaleiro no nível alto caçando os monstros dele, com um mago de nível baixo que ajuda (conta no grupo).")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Alto | Baixo | Diferença | XP que sobra | XP/h do baixo | Baixo sozinho | Baixo ÷ sozinho | XP/h do alto | Alto ÷ sozinho |")
	fmt.Fprintln(w, "|---:|---:|---:|---:|---:|---:|---:|---:|---:|")
	for _, c := range [][2]int{{30, 22}, {30, 21}, {30, 20}, {30, 17}, {39, 31}, {39, 30}, {39, 25}} {
		r := SimulateCarry(c[0], c[1])
		fmt.Fprintf(w, "| %d | %d | %d | %s%% | %s | %s | %s× | %s | %s× |\n", r.High, r.Low, r.High-r.Low, num(r.GapMult*100, 0),
			thousands(r.LowXPH), thousands(r.LowSoloXPH), num(r.LowXPH/r.LowSoloXPH, 2), thousands(r.HighXPH), num(r.HighXPH/r.HighSoloXPH, 2))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Monstro fraco: um caçador sozinho (Mago/Elementalista, +0) caçando a região de monstros abaixo do nível dele. A regra: XP cheia até 5 níveis abaixo; depois −15% por nível, mínimo de 10%.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Nível | Monstros abaixo | Corte de XP | XP/h | ÷ no próprio nível |")
	fmt.Fprintln(w, "|---:|---:|---:|---:|---:|")
	for _, l := range []int{20, 30, 39} {
		for _, d := range []int{0, 5, 8, 11, 14, 19} {
			if l-d < 1 {
				continue
			}
			r := SimulateWeak(Path{"mage", "elementalist"}, l, d)
			fmt.Fprintf(w, "| %d | %d | %s%% | %s | %s× |\n", l, r.Below, num(r.XPMult*100, 0), thousands(r.XPH), num(r.Ratio, 2))
		}
	}
}
