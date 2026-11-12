package zone

import (
	"math"

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
}

func (z *Zone) spawnMonsters() {
	points := game.ExpandSpawns(z.cfg.Spawns, z.rng)
	for i, sp := range points {
		id := monsterIDBase + uint32(i)
		home := z.freeSpot(clampToZone(Vec2{X: sp.X, Z: sp.Z}, z.cfg.Size/2))
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

	if ms.returning {
		if dist(m.pos, ms.home) < 0.5 {
			ms.returning = false
			m.hp = m.stats.MaxHP
		} else {
			z.moveToward(m, ms.home)
		}
		return
	}

	if dist(m.pos, ms.home) > leashRange {
		z.resetMonster(m)
		return
	}

	// Esquece quem morreu, saiu ou fugiu para a cidade.
	for id := range ms.hate {
		p := z.players[id]
		if p == nil || p.dead || z.inSafe(p.pos) {
			delete(ms.hate, id)
		}
	}

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
			spot := clampToZone(Vec2{X: ms.home.X + r*math.Cos(a), Z: ms.home.Z + r*math.Sin(a)}, z.cfg.Size/2)
			if z.walkable(spot) && (z.cfg.World == nil || z.cfg.World.SegmentClear(toPt(m.pos), toPt(spot))) {
				m.setDest(spot)
			}
		}
	}
}

// resetMonster: desiste da luta, volta para casa e recupera a vida ao chegar.
func (z *Zone) resetMonster(m *entity) {
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
	}
	if z.now >= ms.respawnAt {
		m.dead = false
		ms.hidden = false
		ms.returning = false
		ms.hate = map[uint32]float64{}
		m.pos, m.dest, m.path = ms.home, ms.home, nil
		m.hp = m.stats.MaxHP
		m.effects = nil
		m.clearAction()
		m.sentHP, m.sentMaxHP = m.hpInt(), m.maxHPInt()
		z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Spawn{Spawn: &pb.Spawn{Entity: z.entityPB(m)}}}, 0)
	}
}

func (z *Zone) nearestPlayer(from Vec2, maxDist float64) *entity {
	var best *entity
	bestD := maxDist
	for _, p := range z.players {
		if p.dead || z.inSafe(p.pos) {
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
