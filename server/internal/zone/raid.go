package zone

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Raid boss: um chefe para vários grupos, preso ao covil, com fases, avisos no chão
// (o golpe cai no fim do aviso, em quem ainda estiver dentro), invocações e fúria. A XP
// é dividida pelo dano de cada grupo; o saque fica reservado para quem mais causou dano.
// O renascimento (horas) é em relógio de parede e fica salvo no banco.
const (
	raidIdleReset   = 15.0 // segundos sem ninguém bater: o chefe volta ao trono
	raidMinShare    = 0.05 // fração mínima do dano para receber XP
	raidReserve     = 60.0 // segundos de saque reservado
	raidNear        = 60.0 // quem está a esta distância ouve o chefe
	raidWarnBefore  = 10 * time.Minute
	raidPhase2At    = 0.70
	raidPhase3At    = 0.40
	raidEnrageAt    = 0.20
	raidEnrageSpeed = 1.4  // fúria: ataques 40% mais rápidos
	raidFinalAfter  = 720. // 12 min de luta: fúria final
	raidFinalMult   = 3.0
	raidLootSpread  = 4.0

	// Os golpes de área (Onda, Chuva e Marca) são dados: game.MonsterDef.Abilities do Ozrhal.
	raidMarkRadius, raidMarkDelay, raidMarkMult = game.RaidMarkRadius, game.RaidMarkDelay, game.RaidMarkMult
	raidSummonsP2, raidSummonsP3                = 4, 6

	summonIDBase = monsterIDBase + 1<<20 // invocações (não colidem com os monstros fixos)
)

// Golpes do raid nos CombatEvent: o cliente mostra o dano sem animar o chefe batendo
// (o visual sai do Telegraph).
const (
	RaidHitWave = game.HitRaidWave
	RaidHitRain = game.HitRaidRain
	RaidHitMark = game.HitRaidMark
)

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

type raidState struct {
	phase               int
	fighting            bool
	fightStart, lastHit float64 // z.now
	fightWall           time.Time
	nextWave, nextRain  float64
	nextMark            float64
	enraged, final      bool
	summons             []uint32
	damage              map[raidKey]float64
	byPlayer            map[uint32]float64
	names               map[raidKey]string // nome para o anúncio (grupo de X, clã Y)
	total               float64
	spawnAt, killedAt   time.Time // morto: renasce em spawnAt (relógio)
	warned              bool
	lastStatusPhase     int
	lastStatusEnraged   bool
	lastStatusFinal     bool
	lastStatusFighting  bool
}

func newRaidState() *raidState {
	return &raidState{phase: 1, damage: map[raidKey]float64{}, byPlayer: map[uint32]float64{}, names: map[raidKey]string{}}
}

// clock: o relógio de cada golpe do Ozrhal (id da habilidade na def).
func (r *raidState) clock(id string) *float64 {
	switch id {
	case "wave":
		return &r.nextWave
	case "rain":
		return &r.nextRain
	case "mark":
		return &r.nextMark
	}
	return nil
}

// initRaid prepara o estado do raid boss e aplica o renascimento salvo.
func (z *Zone) initRaid(m *entity) {
	r := newRaidState()
	m.mon.raid = r
	for _, b := range z.cfg.Bosses {
		if b.ID != m.mon.def.ID {
			continue
		}
		if b.RespawnAt.After(z.cfg.Clock()) {
			m.dead, m.hp = true, 0
			m.mon.hidden = true
			r.spawnAt, r.killedAt = b.RespawnAt, b.KilledAt
			z.setWindow(m, b.KilledAt, b.RespawnAt)
		}
	}
}

