package game

import (
	"math"
	"math/rand/v2"
)

const (
	CritMult = 2.0
	// DeathXPLoss: fração da XP do nível perdida ao morrer. Do DeathLossLateLevel em diante
	// a perda cai para DeathXPLossLate: um nível do 30 ao 40 leva de 6 a 12 h de caça, e 4%
	// disso seriam até meia hora por morte.
	DeathXPLoss        = 0.04
	DeathXPLossLate    = 0.02
	DeathLossLateLevel = 30
	monsterXPExp       = 1.6
	freeLevelDiff      = 5 // até 5 níveis abaixo, XP cheia
)

// XPCurve é a versão da curva de XP. A XP fica salva no personagem (characters.xp) e o
// nível sai dela; quando a curva muda, quem foi salvo na versão antiga é convertido ao
// carregar (ConvertXP), para ficar no mesmo nível e na mesma fração dele.
//   - 1: 80·l^2,2 (×1,1 a ×2 do 20 ao 29), nível máximo 30;
//   - 2: xpTable, nível máximo 40 (tunnel#149).
const XPCurve = 2

// xpTable: XP para sair de cada nível (índice 0 = nível 1), da curva 2. Saiu dos tempos-
// alvo do tunnel#149 (1→10 em ~3 h, 10→20 em ~15 h, 20→30 em ~40 h e 30→40 em ~90 h, a
// 25 s por abate, com ~30% da XP vindo de missões): o tempo de cada nível cresce devagar
// e sem degrau, e a XP é esse tempo vezes a XP dos monstros que se caça no nível. Por isso
// ela salta no 20 (o golem vale 2,5× um esqueleto) e não no tempo. Arredondada em 3
// algarismos; o docs/progressao.md mostra os abates e as horas de cada nível.
var xpTable = [MaxLevel - 1]int64{
	43, 265, 1_060, 3_460, 10_300, 23_700, 39_300, 63_100, 98_600, // 1 → 10
	124_000, 158_000, 200_000, 265_000, 328_000, 403_000, 491_000, 594_000, 715_000, 857_000, // 10 → 20
	2_620_000, 3_080_000, 3_620_000, 4_230_000, 4_940_000, 5_740_000, 6_660_000, 7_710_000, 8_900_000, 10_300_000, // 20 → 30
	13_400_000, 15_300_000, 17_500_000, 20_700_000, 23_500_000, 27_700_000, 31_400_000, 35_500_000, 40_100_000, 45_200_000, // 30 → 40
}

// xpStarts: XP total no início de cada nível (índice 0 = nível 1), somada uma vez.
var xpStarts = func() [MaxLevel]int64 {
	var out [MaxLevel]int64
	for l := 1; l < MaxLevel; l++ {
		out[l] = out[l-1] + xpTable[l-1]
	}
	return out
}()

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
	if l < 1 || l >= MaxLevel {
		return 0
	}
	return xpTable[l-1]
}

// XPLevelStart é a XP total acumulada no início do nível l.
func XPLevelStart(l int) int64 {
	return xpStarts[min(max(l, 1), MaxLevel)-1]
}

// LevelForXP devolve o nível correspondente a uma XP total.
func LevelForXP(xp int64) int {
	l := 1
	for l < MaxLevel && xp >= XPLevelStart(l+1) {
		l++
	}
	return l
}

// Curva 1 (até o tunnel#149): só para converter a XP salva.
const (
	xpV1Max      = 30
	xpV1Base     = 80.0
	xpV1Exp      = 2.2
	xpV1LateFrom = 20
	xpV1LateStep = 0.10
)

// xpToNextV1 é o XPToNext da curva 1.
func xpToNextV1(l int) int64 {
	if l < 1 || l >= xpV1Max {
		return 0
	}
	xp := xpV1Base * math.Pow(float64(l), xpV1Exp)
	if l >= xpV1LateFrom {
		xp *= 1 + xpV1LateStep*float64(l-xpV1LateFrom+1)
	}
	return int64(math.Round(xp))
}

// xpLevelStartV1 é o XPLevelStart da curva 1.
func xpLevelStartV1(l int) int64 {
	var total int64
	for i := 1; i < l && i < xpV1Max; i++ {
		total += xpToNextV1(i)
	}
	return total
}

