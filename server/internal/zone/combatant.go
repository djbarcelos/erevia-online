package zone

import (
	"math"
	"time"

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
	force  bool // atacar jogador "branco" (Ctrl + clique)
}

type activeEffect struct {
	kind     game.EffectKind
	value    float64
	until    float64
	source   uint32  // quem aplicou (venenos dão o crédito do abate; provocar aponta o alvo)
	nextTick float64 // venenos/queimaduras: próximo segundo de dano
}

// entity é um jogador ou monstro na zona. Só a goroutine da zona mexe nele.
type entity struct {
	// spawnSeq muda quando o monstro renasce: a AOI manda um Spawn novo a quem já o conhecia.
	spawnSeq uint32
	// gone: saiu do mundo (jogador saiu, invocação sumiu); a AOI manda o Despawn.
	gone bool
	// Estado e vida deste tick já codificados para o snapshot (só quando encTick == tick).
	encTick         uint32
	encState, encHP []byte
	id              uint32
	kind            pb.EntityKind
	name            string
	model           string
	level           int
	scale           float64

	pos, dest Vec2
	face      Vec2    // última direção em que andou (Piscar vai para lá)
	teleport  bool    // pulou de lugar neste tick (o snapshot avisa o cliente)
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
	obj *objectState  // só objetos (estandarte)
}

type playerState struct {
	peer       Peer
	class      string
	emblem     string // cor do emblema (#rrggbb)
	look       game.Appearance
	xp         int64
	cooldowns  map[uint32]float64 // skill → instante em que fica pronta
	moveTokens float64
	chatTokens float64
	statsDirty bool
	sentMP     int
	sentSelfHP int // vida no último SelfStats (o sentHP é do snapshot e muda antes)
	inv        inventory
	saveAt     float64 // salvar em breve (0 = só no periódico)

	// Progressão (Fase B): pontos de atributo gastos, ranks da árvore e barra de atalhos.
	attrs  game.Attrs
	ranks  map[uint32]int
	hotbar []string
	// Especialização (segunda classe, nível 20): id do caminho ("" = ainda não escolheu).
	spec string
	// Muralha Viva: quando pode disparar de novo. Sumir: o próximo golpe é crítico.
	lastStandReady float64
	ambushCrit     bool

	// Fase 4: PvP, grupo e clã.
	accountID  int64
	karma      int
	pks        int // jogadores "brancos" mortos
	pvpKills   int // vitórias contra quem estava em combate
	flagUntil  float64
	sentPvp    pb.PvpState
	killedBy   string // morto: nome de quem matou
	party      *party
	clan       *clan
	clanLeftAt time.Time     // saiu de um clã (espera para entrar em outro)
	channel    *channelState // canalizando teleporte ou pergaminho
	pathGoal   *Vec2         // caminho longo esperando na fila (ver pathqueue.go)

	// AOI: jogadores e monstros que este cliente conhece (e o spawnSeq do Spawn que recebeu).
	known map[uint32]knownEntity

	// Moderação.
	gm         bool
	mutedUntil time.Time // relógio de parede (zero = pode falar)
	userAgent  string
	nextBug    float64 // próximo /bug aceito (tempo de simulação)

	quests   questLog
	trade    *trade    // troca aberta (nil = nenhuma)
	joinedAt time.Time // telemetria: início da sessão

	social socialState // amigos, emote e leitura do correio
	// Conquistas, título escolhido e quando pode pedir o ranking de novo.
	ach         achievements
	rankingNext float64
	// Maestria (nível máximo): barra, pontos da árvore e a soma dos efeitos (ver mastery.go).
	mastery masteryState
	vit     vitalityState // vitalidade e recuperação (vitality.go)
}

// rank da skill (0 = não aprendida).
func (p *playerState) rank(id uint32) int { return p.ranks[id] }

func (e *entity) alive() bool { return !e.dead }

// bodyRadius: raio do corpo (m) para alcance de golpes e skills (0 = tamanho normal).
func (e *entity) bodyRadius() float64 {
	if e.mon != nil {
		return e.mon.def.BodyRadius
	}
	return 0
}

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

func (e *entity) has(kind game.EffectKind, now float64) bool {
	_, ok := e.effect(kind, now)
	return ok
}

// effectRef devolve o efeito ativo (para mudar o valor, ex.: escudo que vai acabando).
func (e *entity) effectRef(kind game.EffectKind, now float64) *activeEffect {
	for i := range e.effects {
		if e.effects[i].kind == kind && e.effects[i].until > now {
			return &e.effects[i]
		}
	}
	return nil
}

// rooted: preso no lugar (Prisão Glacial) ou atordoado.
func (e *entity) rooted(now float64) bool { return e.has(game.EffectRoot, now) || e.stunned(now) }

