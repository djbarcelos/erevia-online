package zone

import (
	"math"

	"erevia/server/internal/game"
)

// XP por fonte (tunnel#147, docs/progressao.md). Uma linha por abate encheria a tabela,
// então a zona só soma num mapa (personagem → nível → fonte) e grava um evento por fonte a
// cada xpTallyEvery, ao subir de nível (a soma do nível anterior sai com o nível dele) e
// ao sair do jogo:
//
//	xp  ref = fonte (xpSourceNames), amount = XP (+ ganhou / − perdeu),
//	    level = nível em que a XP veio
//
// No nível máximo a XP não entra (grantXP), então também não é somada aqui.

type xpSource uint8

const (
	xpMonster  xpSource = iota // abate sozinho (sem os extras abaixo)
	xpParty                    // abate em grupo: a parte deste membro, com o bônus do grupo
	xpNight                    // o extra da noite (game.NightXP)
	xpEvent                    // o extra da XP em dobro (fim de semana ou GM, events.go)
	xpQuest                    // recompensa de missão
	xpInvasion                 // recompensa da defesa da cidade
	xpDeath                    // perdida ao morrer (negativa)
	xpSubclass                 // o extra do bônus Duas Sendas (subclass.go)
	numXPSources
)

var xpSourceNames = [numXPSources]string{"monster", "party", "night_bonus", "event_bonus", "quest", "invasion", "death_loss", "subclass_bonus"}

const xpTallyEvery = 60.0 // segundos de jogo entre as gravações

type xpTally struct {
	level int
	xp    [numXPSources]int64
}

// xpTallies é a soma de quem está online (só da goroutine da zona).
type xpTallies struct {
	by   map[uint32]*xpTally
	next float64 // z.now da próxima gravação
}

// tallyXP soma XP de uma fonte. Custo: uma busca no mapa (e nada de banco).
func (z *Zone) tallyXP(p *entity, src xpSource, xp int64) {
	if xp == 0 || p.pl == nil || z.eventSink == nil {
		return
	}
	t := z.xpTally.by[p.id]
	switch {
	case t == nil:
		if z.xpTally.by == nil {
			z.xpTally.by = map[uint32]*xpTally{}
		}
		t = &xpTally{level: p.level}
		z.xpTally.by[p.id] = t
	case t.level != p.level:
		z.flushXPTally(p, t) // subiu: o que veio no nível anterior fica no nível anterior
		t.level = p.level
	}
	t.xp[src] += xp
}

// grantXPFrom é o grantXP com a fonte anotada.
func (z *Zone) grantXPFrom(p *entity, src xpSource, xp int64) {
	if p.level < game.MaxLevel {
		z.tallyXP(p, src, xp)
	}
	z.grantXP(p, xp)
}

// tallyMonsterXP divide a XP de um abate entre as fontes: primeiro sai o extra da XP em
// dobro, depois o da noite; o resto é do abate (sozinho ou em grupo). A soma é a XP dada.
func (z *Zone) tallyMonsterXP(p *entity, xp int64, night bool, eventMult float64, party bool) {
	if p.level >= game.MaxLevel {
		return
	}
	ev := xp - int64(math.Round(float64(xp)/max(1, eventMult)))
	rest := xp - ev
	var nt int64
	if night {
		nt = rest - int64(math.Round(float64(rest)/game.NightXP))
	}
	src := xpMonster
	if party {
		src = xpParty
	}
	z.tallyXP(p, src, rest-nt)
	z.tallyXP(p, xpNight, nt)
	z.tallyXP(p, xpEvent, ev)
}

// flushXPTally manda para a telemetria o que o jogador acumulou e zera a soma.
func (z *Zone) flushXPTally(p *entity, t *xpTally) {
	at := z.cfg.Clock()
	for src, xp := range t.xp {
		if xp == 0 {
			continue
		}
		t.xp[src] = 0
		e := GameEvent{At: at, Kind: "xp", CharacterID: int64(p.id), AccountID: p.pl.accountID, Level: t.level,
			X: p.pos.X, Z: p.pos.Z, Ref: xpSourceNames[src], Amount: xp}
		select {
		case z.telemetry <- e:
		default:
			z.droppedEvents++
		}
	}
}

// flushXPTallies: a cada xpTallyEvery, grava a soma de todo mundo (sistema 1×/s).
func (z *Zone) flushXPTallies() {
	if z.now < z.xpTally.next || len(z.xpTally.by) == 0 {
		return
	}
	z.xpTally.next = z.now + xpTallyEvery
	for id, t := range z.xpTally.by {
		if p := z.players[id]; p != nil {
			z.flushXPTally(p, t)
		} else {
			delete(z.xpTally.by, id)
		}
	}
}

// endXPTally: o jogador saiu (ou a zona parou); grava o que faltava.
func (z *Zone) endXPTally(p *entity) {
	if t := z.xpTally.by[p.id]; t != nil {
		z.flushXPTally(p, t)
		delete(z.xpTally.by, p.id)
	}
}