// setWindow: a janela de renascimento mostrada no mapa (sem o minuto exato).
func (z *Zone) setWindow(m *entity, killed, spawn time.Time) {
	def := m.mon.def
	if killed.IsZero() || def.RespawnMax <= 0 {
		m.mon.windowStart, m.mon.windowEnd = spawn, spawn
		return
	}
	m.mon.windowStart = killed.Add(time.Duration(def.RespawnMin * float64(time.Second)))
	m.mon.windowEnd = killed.Add(time.Duration(def.RespawnMax * float64(time.Second)))
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

// thinkRaid roda dentro do think, depois de esquecer quem morreu ou fugiu. Devolve true
// se o chefe reiniciou (o think para por aqui).
func (z *Zone) thinkRaid(m *entity) bool {
	r := m.mon.raid
	if !r.fighting {
		if len(m.mon.hate) > 0 {
			z.raidEngage(m)
		}
		return false
	}
	// Todos morreram ou fugiram, ou ninguém bate há um tempo: volta ao trono.
	if len(m.mon.hate) == 0 || z.now-r.lastHit > raidIdleReset {
		z.resetMonster(m)
		return true
	}
	z.raidPhases(m)
	z.thinkAbilities(m)
	if !r.final && z.now-r.fightStart >= raidFinalAfter {
		r.final = true
		m.stats.PAtk *= raidFinalMult
		m.stats.MAtk *= raidFinalMult
		z.raidSay(m, fmt.Sprintf("%s perde a paciência: os golpes ficam devastadores!", m.name))
	}
	z.raidStatusIfChanged(m)
	return false
}

func (z *Zone) raidEngage(m *entity) {
	r := m.mon.raid
	r.fighting = true
	r.fightStart, r.lastHit = z.now, z.now
	r.fightWall = z.cfg.Clock()
	z.startAbilities(m)
	z.raidSay(m, fmt.Sprintf("%s se ergue do trono!", m.name))
	z.raidStatusIfChanged(m)
}

// raidPhases: troca de fase pela vida (com invocações) e fúria.
func (z *Zone) raidPhases(m *entity) {
	r := m.mon.raid
	frac := m.hp / m.stats.MaxHP
	if r.phase < 2 && frac <= raidPhase2At {
		r.phase = 2
		z.phaseAbilities(m, 2)
		z.summon(m, raidSummonsP2)
		z.raidSay(m, fmt.Sprintf("%s: \"Levantem-se, meus guardas!\"", m.name))
	}
	if r.phase < 3 && frac <= raidPhase3At {
		r.phase = 3
		z.phaseAbilities(m, 3)
		z.summon(m, raidSummonsP3)
		z.raidSay(m, fmt.Sprintf("%s: \"A morte marca os que me desafiam!\"", m.name))
	}
	if !r.enraged && frac <= raidEnrageAt {
		r.enraged = true
		m.stats.AtkInterval = m.mon.def.Interval / raidEnrageSpeed
		z.raidSay(m, fmt.Sprintf("%s entra em fúria!", m.name))
	}
}

// summon chama n Guardas Ossários em volta do chefe (não renascem).
func (z *Zone) summon(m *entity, n int) {
	def := game.Monsters["bone_guard"]
	if def == nil {
		return
	}
	level := max(1, m.level-5)
	for i := range n {
		a := 2 * math.Pi * float64(i) / float64(n)
		spot := z.freeSpot(Vec2{X: m.pos.X + 4*math.Cos(a), Z: m.pos.Z + 4*math.Sin(a)})
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
		// Já chegam bravos com quem está batendo no chefe.
		for pid, v := range m.mon.hate {
			g.mon.hate[pid] = v
		}
		z.monsters[id] = g
		m.mon.raid.summons = append(m.mon.raid.summons, id) // Spawn pela AOI
		z.aoiDirty = true
	}
}

// dismissSummons tira as invocações do chefe do mundo.
func (z *Zone) dismissSummons(m *entity) {
	for _, id := range m.mon.raid.summons {
		if g := z.monsters[id]; g != nil {
			z.removeMonster(g)
		}
	}
	m.mon.raid.summons = nil
}

// removeMonster apaga um monstro de vez (invocações).
func (z *Zone) removeMonster(g *entity) {
	delete(z.monsters, g.id) // Despawn pela AOI
	g.gone = true
	z.aoiDirty = true
	z.forgetTarget(g.id)
}

// raidReset: o chefe desistiu da luta. Volta à fase 1, sem fúria, sem invocações e sem a
// contagem de dano (o resetMonster leva de volta ao trono, onde a vida enche).
func (z *Zone) raidReset(m *entity) {
	r := m.mon.raid
	if r.fighting {
		z.raidSay(m, fmt.Sprintf("%s voltou ao trono.", m.name))
	}
	z.raidClear(m)
	z.raidStatusIfChanged(m)
}

func (z *Zone) raidClear(m *entity) {
	r := m.mon.raid
	z.dismissSummons(m)
	m.mon.stopAbilities() // cancela os avisos pendentes
	r.phase, r.fighting, r.fightStart, r.lastHit = 1, false, 0, 0
	r.fightWall = time.Time{}
	r.enraged, r.final = false, false
	clear(r.damage)
	clear(r.byPlayer)
	clear(r.names)
	r.total = 0
	m.stats = game.MonsterStats(m.mon.def, m.level) // desfaz a fúria
}

// raidCredit soma o dano de um jogador no chefe (antes da vida cair).
func (z *Zone) raidCredit(m, src *entity, amount float64) {
	r := m.mon.raid
	if r == nil || src == nil || src.pl == nil || amount <= 0 {
		return
	}
	r.lastHit = z.now
	k, name := z.raidKeyOf(src)
	r.damage[k] += amount
	r.names[k] = name
	r.byPlayer[src.id] += amount
	r.total += amount
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

// raidKilled: XP por dano, saque reservado para quem mais causou dano, anúncio e
// renascimento salvo.
func (z *Zone) raidKilled(m *entity) {
	r := m.mon.raid
	ms := m.mon
	pools := r.pools()

	// Saque: reservado para a pontuação com mais dano (o representante é quem dela mais bateu).
	var owner uint32
	var ownerClan int64
	topName := ""
	if len(pools) > 0 {
		win := pools[0].key
		topName = r.names[win]
		if win.kind == 'c' {
			ownerClan = win.id
		}
		best := -1.0
		for _, p := range z.poolMembers(win) {
			if v := r.byPlayer[p.id]; v > best {
				owner, best = p.id, v
			}
		}
	}
	z.dropRaidLoot(m, owner, ownerClan)

	// XP: cada pontuação com 5% ou mais do dano leva a parte dela.
	for _, pl := range pools {
		share := pl.damage / math.Max(1, r.total)
		members := z.poolMembers(pl.key)
		if share < raidMinShare {
			for _, p := range members {
				z.systemAfterFlush(p, fmt.Sprintf("Seu lado causou só %.0f%% do dano em %s: sem XP (mínimo de %.0f%%).", share*100, m.name, raidMinShare*100))
			}
			continue
		}
		takers := z.eligible(members, m.pos)
		for _, p := range takers {
			z.systemAfterFlush(p, fmt.Sprintf("Seu lado causou %.0f%% do dano em %s.", share*100, m.name))
		}
		z.shareMonsterXPScaled(m, takers, share)
	}
	// Conquista: todo mundo que causou dano e ainda está aqui participou.
	for id := range r.byPlayer {
		if p := z.players[id]; p != nil {
			z.achEvent(p, game.AchRaid, ms.def.ID)
			z.bestiaryKill(p, m) // Bestiário: 1 abate abre a entrada do raid boss
		}
	}

	text := fmt.Sprintf("%s foi derrotado!", m.name)
	if topName != "" {
		text += " Maior dano: " + topName + "."
	}
	z.afterFlush(func() { z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_DEFEAT, "Raid boss derrotado", text) })

	z.raidClear(m)
	wall := z.cfg.Clock()
	def := ms.def
	delay := def.RespawnMin + z.rng.Float64()*(def.RespawnMax-def.RespawnMin)
	r.killedAt = wall
	r.spawnAt = wall.Add(time.Duration(delay * float64(time.Second)))
	r.warned = false
	z.setWindow(m, wall, r.spawnAt)
	ms.hate = map[uint32]float64{}
	ms.corpseEnd = z.now + corpseSeconds*2
	z.saveBoss(BossData{ID: def.ID, RespawnAt: r.spawnAt, KilledAt: wall, TopName: topName})
	z.log.Info("raid boss killed", "boss", def.ID, "top", topName, "respawn_at", r.spawnAt)
	z.bossChanged(m)
}

// dropRaidLoot: o saque cai espalhado em volta do corpo, reservado por 60 s.
func (z *Zone) dropRaidLoot(m *entity, owner uint32, clanID int64) {
	drops := game.RollLoot(m.mon.def, m.level, z.rng)
	added := &pb.GroundItemsAdded{}
	for _, d := range drops {
		a := z.rng.Float64() * 2 * math.Pi
		r := 0.8 + z.rng.Float64()*(raidLootSpread-0.8)
		pos := Vec2{X: m.pos.X + r*math.Cos(a), Z: m.pos.Z + r*math.Sin(a)}
		if !z.walkable(pos) {
			pos = m.pos
		}
		if g := z.placeItem(pos, d.Item, d.Count, 0, owner, raidReserve); g != nil {
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

// updateDeadRaid: aviso 10 min antes e renascimento na hora marcada (relógio).
func (z *Zone) updateDeadRaid(m *entity) {
	r := m.mon.raid
	if !m.mon.hidden || r.spawnAt.IsZero() {
		return
	}
	wall := z.cfg.Clock()
	if !r.warned && !wall.Before(r.spawnAt.Add(-raidWarnBefore)) && wall.Before(r.spawnAt) {
		r.warned = true
		z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SOON, "O chão treme…",
			fmt.Sprintf("Algo se mexe na Cripta Esquecida. %s desperta em breve.", m.mon.def.Name))
	}
	if wall.Before(r.spawnAt) {
		return
	}
	r.spawnAt, r.warned = time.Time{}, false
	z.reviveMonster(m)
	z.announce(pb.AnnounceKind_ANNOUNCE_KIND_BOSS_SPAWN, "Raid boss",
		fmt.Sprintf("%s despertou na Cripta Esquecida!", m.mon.def.Name))
	z.log.Info("raid boss spawned", "boss", m.mon.def.ID)
	z.bossChanged(m)
}

// announce: banner e linha dourada para o servidor inteiro.
func (z *Zone) announce(kind pb.AnnounceKind, title, text string) {
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Announce{Announce: &pb.Announce{Kind: kind, Title: title, Text: text}}}, 0)
}

// raidSay: aviso do sistema para quem está perto do chefe.
func (z *Zone) raidSay(m *entity, text string) {
	for _, id := range z.sortedPlayerIDs() {
		if p := z.players[id]; dist(p.pos, m.pos) <= raidNear {
			z.system(p, text)
		}
	}
}

// ---------- Estado dos chefes (mapa e barra do raid) ----------

func (z *Zone) bossStatusPB(m *entity) *pb.BossStatus {
	def := m.mon.def
	out := &pb.BossStatus{
		Id: m.id, Name: def.Name, Level: uint32(m.level),
		X: float32(m.mon.home.X), Z: float32(m.mon.home.Z),
		Raid: def.Raid, Alive: !m.dead,
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

// raidStatusIfChanged envia o estado só quando a fase, a fúria ou a luta mudaram.
func (z *Zone) raidStatusIfChanged(m *entity) {
	r := m.mon.raid
	if r.phase != r.lastStatusPhase || r.enraged != r.lastStatusEnraged || r.final != r.lastStatusFinal || r.fighting != r.lastStatusFighting {
		z.bossChanged(m)
	}
}

// ---------- Banco ----------

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

// ---------- Comandos de teste (só com DEV_LOGIN, nunca em produção) ----------

// devRaid: /raid nascer | avisar | ir | vida N | matar | reiniciar
func (z *Zone) devRaid(p *entity, args []string) {
	var m *entity
	for _, e := range z.monsters {
		if e.mon.raid != nil {
			m = e
		}
	}
	if m == nil || len(args) == 0 {
		z.system(p, "Uso: /raid nascer | avisar | ir | vida N | matar | reiniciar")
		return
	}
	r := m.mon.raid
	switch args[0] {
	case "nascer":
		if !m.dead {
			z.system(p, m.name+" já está vivo.")
			return
		}
		m.mon.hidden = true
		r.spawnAt = z.cfg.Clock()
		r.warned = true
	case "avisar":
		if !m.dead {
			z.system(p, m.name+" está vivo.")
			return
		}
		m.mon.hidden = true
		r.spawnAt = z.cfg.Clock().Add(raidWarnBefore - time.Minute)
		r.warned = false
	case "ir":
		if p.dead {
			return
		}
		p.clearAction()
		p.teleportTo(z.freeSpot(Vec2{X: m.mon.home.X - 14, Z: m.mon.home.Z - 14}))
		z.forgetTarget(p.id)
	case "vida":
		if m.dead || len(args) < 2 {
			return
		}
		var pct float64
		if _, err := fmt.Sscanf(args[1], "%g", &pct); err != nil || pct <= 0 || pct > 100 {
			z.system(p, "Uso: /raid vida 1–100")
			return
		}
		m.hp = m.stats.MaxHP * pct / 100
	case "matar":
		if m.dead {
			return
		}
		m.mon.hate[p.id] += m.hp
		z.damage(p, m, m.hp+1, nil)
	case "reiniciar":
		if !m.dead {
			z.resetMonster(m)
		}
	default:
		z.system(p, "Uso: /raid nascer | avisar | ir | vida N | matar | reiniciar")
	}
}
