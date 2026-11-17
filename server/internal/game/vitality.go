package game

import (
	"math"
	"time"
)

// Vitalidade (tunnel#150), como a do Lineage 2 e o Rested XP do WoW: um estoque de XP
// bônus que enche fora do jogo (e devagar parado na cidade) e que dobra a XP de monstro
// até acabar. O bônus pago sai do estoque. Premia voltar ao jogo, não ficar horas ligado:
// quem joga 1 h por dia caça quase sempre com a XP de monstro em dobro.
//
//   - O estoque é XP. O teto é ~1 nível da faixa atual (VitalityCap); a barra do HUD mostra
//     estoque/teto de 0 a 100%. Ao subir de nível, o estoque fica limitado ao teto novo.
//   - Enche VitalityOfflinePerHour do teto por hora fora do jogo (cheia em ~67 h, ~3 dias),
//     calculado no login a partir da última saída, e VitalityTownPerHour por hora online
//     na cidade, fora de combate.
//   - Só a XP de monstro dobra e gasta. Missão, contrato, invasão e o bônus diário da
//     Maestria não dobram nem gastam.
//   - No nível máximo, vale para a barra de Maestria (o teto é uma barra, MasteryPointXP).
//
// Recuperação do 2º personagem: se a conta já tem um personagem no nível máximo, os outros
// ganham CatchupMult na XP de monstro e de missão até ficarem CatchupGap níveis abaixo do
// maior nível da conta.
//
// Teto: a XP de monstro nunca passa de MonsterXPMultCap× a do abate puro, somando noite, XP
// em dobro do evento, recuperação e vitalidade (MonsterXPExtras).

const (
	// VitalityMult: a XP de monstro sai em dobro enquanto houver vitalidade.
	VitalityMult = 2.0
	// VitalityOfflinePerHour: fração do teto que enche por hora fora do jogo.
	VitalityOfflinePerHour = 0.015
	// VitalityTownPerHour: fração do teto por hora online na cidade, sem combate.
	VitalityTownPerHour = 0.015
	// CatchupMult: o bônus de recuperação (+50%).
	CatchupMult = 1.5
	// CatchupGap: a recuperação vale até o personagem ficar a este tanto de níveis do maior
	// nível da conta.
	CatchupGap = 5
	// MonsterXPMultCap: teto do multiplicador da XP de monstro (noite × evento, recuperação
	// e vitalidade somados).
	MonsterXPMultCap = 3.0
)

// VitalityState é o que o login traz do banco.
type VitalityState struct {
	Stored     int64     // XP bônus guardada
	LastSeen   time.Time // última vez que o personagem foi salvo online (zero = nunca)
	AccountTop int       // maior nível entre os personagens da conta (a recuperação)
}

// VitalityCap: o teto da vitalidade no nível (XPToNext; no máximo, uma barra de Maestria).
func VitalityCap(level int) int64 {
	if level >= MaxLevel {
		return MasteryPointXP
	}
	return XPToNext(max(level, 1))
}

// ClampVitality limita o estoque a [0, teto do nível].
func ClampVitality(stored int64, level int) int64 {
	return min(max(stored, 0), VitalityCap(level))
}

// VitalityFill: o estoque depois de hours horas enchendo perHour do teto por hora.
func VitalityFill(stored int64, level int, hours, perHour float64) int64 {
	c := VitalityCap(level)
	stored = ClampVitality(stored, level)
	if !(hours > 0) || perHour <= 0 { // !(>) também pega NaN
		return stored
	}
	add := math.Min(float64(c), float64(c)*perHour*hours)
	return min(c, stored+int64(add))
}

// VitalityOffline: o estoque no login, a partir da última saída.
func VitalityOffline(stored int64, level int, lastSeen, now time.Time) int64 {
	if lastSeen.IsZero() {
		return ClampVitality(stored, level)
	}
	return VitalityFill(stored, level, now.Sub(lastSeen).Hours(), VitalityOfflinePerHour)
}

// VitalityPercent: o estoque em % do teto (0 a 100).
func VitalityPercent(stored int64, level int) float64 {
	c := VitalityCap(level)
	if c <= 0 {
		return 0
	}
	return 100 * float64(ClampVitality(stored, level)) / float64(c)
}

// CatchupActive: a recuperação vale no nível (a conta tem alguém no máximo e este ainda
// está a mais de CatchupGap níveis dele).
func CatchupActive(level, accountTop int) bool {
	return accountTop >= MaxLevel && level < accountTop-CatchupGap
}

// MonsterXPExtras divide o que a recuperação e a vitalidade somam a um abate. xp é a XP do
// abate já com noite e evento (mult = noite × evento); a base é xp/mult. A recuperação dá
// +50% da base e a vitalidade, +100% (até o que houver no estoque), e o total fica em no
// máximo MonsterXPMultCap× a base. A recuperação entra primeiro: o teto corta o que sobra
// da vitalidade, que então gasta menos.
func MonsterXPExtras(xp int64, mult float64, catchup bool, vitality int64) (cu, vit int64) {
	if xp <= 0 {
		return 0, 0
	}
	base := float64(xp) / math.Max(1, mult)
	room := math.Max(0, MonsterXPMultCap*base-float64(xp))
	if catchup {
		cu = int64(math.Round(math.Min(room, base*(CatchupMult-1))))
		room = math.Max(0, room-float64(cu))
	}
	if vitality > 0 {
		vit = min(vitality, int64(math.Round(math.Min(room, base*(VitalityMult-1)))))
	}
	return cu, vit
}

// QuestXPCatchup: o extra da recuperação numa recompensa de missão.
func QuestXPCatchup(xp int64, catchup bool) int64 {
	if !catchup || xp <= 0 {
		return 0
	}
	return int64(math.Round(float64(xp) * (CatchupMult - 1)))
}
