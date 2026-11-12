package zone

import (
	"math"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

type actionKind uint8

const (
	actNone actionKind = iota
	actAttack
	actSkill
	actPickup // target = id do item no chão
)

// action é o que a entidade está fazendo: atacar um alvo ou usar uma skill nele.
type action struct {
	kind   actionKind
	target uint32
	skill  *game.SkillDef
}

type activeEffect struct {
	kind  game.EffectKind
	value float64
	until float64
}

// entity é um jogador ou monstro na zona. Só a goroutine da zona mexe nele.
type entity struct {
	id    uint32
	kind  pb.EntityKind
	name  string
	model string
	level int
	scale float64

	pos, dest Vec2
	path      []Vec2  // próximos pontos do caminho (depois de dest)
	repathAt  float64 // próximo recálculo de caminho permitido
	moved     bool    // destino/velocidade mudou: vai no próximo snapshot mesmo parado

	stats      game.Stats
	hp, mp     float64
	sentHP     int // último valor de vida enviado (só envia quando muda o inteiro)
	sentMaxHP  int
	dead       bool
	act        action
	nextAttack float64
	lastCombat float64
	effects    []activeEffect

	pl  *playerState  // só jogadores
	mon *monsterState // só monstros
	npc *npcState     // só NPCs
}

type playerState struct {
	peer       Peer
	class      string
	xp         int64
	cooldowns  map[uint32]float64 // skill → instante em que fica pronta
	moveTokens float64
	chatTokens float64
	statsDirty bool
	sentMP     int
	inv        inventory
	saveAt     float64 // salvar em breve (0 = só no periódico)
}

func (e *entity) alive() bool { return !e.dead }

func (e *entity) effect(kind game.EffectKind, now float64) (float64, bool) {
	for _, ef := range e.effects {
		if ef.kind == kind && ef.until > now {
			return ef.value, true
		}
	}
	return 0, false
}

func (e *entity) stunned(now float64) bool {
	_, ok := e.effect(game.EffectStun, now)
	return ok
}

func (e *entity) speed(now float64) float64 {
	s := e.stats.Speed
	if v, ok := e.effect(game.EffectBuffSpeed, now); ok {
		s *= 1 + v
	}
	if v, ok := e.effect(game.EffectSlow, now); ok {
		s *= 1 - v
	}
	return s
}

func (e *entity) atk(magic bool, now float64) float64 {
	a := e.stats.PAtk
	if magic {
		a = e.stats.MAtk
	}
	if v, ok := e.effect(game.EffectBuffAtk, now); ok {
		a *= 1 + v
	}
	return a
}

func (e *entity) def(magic bool, now float64) float64 {
	d := e.stats.PDef
	if magic {
		d = e.stats.MDef
	}
	if v, ok := e.effect(game.EffectBuffDef, now); ok {
		d *= 1 + v
	}
	return d
}

// addEffect aplica (ou renova) um efeito. Devolve true se mexe na velocidade.
func (e *entity) addEffect(def *game.EffectDef, now float64) {
	for i := range e.effects {
		if e.effects[i].kind == def.Kind {
			e.effects[i] = activeEffect{kind: def.Kind, value: def.Value, until: now + def.Duration}
			return
		}
	}
	e.effects = append(e.effects, activeEffect{kind: def.Kind, value: def.Value, until: now + def.Duration})
}

// pruneEffects remove efeitos vencidos. Devolve os tipos que expiraram.
func (e *entity) pruneEffects(now float64) (expired []game.EffectKind) {
	kept := e.effects[:0]
	for _, ef := range e.effects {
		if ef.until > now {
			kept = append(kept, ef)
		} else {
			expired = append(expired, ef.kind)
		}
	}
	e.effects = kept
	return expired
}

func (e *entity) hpInt() int    { return int(math.Ceil(e.hp)) }
func (e *entity) maxHPInt() int { return int(math.Round(e.stats.MaxHP)) }

func (e *entity) clearAction() { e.act = action{} }

// stopMoving faz a entidade parar onde está.
func (e *entity) stopMoving() {
	e.path = nil
	if e.dest != e.pos {
		e.dest = e.pos
		e.moved = true
	}
}

// setDest muda o destino; ignora mudanças mínimas para não inflar os snapshots.
func (e *entity) setDest(d Vec2) {
	if math.Hypot(d.X-e.dest.X, d.Z-e.dest.Z) < 0.3 {
		return
	}
	e.dest = d
	e.moved = true
}

func dist(a, b Vec2) float64 { return math.Hypot(a.X-b.X, a.Z-b.Z) }
