package zone

import (
	"fmt"
	"slices"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Chefe de campo (tunnel#132): o chefe de uma área usa o mesmo estado do raid (raidState,
// raid.go): dano por grupo, avisos no chão, fases, saque reservado e renascimento salvo.
// Muda o que é dado em game.FieldBossDef: o renascimento em minutos com janela, as fases
// por vida (com invocações que voltam a curar) e o aviso de 5 min. Ao morrer, deixa um
// túmulo no chão por 10 min.
const (
	fieldWarnBefore = 5 * time.Minute
	tombSeconds     = 600.0 // o túmulo dura 10 min
	tombModel       = "tomb"
)

// respawnSeconds: o renascimento em segundos de relógio (minutos no chefe de campo, os
// segundos que o raid já usava).
func respawnSeconds(def *game.MonsterDef) (lo, hi float64) {
	if f := def.FieldBoss; f != nil {
		return f.RespawnMin * 60, f.RespawnMax * 60
	}
	return def.RespawnMin, def.RespawnMax
}

// fieldPhases: troca de fase pela vida, e a volta das invocações.
func (z *Zone) fieldPhases(m *entity) {
	r := m.mon.raid
	f := m.mon.def.FieldBoss
	frac := m.hp / m.stats.MaxHP
	for r.phase < f.PhaseCount() && frac <= f.Phases[r.phase-1].AtHP {
		p := f.Phases[r.phase-1]
		r.phase++
		z.phaseAbilities(m, r.phase)
		if p.Say != "" {
			z.raidSay(m, p.Say)
		}
		if p.Enrage && !r.enraged {
			r.enraged = true
			m.stats.AtkInterval = m.mon.def.Interval / game.FieldBossEnrageSpeed
		}
		if p.Summon != "" {
			z.fieldSummon(m, p)
		}
	}
	if r.retAt > 0 && z.now >= r.retAt {
		z.fieldReturn(m)
	}
}

// fieldSummon chama as invocações da fase em volta do chefe.
func (z *Zone) fieldSummon(m *entity, p game.FieldPhase) {
	def := game.Monsters[p.Summon]
	if def == nil {
		return
	}
	r := m.mon.raid
	ids := z.summonAround(m, def, max(1, m.level-3), p.SummonCount, 4)
	r.summons = append(r.summons, ids...)
	if p.ReturnAfter > 0 {
		r.retAt, r.retHeal, r.retCount = z.now+p.ReturnAfter, p.ReturnHeal, len(ids)
	}
}

// fieldReturn: as invocações que sobraram voltam para o chefe e o curam, cada uma com a
// sua parte da cura da fase. Quem matou todas antes não deixa nada para curar.
func (z *Zone) fieldReturn(m *entity) {
	r := m.mon.raid
	left := 0
	for _, id := range slices.Clone(r.summons) {
		if g := z.monsters[id]; g != nil && !g.dead {
			left++
			z.removeMonster(g)
		}
	}
	r.summons = nil
	heal := r.retHeal * float64(left) / float64(max(1, r.retCount))
	r.retAt, r.retHeal, r.retCount = 0, 0, 0
	if left == 0 {
		return
	}
	m.hp = min(m.stats.MaxHP, m.hp+m.stats.MaxHP*heal)
	z.raidSay(m, fmt.Sprintf("%d das crias de %s voltam para ele e o curam em %.0f%%!", left, m.name, heal*100))
}

// updateDeadField: o aviso de 5 min (para o mapa da zona) e o renascimento na hora marcada.
func (z *Zone) updateDeadField(m *entity, f *game.FieldBossDef, wall time.Time) {
	r := m.mon.raid
	if !r.warned && !wall.Before(r.spawnAt.Add(-fieldWarnBefore)) && wall.Before(r.spawnAt) {
		r.warned = true
		z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SOON, "Um chefe desperta em breve",
			fmt.Sprintf("%s desperta %s em 5 minutos.", m.mon.def.Name, f.Place))
	}
	if wall.Before(r.spawnAt) {
		return
	}
	r.spawnAt, r.warned = time.Time{}, false
	z.reviveMonster(m)
	z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SPAWN, "Chefe de campo",
		fmt.Sprintf("%s despertou %s!", m.mon.def.Name, f.Place))
	z.log.Info("field boss spawned", "boss", m.mon.def.ID)
	z.bossChanged(m)
}

// spawnTomb deixa o túmulo no corpo do chefe por 10 min: um objeto clicável com a hora da
// morte e o nome de quem mais causou dano (a dica sai do título).
func (z *Zone) spawnTomb(m *entity, at time.Time, top string) {
	r := m.mon.raid
	if old := z.objects[r.tomb]; old != nil {
		delete(z.objects, r.tomb)
		z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Despawn{Despawn: &pb.Despawn{Id: r.tomb}}}, 0)
	}
	title := "Caiu às " + at.In(brasilia).Format("15:04")
	if top != "" {
		title += " · maior dano: " + top
	}
	z.objectSeq++
	id := objectIDBase + z.objectSeq
	e := &entity{
		id: id, kind: pb.EntityKind_ENTITY_KIND_OBJECT,
		name: "Túmulo de " + m.mon.def.Name, model: tombModel, level: m.level, scale: 1,
		pos: m.pos, dest: m.pos,
		stats: game.Stats{MaxHP: 1},
		obj:   &objectState{until: z.now + tombSeconds, title: title},
	}
	e.hp = 1
	e.sentHP, e.sentMaxHP = 1, 1
	z.objects[id] = e
	r.tomb = id
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Spawn{Spawn: &pb.Spawn{Entity: z.entityPB(e)}}}, 0)
}
