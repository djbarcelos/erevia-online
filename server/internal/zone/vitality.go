package zone

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Vitalidade e recuperação do 2º personagem (tunnel#150, regras em game/vitality.go). Tudo
// o que mexe na XP fica aqui, num multiplicador por jogador chamado no abate
// (monsterXPBonus) e na missão (questXPBonus): o caminho da XP (grantXP) não muda.
//
// Custo: no abate, umas contas; na cidade, um laço pelos jogadores a cada
// vitalityTownEvery segundos. O cliente só recebe o estado quando o % inteiro muda, ao
// subir de nível ou quando a vitalidade acaba.

const vitalityTownEvery = 10 // segundos entre os enchimentos na cidade

// vitalityState: a vitalidade do jogador em memória.
type vitalityState struct {
	stored  int64   // XP bônus guardada
	frac    float64 // o que encheu na cidade e ainda não fechou 1 de XP (níveis baixos)
	top     int     // maior nível da conta no login (a recuperação; lido uma vez, no store)
	catchup bool    // a recuperação valia na última conferência (para avisar quando acaba)
	sentPct int     // % inteiro do último estado mandado (-1 = nunca)
}

// vitalityOnJoin: enche o que passou fora do jogo, avisa e manda o estado. Pelo portão
// (hop) o estado vem da outra zona, sem tempo fora e sem repetir os avisos.
func (z *Zone) vitalityOnJoin(p *entity, v game.VitalityState, hop bool) {
	before := game.ClampVitality(v.Stored, p.level)
	p.pl.vit = vitalityState{
		stored:  game.VitalityOffline(v.Stored, p.level, v.LastSeen, z.cfg.Clock()),
		top:     v.AccountTop,
		catchup: game.CatchupActive(p.level, v.AccountTop),
		sentPct: -1,
	}
	st := &p.pl.vit
	if hop {
		z.sendVitality(p)
		return
	}
	if st.stored > 0 {
		msg := fmt.Sprintf("Vitalidade: %s.", pctText(game.VitalityPercent(st.stored, p.level)))
		if st.stored > before {
			msg = fmt.Sprintf("Você descansou: vitalidade em %s.", pctText(game.VitalityPercent(st.stored, p.level)))
		}
		z.system(p, msg+" "+vitalityWhat(p))
	}
	if st.catchup {
		z.system(p, fmt.Sprintf("Bônus de recuperação: +%d%% de XP de monstro e de missão até o nível %d (a conta já tem um personagem no %d).",
			int(math.Round((game.CatchupMult-1)*100)), st.top-game.CatchupGap, st.top))
	}
	z.sendVitality(p)
}

// vitalityWhat: o que a vitalidade faz, no nível do jogador.
func vitalityWhat(p *entity) string {
	if p.level >= game.MaxLevel {
		return "A XP de monstro na barra de Maestria sai em dobro até ela acabar."
	}
	return "A XP de monstro sai em dobro até ela acabar."
}

// pctText: 45 → "45%" (com vírgula abaixo de 1%).
func pctText(v float64) string {
	if v > 0 && v < 1 {
		return strings.Replace(fmt.Sprintf("%.1f%%", v), ".", ",", 1)
	}
	return fmt.Sprintf("%d%%", int(v))
}

func (z *Zone) vitalityPB(p *entity) *pb.VitalityState {
	v := &p.pl.vit
	out := &pb.VitalityState{
		Stored: uint64(v.stored), Cap: uint64(game.VitalityCap(p.level)), Mult: game.VitalityMult,
		Mastery:        p.level >= game.MaxLevel,
		OfflinePerHour: game.VitalityOfflinePerHour, TownPerHour: game.VitalityTownPerHour,
		MultCap: game.MonsterXPMultCap,
	}
	if v.catchup {
		out.CatchupMult, out.CatchupUntil = game.CatchupMult, uint32(v.top-game.CatchupGap)
	}
	return out
}

func (z *Zone) sendVitality(p *entity) {
	p.pl.vit.sentPct = int(game.VitalityPercent(p.pl.vit.stored, p.level))
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Vitality{Vitality: z.vitalityPB(p)}})
}

// vitalityDirty manda o estado se o % inteiro mudou.
func (z *Zone) vitalityDirty(p *entity) {
	v := &p.pl.vit
	if pct := int(game.VitalityPercent(v.stored, p.level)); pct != v.sentPct {
		z.sendVitality(p)
	}
}

