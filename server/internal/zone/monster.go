package zone

import (
	"math"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

const (
	monsterIDBase = uint32(1) << 31 // ids de monstro nunca colidem com ids de personagem
	aggroRange    = 8.0
	leashRange    = 25.0
	wakeRange     = 45.0 // monstro sem jogador por perto não vaga (economiza CPU e rede)
	wanderRadius  = 5.0
	corpseSeconds = 6.0
)

type monsterState struct {
	def        *game.MonsterDef
	spawn      game.SpawnPoint
	home       Vec2
	returning  bool
	hate       map[uint32]float64
	nextWander float64
	respawnAt  float64
	corpseEnd  float64
	hidden     bool // corpo já sumiu; aguardando respawn
	raid       *raidState // raid boss (nil nos outros)
	summon     bool       // invocação: não renasce e some ao morrer
	// Chefes mortos: janela de renascimento mostrada no mapa (relógio de parede).
	windowStart, windowEnd time.Time
}

func (z *Zone) spawnMonsters() {
	points := game.ExpandSpawns(z.cfg.Spawns, z.rng)
	for i, sp := range points {
		id := monsterIDBase + uint32(i)
		home := z.freeSpot(clampToZone(Vec2{X: sp.X, Z: sp.Z}, z.bounds()))
		m := &entity{
			id: id, kind: pb.EntityKind_ENTITY_KIND_MONSTER,
			name: sp.Monster.Name, model: sp.Monster.Model, level: sp.Level, scale: sp.Monster.Scale,
			pos: home, dest: home,
			stats: game.MonsterStats(sp.Monster, sp.Level),
			mon: &monsterState{
				def: sp.Monster, spawn: sp, home: home,
				hate: map[uint32]float64{}, nextWander: z.rng.Float64() * 5,
			},
		}
		m.hp = m.stats.MaxHP
		if sp.Monster.Raid {
			z.initRaid(m) // pode começar morto (renascimento salvo no banco)
		}
		m.sentHP, m.sentMaxHP = m.hpInt(), m.maxHPInt()
		z.monsters[id] = m
	}
}

// think é a IA do monstro: vagar → perseguir → atacar → desistir e voltar.
func (z *Zone) think(m *entity) {
	ms := m.mon
	if m.stunned(z.now) {
		return
	}
	// Raid boss: fases, habilidades e reinício rodam mesmo provocado (o tanque segura o
	// chefe, mas a luta continua).
	if ms.raid != nil && !ms.returning {
		z.pruneHate(m)
		if z.thinkRaid(m) {
			return
		}
	}
	// Provocado: só pode atacar quem provocou (enquanto ele estiver vivo e visível).
	if ef := m.effectRef(game.EffectTaunt, z.now); ef != nil {
		if t := z.players[ef.source]; t != nil && !t.dead && !t.invisible(z.now) {
			ms.returning = false
			if m.act.kind != actAttack || m.act.target != t.id {
				m.act = action{kind: actAttack, target: t.id}
			}
			return
		}
	}

	if ms.returning {
		if dist(m.pos, ms.home) < 0.5 {
			ms.returning = false
			m.hp = m.stats.MaxHP
		} else {
			z.moveToward(m, ms.home)
		}
		return
	}

	if dist(m.pos, ms.home) > leashOf(m) {
		z.resetMonster(m)
		return
	}

	z.pruneHate(m)

	if ms.def.Aggressive && len(ms.hate) == 0 {
		if p := z.nearestPlayer(m.pos, z.aggroRange()); p != nil {
			ms.hate[p.id] = 1
		}
	}

	if top := topHate(ms.hate); top != 0 {
		if m.act.kind == actNone || m.act.target != top {
			m.act = action{kind: actAttack, target: top}
		}
		return
	}

	if m.act.kind != actNone {
		m.clearAction()
	}
	if dist(m.pos, ms.home) > wanderRadius+1 {
		ms.returning = true
		z.moveToward(m, ms.home)
		return
	}
	if z.now >= ms.nextWander {
		ms.nextWander = z.now + 4 + z.rng.Float64()*6
		if z.nearestPlayer(m.pos, wakeRange) != nil {
			a := z.rng.Float64() * 2 * math.Pi
			r := z.rng.Float64() * wanderRadius
			spot := clampToZone(Vec2{X: ms.home.X + r*math.Cos(a), Z: ms.home.Z + r*math.Sin(a)}, z.bounds())
			if z.walkable(spot) && (z.cfg.World == nil || z.cfg.World.SegmentClear(toPt(m.pos), toPt(spot))) {
				m.setDest(spot)
			}
		}
	}
}

// pruneHate esquece quem morreu, saiu ou fugiu para a cidade.
func (z *Zone) pruneHate(m *entity) {
	for id := range m.mon.hate {
		p := z.players[id]
		if p == nil || p.dead || z.inSafe(p.pos) || p.invisible(z.now) {
			delete(m.mon.hate, id)
		}
	}
}

// resetMonster: desiste da luta, volta para casa e recupera a vida ao chegar.
func (z *Zone) resetMonster(m *entity) {
	if m.mon.raid != nil {
		z.raidReset(m)
	}
	m.mon.hate = map[uint32]float64{}
	m.mon.returning = true
	m.clearAction()
	m.repathAt = 0
	z.moveToward(m, m.mon.home)
}

// updateDeadMonster some com o corpo e faz o monstro renascer no ponto de origem.
func (z *Zone) updateDeadMonster(m *entity) {
	ms := m.mon
	if !ms.hidden && z.now >= ms.corpseEnd {
		ms.hidden = true
		z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Despawn{Despawn: &pb.Despawn{Id: m.id}}}, 0)
		if ms.summon {
			delete(z.monsters, m.id)
			return
		}
	}
	switch {
	case ms.summon:
	case ms.raid != nil:
		z.updateDeadRaid(m)
	case z.now >= ms.respawnAt:
		z.reviveMonster(m)
		z.bossChanged(m)
	}
}

// reviveMonster faz o monstro renascer no ponto de origem, inteiro.
func (z *Zone) reviveMonster(m *entity) {
	ms := m.mon
	m.dead = false
	ms.hidden = false
	ms.returning = false
	ms.hate = map[uint32]float64{}
	ms.windowStart, ms.windowEnd = time.Time{}, time.Time{}
	m.pos, m.dest, m.path = ms.home, ms.home, nil
	m.hp = m.stats.MaxHP
	m.effects = nil
	m.clearAction()
	m.sentHP, m.sentMaxHP = m.hpInt(), m.maxHPInt()
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Spawn{Spawn: &pb.Spawn{Entity: z.entityPB(m)}}}, 0)
}

func (z *Zone) nearestPlayer(from Vec2, maxDist float64) *entity {
	var best *entity
	bestD := maxDist
	for _, p := range z.players {
		if p.dead || z.inSafe(p.pos) || p.invisible(z.now) {
			continue
		}
		if d := dist(from, p.pos); d <= bestD {
			best, bestD = p, d
		}
	}
	return best
}

func topHate(h map[uint32]float64) uint32 {
	var best uint32
	bestV := -1.0
	for id, v := range h {
		if v > bestV || (v == bestV && id < best) {
			best, bestV = id, v
		}
	}
	return best
}
