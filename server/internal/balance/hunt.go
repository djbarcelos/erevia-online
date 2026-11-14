package balance

import (
	"math"
	"strings"

	"erevia/server/internal/game"
)

// ---------------------------------------------------------------------------------------
// A caça: onde o personagem do nível caça, quanto anda e quanto espera
// ---------------------------------------------------------------------------------------

// Suposições da caça (sozinho, de dia, sem XP em dobro).
const (
	// Onde caça: entre os grupos de monstros que nascem no nível dele (DefaultSpawns, sem
	// chefes nem raid), o da região mais nova (maior nível mínimo): no 20 já vai para o Vale
	// das Pedras Vivas. Monstros que dividem a região entram na proporção da quantidade.
	huntLevelSpread = 2 // caça monstros até 2 níveis acima ou abaixo (contam como do nível dele)
	// Deslocamento: os monstros nascem espalhados por igual na região; a distância média até
	// o vizinho mais perto é 0,5 × √(área por monstro). Anda 50% a mais (o mais perto nem
	// sempre está livre ou é do nível) e para no alcance do ataque.
	travelFactor = 1.5
	// Renascimento: cada monstro volta entre Respawn e 2×Respawn depois de morrer (média
	// 1,5×). Quem mata mais rápido que isso espera os monstros do seu nível voltarem.
	respawnMean = 1.5
	// Descanso: depois de andar até o próximo, só puxa com 80% da vida e 50% da mana. A
	// regeneração é a de combate nos primeiros 5 s e a de fora de combate depois
	// (game.OutOfCombatAfter, CalmHPRegenMult, CalmMPRegenMult).
	restHPTarget = 0.8
	restMPTarget = 0.5
	// Regime: os primeiros abates começam com vida e mana cheias; a conta usa os seguintes.
	warmupKills  = 3
	measureKills = 12
)

// HuntMonster é um tipo de monstro da região, com a quantidade.
type HuntMonster struct {
	Def   *game.MonsterDef
	Count int
}

// Hunt é a região onde o nível caça.
type Hunt struct {
	Region             string
	Monsters           []HuntMonster
	Area               float64 // m²
	Count              int     // monstros na região (todos os níveis)
	Respawn            float64 // renascimento base médio (s)
	LevelMin, LevelMax int
}

func spawnArea(s game.Spawn) float64 {
	if s.Area {
		return math.Pi * s.Radius * s.Radius
	}
	return math.Pi * (s.RMax*s.RMax - s.RMin*s.RMin)
}

func sameRegion(a, b game.Spawn) bool {
	return a.Area == b.Area && a.RMin == b.RMin && a.RMax == b.RMax && a.X == b.X && a.Z == b.Z && a.Radius == b.Radius
}

func huntable(s game.Spawn) bool {
	d := game.Monsters[s.Monster]
	return d != nil && !s.Fixed && !d.Boss && !d.Raid && !d.NoGear && s.Count > 0
}

// HuntFor escolhe a região de caça do nível.
func HuntFor(level int) Hunt {
	spawns := game.DefaultSpawns()
	// Melhor grupo: nenhum nível fora da faixa; empate, a região mais nova.
	best := -1
	gap := func(s game.Spawn) int {
		switch {
		case level < s.LevelMin:
			return s.LevelMin - level
		case level > s.LevelMax:
			return level - s.LevelMax
		}
		return 0
	}
	for i, s := range spawns {
		if !huntable(s) {
			continue
		}
		if best < 0 || gap(s) < gap(spawns[best]) || gap(s) == gap(spawns[best]) && s.LevelMin > spawns[best].LevelMin {
			best = i
		}
	}
	ref := spawns[best]
	h := Hunt{Area: spawnArea(ref), LevelMin: ref.LevelMin, LevelMax: ref.LevelMax, Region: ref.Region}
	var names []string
	var respawn float64
	for _, s := range spawns {
		if !huntable(s) || !sameRegion(s, ref) || s.LevelMin != ref.LevelMin || s.LevelMax != ref.LevelMax {
			continue
		}
		d := game.Monsters[s.Monster]
		h.Monsters = append(h.Monsters, HuntMonster{d, s.Count})
		h.Count += s.Count
		respawn += s.Respawn * float64(s.Count)
		names = append(names, d.Name)
	}
	h.Respawn = respawn / float64(h.Count)
	if h.Region == "" {
		h.Region = strings.Join(names, " + ")
	}
	return h
}