func (e *entity) invisible(now float64) bool { return e.has(game.EffectInvisible, now) }

// attackInterval: tempo entre ataques básicos (Frenesi acelera).
func (e *entity) attackInterval(now float64) float64 {
	iv := e.stats.AtkInterval
	if v, ok := e.effect(game.EffectBuffAtkSpeed, now); ok {
		iv /= 1 + v
	}
	return iv
}

// accuracy: precisão (Bomba de Fumaça cega: cada 1% de erro extra = 5 pontos).
func (e *entity) accuracy(now float64) float64 {
	a := e.stats.Accuracy
	if v, ok := e.effect(game.EffectBlind, now); ok {
		a -= v * 500
	}
	return a
}

func (e *entity) speed(now float64) float64 {
	s := e.stats.Speed
	if v, ok := e.effect(game.EffectBuffSpeed, now); ok {
		s *= 1 + v
	}
	if v, ok := e.effect(game.EffectSlow, now); ok {
		s *= 1 - v
	}
	// Passo Leve (Maestria): só fora de combate. O snapshot leva a velocidade a cada passo.
	if e.pl != nil && now-e.lastCombat > outOfCombatAfter {
		if v := e.pl.mastery.of(game.MStatCalmSpeed); v > 0 {
			s *= 1 + v
		}
	}
	return s
}

func (e *entity) atk(magic bool, now float64) float64 {
	a := e.stats.PAtk
	if magic {
		a = e.stats.MAtk
	}
	bonus := 0.0
	if v, ok := e.effect(game.EffectBuffAtk, now); ok {
		bonus += v
	}
	if v, ok := e.effect(game.EffectWarCry, now); ok {
		bonus += v
	}
	if v, ok := e.effect(game.EffectWeaken, now); ok {
		bonus -= v
	}
	return a * math.Max(0.1, 1+bonus)
}

func (e *entity) def(magic bool, now float64) float64 {
	d := e.stats.PDef
	if magic {
		d = e.stats.MDef
	}
	bonus := 0.0
	for _, k := range []game.EffectKind{game.EffectBuffDef, game.EffectBanner, game.EffectFrostArmor} {
		if v, ok := e.effect(k, now); ok {
			bonus += v
		}
	}
	return d * (1 + bonus)
}

// addEffect aplica (ou renova) um efeito; o mais novo do mesmo tipo substitui o antigo.
func (e *entity) addEffect(def *game.EffectDef, now float64, source uint32) {
	ef := activeEffect{kind: def.Kind, value: def.Value, until: now + def.Duration, source: source, nextTick: now + 1}
	for i := range e.effects {
		if e.effects[i].kind == def.Kind {
			e.effects[i] = ef
			return
		}
	}
	e.effects = append(e.effects, ef)
}

// removeEffect tira um efeito antes da hora. Devolve se havia.
func (e *entity) removeEffect(kind game.EffectKind) bool {
	for i := range e.effects {
		if e.effects[i].kind == kind {
			e.effects = append(e.effects[:i], e.effects[i+1:]...)
			return true
		}
	}
	return false
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

// Vida e mana como inteiros (snapshot, SelfStats, grupo, nível, renascer, salvamento).
// Uma regra só para todas as mensagens: o máximo é o valor arredondado (o mesmo que a
// janela do personagem mostra) e o valor atual nunca passa dele. A vida arredonda para
// cima (vivo com 0,3 aparece com 1, nunca 0) e a mana para baixo (o que aparece dá para
// gastar), mas mana cheia aparece cheia. Ex.: máximo 408,4 e vida cheia = 408 / 408.
func (e *entity) maxHPInt() int { return int(math.Round(e.stats.MaxHP)) }
func (e *entity) hpInt() int    { return min(max(0, int(math.Ceil(e.hp))), e.maxHPInt()) }
func (e *entity) maxMPInt() int { return int(math.Round(e.stats.MaxMP)) }
func (e *entity) mpInt() int {
	if e.mp >= e.stats.MaxMP {
		return e.maxMPInt()
	}
	return min(max(0, int(math.Floor(e.mp))), e.maxMPInt())
}

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
	if l := dist(e.pos, d); l > 0.1 {
		e.face = Vec2{X: (d.X - e.pos.X) / l, Z: (d.Z - e.pos.Z) / l}
	}
}

// teleportTo põe a entidade num ponto na hora (o cliente não interpola).
func (e *entity) teleportTo(p Vec2) {
	e.pos, e.dest, e.path = p, p, nil
	e.moved, e.teleport = true, true
}

func dist(a, b Vec2) float64 { return math.Hypot(a.X-b.X, a.Z-b.Z) }
