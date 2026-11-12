package game

import (
	"math"
	"math/rand/v2"
)

const (
	MissChance    = 0.05
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

// RollHit calcula um golpe: atk × mult × 100/(100+def) × [0,9–1,1], com erro e crítico.
func RollHit(rng *rand.Rand, atk, def, mult, crit float64) Hit {
	if rng.Float64() < MissChance {
		return Hit{Miss: true}
	}
	dmg := atk * mult * 100 / (100 + math.Max(0, def)) * (0.9 + 0.2*rng.Float64())
	h := Hit{Damage: math.Max(1, dmg)}
	if rng.Float64() < crit {
		h.Crit = true
		h.Damage *= CritMult
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
