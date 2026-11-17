package zone

import (
	"context"
	"math"
	"slices"
	"sort"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// O que é comum a todo chefe com estado (raid boss e chefe de campo): o renascimento salvo
// no banco, a pontuação de dano por grupo, clã ou jogador, o saque reservado, o estado
// mostrado no mapa e os anúncios. O raid está em raid.go e o chefe de campo em fieldboss.go.

// BossData é o renascimento de um raid boss salvo no banco.
type BossData struct {
	ID        string    // id da def (ex.: "ozrhal")
	RespawnAt time.Time // quando renasce (passado = vivo)
	KilledAt  time.Time
	TopName   string // quem mais causou dano na última morte
}

// BossBackend grava o renascimento (implementado com o store).
type BossBackend interface {
	SaveBoss(ctx context.Context, b BossData) error
}

// SetBossBackend liga o banco dos chefes (antes de Run). Sem ele (testes), nada é salvo.
func (z *Zone) SetBossBackend(b BossBackend) { z.bossBackend = b }

// raidKey agrupa o dano: grupo, clã (quem está sem grupo) ou jogador sozinho.
type raidKey struct {
	kind byte // 'p' grupo, 'c' clã, 's' sozinho
	id   int64
}

func (z *Zone) raidKeyOf(p *entity) (raidKey, string) {
	switch {
	case p.pl.party != nil:
		return raidKey{kind: 'p', id: int64(p.pl.party.id)}, "grupo de " + z.leaderName(p)
	case p.pl.clan != nil:
		return raidKey{kind: 'c', id: p.pl.clan.id}, "clã " + p.pl.clan.name
	}
	return raidKey{kind: 's', id: int64(p.id)}, p.name
}

// leaderName: o nome do líder do grupo de p (ou o de p, se o líder não está aqui).
func (z *Zone) leaderName(p *entity) string {
	if l := z.players[p.pl.party.leader]; l != nil {
		return l.name
	}
	return p.name
}

type raidPool struct {
	key    raidKey
	damage float64
}

// raidPools: as pontuações em ordem de dano (empate: a menor chave).
func (r *raidState) pools() []raidPool {
	out := make([]raidPool, 0, len(r.damage))
	for k, v := range r.damage {
		out = append(out, raidPool{k, v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].damage != out[j].damage {
			return out[i].damage > out[j].damage
		}
		if out[i].key.kind != out[j].key.kind {
			return out[i].key.kind < out[j].key.kind
		}
		return out[i].key.id < out[j].key.id
	})
	return out
}

// poolMembers: quem está online numa pontuação (grupo, clã ou o próprio jogador).
func (z *Zone) poolMembers(k raidKey) []*entity {
	switch k.kind {
	case 'p':
		if pt := z.parties[uint32(k.id)]; pt != nil {
			return z.partyMembers(pt)
		}
	case 'c':
		var out []*entity
		for _, id := range z.sortedPlayerIDs() {
			if p := z.players[id]; p.pl.clan != nil && p.pl.clan.id == k.id {
				out = append(out, p)
			}
		}
		return out
	case 's':
		if p := z.players[uint32(k.id)]; p != nil {
			return []*entity{p}
		}
	}
	return nil
}

func (z *Zone) sortedPlayerIDs() []uint32 {
	ids := make([]uint32, 0, len(z.players))
	for id := range z.players {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// dropRaidLoot: o saque cai espalhado em volta do corpo, reservado por 60 s (15 s no chefe
// de campo).
func (z *Zone) dropRaidLoot(m *entity, owner uint32, clanID int64) {
	reserve := raidReserve
	if m.mon.def.FieldBoss != nil {
		reserve = reservedFor // o chefe de campo reserva como o abate comum (loot.go)
	}
	drops := game.RollLoot(m.mon.def, m.level, z.rng)
	added := &pb.GroundItemsAdded{}
	for _, d := range drops {
		a := z.rng.Float64() * 2 * math.Pi
		r := 0.8 + z.rng.Float64()*(raidLootSpread-0.8)
		pos := Vec2{X: m.pos.X + r*math.Cos(a), Z: m.pos.Z + r*math.Sin(a)}
		if !z.walkable(pos) {
			pos = m.pos
		}
		if g := z.placeItem(pos, d.Item, d.Count, 0, owner, reserve); g != nil {
			g.clan = clanID
			added.Items = append(added.Items, z.groundPB(g))
		}
	}
	if len(added.Items) > 0 {
		z.afterFlush(func() {
			z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_GroundAdded{GroundAdded: added}}, 0)
		})
	}
}

// announce: banner e linha dourada para o servidor inteiro.
func (z *Zone) announce(kind pb.AnnounceKind, title, text string) {
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Announce{Announce: &pb.Announce{Kind: kind, Title: title, Text: text}}}, 0)
}

