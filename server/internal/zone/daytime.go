package zone

import (
	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// updateDaytime acompanha o ciclo dia/noite, avisa todos quando ele vira e troca os
// spawns do dia pelos da noite (spawnperiod.go).
func (z *Zone) updateDaytime() {
	night := game.IsNight(game.DayPhase(z.cfg.Clock()))
	if night == z.night {
		if z.tick%uint32(z.cfg.TickRate) == 0 {
			z.updatePeriodSpawns() // quem lutava na virada sai quando a luta acaba
		}
		return
	}
	z.night = night
	z.updatePeriodSpawns() // os do dia saem e os da noite entram (ou o contrário)
	text := "O sol nasceu sobre Erevia."
	if night {
		text = "A noite caiu sobre Erevia… os monstros ficam mais fortes, mas dão mais experiência."
	}
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Chat{Chat: &pb.ChatMessage{
		Channel: pb.ChatChannel_CHAT_CHANNEL_SYSTEM, Text: text,
	}}}, 0)
	if night {
		z.nightInvasion() // em algumas noites, os esqueletos atacam a cidade
	}
}

// attackPower é o ataque usado no acerto (buffs + bônus noturno dos monstros).
func (z *Zone) attackPower(e *entity, magic bool) float64 {
	a := e.atk(magic, z.now)
	if e.mon != nil && z.night {
		a *= game.NightMonsterAtk
	}
	return a
}

// aggroRange é a distância em que monstros agressivos notam jogadores.
func (z *Zone) aggroRange() float64 {
	if z.night {
		return game.NightAggroRange
	}
	return aggroRange
}
