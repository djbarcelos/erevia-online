package game

import (
	"math"
	"math/rand/v2"
)

const (
	CritMult      = 2.0
	DeathXPLoss   = 0.04 // fração da XP do nível perdida ao morrer
	xpCurveBase   = 80.0
	xpCurveExp    = 2.2
	monsterXPExp  = 1.6
	freeLevelDiff = 5 // até 5 níveis abaixo, XP cheia
)

// Hit é o resultado de um golpe.
type Hit struct {
	Damage float64
	Crit   bool
	Miss   bool
}

// HitInput é tudo o que decide um golpe.
type HitInput struct {
	Atk, Def, Mult    float64
	Crit, CritMult    float64 // chance e multiplicador do crítico (CritMult 0 = CritMult padrão)
	Accuracy, Evasion float64 // precisão de quem bate × esquiva de quem apanha
	NoMiss, ForceCrit bool
}

// HitChance: 85% com precisão = esquiva, +0,2% por ponto de diferença, entre 60% e 98%.
// Jogador e monstro do mesmo nível ficam perto de 95% (5% de erro, como antes).
func HitChance(acc, eva float64) float64 {
	return math.Max(0.6, math.Min(0.98, 0.85+(acc-eva)*0.002))
}

// RollHit calcula um golpe: atk × mult × 100/(100+def) × [0,9–1,1], com erro e crítico.
func RollHit(rng *rand.Rand, in HitInput) Hit {
	if !in.NoMiss && !in.ForceCrit && rng.Float64() >= HitChance(in.Accuracy, in.Evasion) {
		return Hit{Miss: true}
	}
	dmg := in.Atk * in.Mult * 100 / (100 + math.Max(0, in.Def)) * (0.9 + 0.2*rng.Float64())
	h := Hit{Damage: math.Max(1, dmg)}
	if in.ForceCrit || rng.Float64() < in.Crit {
		h.Crit = true
		cm := in.CritMult
		if cm <= 0 {
			cm = CritMult
		}
		h.Damage *= cm
	}
	h.Damage = math.Round(h.Damage)
	return h
}

// XPToNext é a XP necessária para sair do nível l para l+1.
func XPToNext(l int) int64 {
	if l >= MaxLevel {
		return 0
	}
	return int64(math.Round(xpCurveBase * math.Pow(float64(l), xpCurveExp)))
}

// XPLevelStart é a XP total acumulada no início do nível l.
func XPLevelStart(l int) int64 {
	var total int64
	for i := 1; i < l && i < MaxLevel; i++ {
		total += XPToNext(i)
	}
	return total
}

// LevelForXP devolve o nível correspondente a uma XP total.
func LevelForXP(xp int64) int {
	l := 1
	for l < MaxLevel && xp >= XPLevelStart(l+1) {
		l++
	}
	return l
}

// MonsterXP é a XP de um monstro, reduzida se o jogador for muito mais forte.
func MonsterXP(m *MonsterDef, monsterLevel, playerLevel int) int64 {
	xp := m.XPBase * math.Pow(float64(monsterLevel), monsterXPExp)
	if diff := playerLevel - monsterLevel; diff > freeLevelDiff {
		xp *= math.Max(0.1, 1-0.15*float64(diff-freeLevelDiff))
	}
	return int64(math.Max(1, math.Round(xp)))
}

// XPAfterDeath aplica a perda por morte sem derrubar o nível.
func XPAfterDeath(xp int64, level int) int64 {
	loss := int64(math.Round(float64(XPToNext(level)) * DeathXPLoss))
	floor := XPLevelStart(level)
	if xp-loss < floor {
		return floor
	}
	return xp - loss
}