// ConvertXP leva a XP salva na curva from para a curva de hoje: o mesmo nível e a mesma
// fração dele. No 30 da curva 1 a barra estava cheia ou travada (era o máximo), então quem
// estava lá começa o 30 → 31 do zero. Roda uma vez por personagem, ao carregar.
func ConvertXP(xp int64, from int) int64 {
	if from >= XPCurve || xp <= 0 {
		return max(xp, 0)
	}
	l := 1
	for l < xpV1Max && xp >= xpLevelStartV1(l+1) {
		l++
	}
	if l >= xpV1Max {
		return XPLevelStart(xpV1Max)
	}
	into, span := xp-xpLevelStartV1(l), xpToNextV1(l)
	// into < span ≤ 263.873 e XPToNext ≤ 45,2 milhões: o produto cabe folgado em int64.
	return XPLevelStart(l) + into*XPToNext(l)/span
}

// XPRepair diz o que CheckSavedXP fez com a XP salva.
type XPRepair int

const (
	XPOK      XPRepair = iota // a XP cabe no nível salvo pela curva de hoje
	XPFromV1                  // era XP da curva 1 numa linha marcada na de hoje: convertida
	XPClamped                 // não cabia em nenhuma curva: presa no início do nível salvo
)

// CheckSavedXP confere a XP de uma linha já marcada na curva de hoje contra o nível salvo
// junto (characters.level). No deploy, o servidor antigo ainda grava quem estava nele com
// a XP da curva 1 (e o nível certo) quando o novo já converteu e marcou a linha; pela curva
// nova essa XP derrubaria o nível. Então:
//   - cabe no nível salvo pela curva de hoje: fica como está;
//   - cabe no nível salvo pela curva 1: converte de novo (ConvertXP);
//   - não cabe em nenhuma: fica no início do nível salvo.
//
// O nível salvo nunca desce. Onde as duas curvas se sobrepõem (níveis bem baixos), vale a
// de hoje.
func CheckSavedXP(xp int64, level int) (int64, XPRepair) {
	l := min(max(level, 1), MaxLevel)
	if xp >= XPLevelStart(l) && (l == MaxLevel || xp < XPLevelStart(l+1)) {
		return xp, XPOK
	}
	if l <= xpV1Max && xp >= xpLevelStartV1(l) && (l == xpV1Max || xp < xpLevelStartV1(l+1)) {
		return ConvertXP(xp, 1), XPFromV1
	}
	return XPLevelStart(l), XPClamped
}

// MonsterXP é a XP de um monstro, reduzida se o jogador for muito mais forte.
func MonsterXP(m *MonsterDef, monsterLevel, playerLevel int) int64 {
	xp := m.XPBase * math.Pow(float64(monsterLevel), monsterXPExp)
	if diff := playerLevel - monsterLevel; diff > freeLevelDiff {
		xp *= math.Max(0.1, 1-0.15*float64(diff-freeLevelDiff))
	}
	return int64(math.Max(1, math.Round(xp)))
}

// DeathLossShare: a fração da XP do nível (ou da barra de Maestria) que uma morte tira.
func DeathLossShare(level int) float64 {
	if level >= DeathLossLateLevel {
		return DeathXPLossLate
	}
	return DeathXPLoss
}

// XPAfterDeath aplica a perda por morte sem derrubar o nível.
func XPAfterDeath(xp int64, level int) int64 {
	loss := int64(math.Round(float64(XPToNext(level)) * DeathLossShare(level)))
	floor := XPLevelStart(level)
	if xp-loss < floor {
		return floor
	}
	return xp - loss
}

// ExpectedHit é o dano médio de um golpe (RollHit sem o sorteio): chance de acerto ×
// atk × mult × 100/(100+def) × (1 + chance de crítico × (multiplicador − 1)). O piso de 1
// vale para o golpe sem crítico, como em RollHit; o arredondamento não muda a média.
func ExpectedHit(in HitInput) float64 {
	hit := 1.0
	if !in.NoMiss && !in.ForceCrit {
		hit = HitChance(in.Accuracy, in.Evasion)
	}
	crit := math.Max(0, math.Min(1, in.Crit))
	if in.ForceCrit {
		crit = 1
	}
	cm := in.CritMult
	if cm <= 0 {
		cm = CritMult
	}
	base := math.Max(1, in.Atk*in.Mult*100/(100+math.Max(0, in.Def)))
	return hit * base * (1 + crit*(cm-1))
}