// vitalityUsable: quanto da vitalidade pode pagar bônus agora. No nível máximo ela espera
// o bônus diário da Maestria (que já enche a barra mais rápido) e não gasta com a árvore
// cheia (a XP não entra).
func (z *Zone) vitalityUsable(p *entity) int64 {
	if p.level < game.MaxLevel {
		return p.pl.vit.stored
	}
	st := &p.pl.mastery.st
	if st.Points >= game.MasteryMaxPoints {
		return 0
	}
	st.Refresh(z.masteryDay())
	if st.Rested > 0 {
		return 0
	}
	return p.pl.vit.stored
}

// monsterXPBonus: a XP de um abate (já com grupo, noite e evento) mais a recuperação e a
// vitalidade, com o teto de game.MonsterXPMultCap. O extra vai para a telemetria (fontes
// vitality e catchup) e o da vitalidade sai do estoque.
func (z *Zone) monsterXPBonus(p *entity, xp int64) int64 {
	mult := z.xpEventMult()
	if z.night {
		mult *= game.NightXP
	}
	v := &p.pl.vit
	cu, vit := game.MonsterXPExtras(xp, mult, v.catchup, z.vitalityUsable(p))
	if p.level < game.MaxLevel {
		z.tallyXP(p, xpCatchup, cu)
		z.tallyXP(p, xpVitality, vit)
	}
	if vit > 0 {
		v.stored -= vit
		if v.stored > 0 {
			z.vitalityDirty(p)
		} else {
			v.stored = 0
			z.sendVitality(p)
			z.systemAfterFlush(p, "Sua vitalidade acabou: a XP de monstro volta ao normal. Ela enche fora do jogo e parado na cidade.")
		}
	}
	return xp + cu + vit
}

// questXPBonus: o extra da recuperação numa recompensa de missão (a vitalidade não conta).
func (z *Zone) questXPBonus(p *entity, xp int64) int64 {
	cu := game.QuestXPCatchup(xp, p.pl.vit.catchup)
	if p.level < game.MaxLevel {
		z.tallyXP(p, xpCatchup, cu)
	}
	return cu
}

// vitalityLevelUp: subiu de nível. O estoque fica no teto novo e a recuperação pode acabar.
func (z *Zone) vitalityLevelUp(p *entity) {
	v := &p.pl.vit
	v.stored = game.ClampVitality(v.stored, p.level)
	if on := game.CatchupActive(p.level, v.top); on != v.catchup {
		v.catchup = on
		if !on {
			z.systemAfterFlush(p, fmt.Sprintf("Bônus de recuperação encerrado: você chegou a %d níveis do seu personagem mais alto.", game.CatchupGap))
		}
	}
	z.sendVitality(p) // o teto mudou
}

// updateVitalityTown: a cada vitalityTownEvery segundos, enche um pouco a de quem está na
// cidade de verdade (área segura sem invasão), vivo e fora de combate. Só na zona que tem
// a cidade (a Clareira de Testes não tem).
func (z *Zone) updateVitalityTown() {
	if z.cfg.NoTown {
		return
	}
	for _, p := range z.players {
		if p.dead || !z.pveSafe(p.pos) || z.now-p.lastCombat <= outOfCombatAfter {
			continue
		}
		v := &p.pl.vit
		c := game.VitalityCap(p.level)
		if v.stored >= c {
			continue
		}
		v.frac += float64(c) * game.VitalityTownPerHour * vitalityTownEvery / 3600
		whole := int64(v.frac)
		v.frac -= float64(whole)
		v.stored = min(c, v.stored+whole)
		z.vitalityDirty(p)
	}
}

// gmVitality: /vitalidade N [Nome] põe a vitalidade em N% do teto (0 a 100).
func (z *Zone) gmVitality(gm *entity, args []string) {
	if len(args) < 1 {
		z.system(gm, "Uso: /vitalidade N [Nome] (N de 0 a 100, em % do teto)")
		return
	}
	n, err := strconv.ParseFloat(strings.Replace(args[0], ",", ".", 1), 64)
	if err != nil || math.IsNaN(n) || n < 0 || n > 100 {
		z.system(gm, "Porcentagem inválida (0 a 100).")
		return
	}
	t := gm
	if len(args) > 1 {
		if t = z.online(gm, args[1]); t == nil {
			return
		}
	}
	t.pl.vit.stored = int64(math.Round(float64(game.VitalityCap(t.level)) * n / 100))
	t.pl.vit.frac = 0
	z.sendVitality(t)
	z.system(gm, fmt.Sprintf("%s: vitalidade em %s (%d de XP).", t.name, pctText(game.VitalityPercent(t.pl.vit.stored, t.level)), t.pl.vit.stored))
	z.gmLog(gm, "vitalidade", t.name, args[0])
}