// TravelSeconds: tempo andando até o próximo monstro, na velocidade do personagem.
func (h Hunt) TravelSeconds(speed float64) float64 {
	return travelFactor * 0.5 * math.Sqrt(h.Area/float64(h.Count)) / speed
}

// MinCycle: o menor intervalo entre abates que o renascimento dos monstros do nível deixa.
func (h Hunt) MinCycle() float64 {
	span := float64(h.LevelMax - h.LevelMin + 1)
	eligible := float64(h.Count) * math.Min(1, float64(2*huntLevelSpread+1)/span)
	return respawnMean * h.Respawn / eligible
}

// regenTo: segundos para o recurso ir de cur a target (combate nos primeiros 5 s, depois
// fora de combate; extra é regeneração que não multiplica, como a Aura de Coragem).
func regenTo(cur, target, rate, extra, calm float64) float64 {
	need := target - cur
	if need <= 0 {
		return 0
	}
	r1 := rate + extra
	if r1 <= 0 {
		return math.Inf(1)
	}
	if need <= r1*game.OutOfCombatAfter {
		return need / r1
	}
	r2 := rate*calm + extra
	return game.OutOfCombatAfter + (need-r1*game.OutOfCombatAfter)/r2
}

// regenFor: o recurso depois de t segundos parado (sem passar do máximo).
func regenFor(cur, maxv, t, rate, extra, calm float64) float64 {
	r1 := rate + extra
	t1 := math.Min(t, game.OutOfCombatAfter)
	v := cur + r1*t1 + (rate*calm+extra)*math.Max(0, t-game.OutOfCombatAfter)
	return math.Min(maxv, v)
}

// Row é uma linha da tabela: um caminho, um nível e um encantamento.
type Row struct {
	Path           Path
	Level, Enchant int
	Hunt           Hunt
	DPS            float64 // dano por segundo da rotação numa luta longa (contra a defesa do monstro)
	TTK            float64 // tempo para matar em regime (média dos monstros da região)
	TTKOpen        float64 // tempo para matar na abertura (tudo fora de recarga)
	TTD            float64 // tempo para morrer, com regeneração e poções (pior monstro; +Inf = aguenta)
	TTDNoPot       float64 // tempo para morrer só com a regeneração
	HPLoss         float64 // fração da vida perdida por luta, em regime
	Cycle          float64 // segundos por abate (luta + deslocamento/descanso, ou renascimento)
	XPKill         float64
	XPH            float64
	KillsH, PotsH  float64
	LevelTime      float64 // segundos para sair do nível (+Inf = não sai)
	LevelTimeFree  float64 // o mesmo sem o limite do renascimento (monstros à vontade)
	Died           bool    // morre caçando no nível
	RespawnLimited bool    // o renascimento limita o ritmo
}

// regime: um tipo de monstro caçado sem parar. A luta segue as recargas da anterior; entre
// uma e outra o caçador anda até o próximo (e descansa até as metas, se precisar), ou espera
// o renascimento.
type regime struct {
	ttk, dps, cycle, pots, loss float64
	died, respawnLimited        bool
}

func huntRegime(b *Build, h Hunt, def *game.MonsterDef, level int, night, respawnCap bool) regime {
	f := newFight(b, night, true)
	travel := h.TravelSeconds(b.Stats.Speed)
	var r regime
	var dmg, fightTime float64
	for k := range warmupKills + measureKills {
		hp0 := f.hp
		f.engage(def, level)
		out := f.run()
		if out.Died || out.Capped {
			r.died = true
			return r
		}
		down := math.Max(travel, f.restNeeded())
		if respawnCap && out.Time+down < h.MinCycle() {
			down = h.MinCycle() - out.Time
			r.respawnLimited = true
		}
		if k >= warmupKills {
			fightTime += out.Time
			dmg += out.DamageDone
			r.cycle += out.Time + down
			r.pots += float64(out.PotsHP + out.PotsMP)
			r.loss += (hp0 - out.HP) / b.Stats.MaxHP
		}
		f.rest(down)
	}
	r.ttk, r.dps = fightTime/measureKills, dmg/fightTime
	r.cycle /= measureKills
	r.pots /= measureKills
	r.loss /= measureKills
	return r
}