func (z *Zone) bossStatusPB(m *entity) *pb.BossStatus {
	def := m.mon.def
	out := &pb.BossStatus{
		Id: m.id, Name: def.Name, Level: uint32(m.level),
		X: float32(m.mon.home.X), Z: float32(m.mon.home.Z),
		Raid: def.Raid, Alive: !m.dead,
	}
	if f := def.FieldBoss; f != nil {
		out.PhaseMarks = f.Marks() // a barra do chefe de campo marca as fases dele
	}
	if m.dead && !m.mon.windowStart.IsZero() {
		out.WindowStartMs, out.WindowEndMs = m.mon.windowStart.UnixMilli(), m.mon.windowEnd.UnixMilli()
	}
	if r := m.mon.raid; r != nil && !m.dead {
		out.Phase = uint32(r.phase)
		out.Enraged, out.FinalEnrage = r.enraged, r.final
		if r.fighting {
			out.FightStartMs = r.fightWall.UnixMilli()
		}
	}
	return out
}

// bossList: todos os chefes fixos (para o Welcome), em ordem de id.
func (z *Zone) bossList() []*pb.BossStatus {
	var out []*pb.BossStatus
	for _, m := range z.monsters {
		if m.mon.def.Boss && !m.mon.summon {
			out = append(out, z.bossStatusPB(m))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Id < out[j].Id })
	return out
}

func (z *Zone) bossChanged(m *entity) {
	if !m.mon.def.Boss || m.mon.summon {
		return
	}
	if r := m.mon.raid; r != nil {
		r.lastStatusPhase, r.lastStatusEnraged, r.lastStatusFinal, r.lastStatusFighting = r.phase, r.enraged, r.final, r.fighting
	}
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_BossStatus{BossStatus: z.bossStatusPB(m)}}, 0)
}

// saveBoss entrega o renascimento ao worker sem bloquear a zona.
func (z *Zone) saveBoss(b BossData) {
	select {
	case z.bossJobs <- b:
	default:
		z.log.Error("boss save queue full", "boss", b.ID)
	}
}

// runBossWorker grava os renascimentos em ordem, tentando de novo se o banco falhar.
func (z *Zone) runBossWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-z.bossJobs:
			if z.bossBackend == nil {
				continue
			}
			for attempt := 1; attempt <= 3; attempt++ {
				cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				err := z.bossBackend.SaveBoss(cctx, b)
				cancel()
				if err == nil {
					break
				}
				z.log.Error("save boss", "err", err, "boss", b.ID, "attempt", attempt)
				z.ReportDBError("boss_save", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(attempt) * time.Second):
				}
			}
		}
	}
}

// setWindow: a janela de renascimento mostrada no mapa (sem o minuto exato).
func (z *Zone) setWindow(m *entity, killed, spawn time.Time) {
	def := m.mon.def
	lo, hi := respawnSeconds(def)
	if killed.IsZero() || hi <= 0 {
		m.mon.windowStart, m.mon.windowEnd = spawn, spawn
		return
	}
	m.mon.windowStart = killed.Add(time.Duration(lo * float64(time.Second)))
	m.mon.windowEnd = killed.Add(time.Duration(hi * float64(time.Second)))
}

// summonAround chama n monstros def no nível level, num anel de raio radius em volta de m
// (o chefe do raid, o invocador e o que divide). São invocações: não renascem, somem ao
// morrer e voltam para a casa de m. Já chegam bravos com quem m odeia; o Spawn vai pela AOI.
func (z *Zone) summonAround(m *entity, def *game.MonsterDef, level, n int, radius float64) []uint32 {
	ids := make([]uint32, 0, n)
	for i := range n {
		a := 2 * math.Pi * float64(i) / float64(n)
		spot := z.freeSpot(Vec2{X: m.pos.X + radius*math.Cos(a), Z: m.pos.Z + radius*math.Sin(a)})
		z.summonSeq++
		id := summonIDBase + z.summonSeq
		g := &entity{
			id: id, kind: pb.EntityKind_ENTITY_KIND_MONSTER,
			name: def.Name, model: def.Model, level: level, scale: def.Scale,
			pos: spot, dest: spot,
			stats: game.MonsterStats(def, level),
			mon: &monsterState{
				def: def, spawn: game.SpawnPoint{Monster: def, Level: level, X: spot.X, Z: spot.Z},
				home: m.mon.home, hate: map[uint32]float64{}, summon: true,
			},
		}
		g.hp = g.stats.MaxHP
		g.sentHP, g.sentMaxHP = g.hpInt(), g.maxHPInt()
		for pid, v := range m.mon.hate {
			g.mon.hate[pid] = v
		}
		z.monsters[id] = g
		ids = append(ids, id)
		z.aoiDirty = true
	}
	return ids
}

// removeMonster apaga um monstro de vez (invocações).
func (z *Zone) removeMonster(g *entity) {
	delete(z.monsters, g.id) // Despawn pela AOI
	g.gone = true
	z.aoiDirty = true
	z.forgetTarget(g.id)
}

// leashOf: distância máxima do ponto de origem antes de desistir.
func leashOf(m *entity) float64 {
	if m.mon.leash > 0 {
		return m.mon.leash
	}
	if r := m.mon.def.LeashRadius; r > 0 {
		return r
	}
	return leashRange
}