// RegimeTTK: tempo médio para matar em regime contra a região do nível (o número que o
// teste de equilíbrio entre classes compara).
func RegimeTTK(p Path, level, enchant int) float64 {
	b := NewBuild(p, level, enchant)
	h := HuntFor(level)
	var wsum, ttk float64
	for _, hm := range h.Monsters {
		r := huntRegime(b, h, hm.Def, level, false, false)
		if r.died {
			return math.Inf(1)
		}
		wsum += float64(hm.Count)
		ttk += float64(hm.Count) * r.ttk
	}
	return ttk / wsum
}

// Simulate calcula a linha do caminho no nível (o caminho só vale do 20 em diante).
func Simulate(p Path, level, enchant int, night bool) Row {
	b := NewBuild(p, level, enchant)
	h := HuntFor(level)
	row := Row{Path: b.Path, Level: level, Enchant: enchant, Hunt: h, TTD: math.Inf(1), TTDNoPot: math.Inf(1)}
	var wsum, ttk, open, dps, cycle, free, xp, pots, loss float64
	for _, hm := range h.Monsters {
		w := float64(hm.Count)
		ttd, d := TimeToDie(b, hm.Def, level, night, true)
		ttd0, _ := TimeToDie(b, hm.Def, level, night, false)
		row.TTD, row.TTDNoPot = math.Min(row.TTD, ttd), math.Min(row.TTDNoPot, ttd0)
		r := huntRegime(b, h, hm.Def, level, night, true)
		rf := huntRegime(b, h, hm.Def, level, night, false)
		if r.died {
			row.Died = true
			break
		}
		row.RespawnLimited = row.RespawnLimited || r.respawnLimited
		xpk := float64(game.MonsterXP(hm.Def, level, level))
		if night {
			xpk *= game.NightXP
		}
		wsum += w
		open += w * KillFight(b, hm.Def, level, night).Time
		ttk += w * r.ttk
		dps += w * d
		cycle += w * r.cycle
		free += w * rf.cycle
		pots += w * r.pots
		loss += w * r.loss
		xp += w * xpk
	}
	if row.Died {
		row.TTK, row.TTKOpen, row.XPH = math.Inf(1), math.Inf(1), 0
		row.LevelTime, row.LevelTimeFree = math.Inf(1), math.Inf(1)
		return row
	}
	row.TTK, row.TTKOpen, row.DPS, row.Cycle = ttk/wsum, open/wsum, dps/wsum, cycle/wsum
	row.XPKill, row.HPLoss = xp/wsum, loss/wsum
	row.KillsH = 3600 / row.Cycle
	row.XPH = row.KillsH * row.XPKill
	row.PotsH = row.KillsH * pots / wsum
	if level < game.MaxLevel {
		need := float64(game.XPToNext(level))
		row.LevelTime = need / row.XPH * 3600
		row.LevelTimeFree = need / row.XPKill * (free / wsum) // abates do nível × segundos por abate
	}
	return row
}

// Progression: as linhas do caminho do nível 1 ao máximo (game.MaxLevel).
func Progression(p Path, enchant int, night bool) []Row {
	out := make([]Row, 0, game.MaxLevel)
	for l := 1; l <= game.MaxLevel; l++ {
		out = append(out, Simulate(p, l, enchant, night))
	}
	return out
}

// TimeToMax: soma dos tempos de nível (segundos) do 1 ao máximo (game.MaxLevel).
func TimeToMax(rows []Row) float64 {
	t := 0.0
	for _, r := range rows {
		if r.Level < game.MaxLevel {
			t += r.LevelTime
		}
	}
	return t
}

// TimeToMaxFree: o mesmo sem o limite do renascimento.
func TimeToMaxFree(rows []Row) float64 {
	t := 0.0
	for _, r := range rows {
		if r.Level < game.MaxLevel {
			t += r.LevelTimeFree
		}
	}
	return t
}
