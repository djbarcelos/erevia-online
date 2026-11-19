package zone

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sync"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/world"
)

// Arena PvP ranqueada (tunnel#81). As regras e o Elo estão em game/arena.go.
//
//   - Fila: no Mestre da Arena, na cidade. 1x1 sozinho; 3x3 é o líder de um grupo de 3 (os
//     três vão juntos). A fila é do processo (Manager.arena): pega gente de todos os canais.
//   - Partida: os dois primeiros times da fila do modo (por ordem de chegada) viram uma cópia
//     do mapa "arena", com o mesmo gerente das masmorras (instance.go). Cada time aparece num
//     lado. Preparação (game.ArenaCountdown), luta (game.ArenaFight) e resultado.
//   - Na arena só o time é aliado; o resto é adversário, sem karma, flag, perda de XP nem
//     itens caídos. Morreu, fica fora até o fim (sem "Voltar à cidade"). Saiu do jogo no meio,
//     conta como fora.
//   - Fim: vence quem derrubar o outro time; no fim do tempo, quem tiver mais vida (soma das
//     frações de vida de quem está de pé); igual = empate (a pontuação não muda). O banco
//     aplica o Elo (ArenaBackend.RecordArenaMatch) e todos voltam à cidade, ao lado do Mestre,
//     com vida e mana cheias.

// ArenaMap é o id do mapa da arena (world/maps/arena.json).
const ArenaMap = "arena"

const (
	arenaRange      = 6.0              // distância do Mestre para abrir a janela
	arenaEmptyTTL   = 10 * time.Second // a cópia vazia (todos voltaram) fecha logo
	arenaTeamSpread = 2.5              // metros entre os membros do time na chegada
	arenaSideX      = 9.0              // cada time começa a esta distância do centro
)

// arenaMasterSpot: onde fica o Mestre da Arena, a sudoeste da praça, longe dos outros NPCs e do
// mercado da loja pessoal (e onde quem sai da arena aparece).
var arenaMasterSpot = Vec2{X: -11.5, Z: -9.6}

// ArenaBackend é o banco da arena (cmd/zone liga o store).
type ArenaBackend interface {
	RecordArenaMatch(ctx context.Context, r ArenaResult) ([]ArenaChange, error)
	ArenaRanking(ctx context.Context, season int, mode game.ArenaMode, limit int) ([]ArenaRank, error)
	ArenaRatingsOf(ctx context.Context, season int, characterID uint32) (map[game.ArenaMode]ArenaRecord, error)
}

// ArenaResult é uma partida terminada (Winner 0 ou 1).
type ArenaResult struct {
	Season int
	Mode   game.ArenaMode
	Teams  [2][]uint32
	Winner int
}

// ArenaChange é a pontuação nova de quem lutou.
type ArenaChange struct {
	ID            uint32
	Before, After int
}

// ArenaRank é uma linha do ranking.
type ArenaRank struct {
	Name, Class                 string
	Level, Rating, Wins, Losses int
}

// ArenaRecord é a pontuação de um personagem num modo.
type ArenaRecord struct{ Rating, Wins, Losses int }

// SetArenaBackend liga o banco da arena (antes de Run). Sem ele, a fila funciona e a
// pontuação não é gravada (testes).
func (z *Zone) SetArenaBackend(b ArenaBackend) {
	z.arenaBackend = b
	if z.arenaJobs == nil {
		z.arenaJobs = make(chan arenaJob, 64)
	}
}

// arenaJob roda no worker do banco e devolve o que fazer na zona (nil = nada).
type arenaJob func(ctx context.Context, b ArenaBackend) func()

func (z *Zone) runArenaWorker(ctx context.Context) {
	if z.arenaBackend == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-z.arenaJobs:
			if fn := job(ctx, z.arenaBackend); fn != nil {
				z.post(ctx, socialDone{fn: fn})
			}
		}
	}
}

// queueArenaJob entrega o trabalho ao worker sem travar a zona (false = sem banco ou fila cheia).
func (z *Zone) queueArenaJob(job arenaJob) bool {
	if z.arenaBackend == nil {
		return false
	}
	select {
	case z.arenaJobs <- job:
		return true
	default:
		z.log.Warn("arena job queue full")
		return false
	}
}

// ---------- A fila (do processo) ----------

// arenaEntry é um time na fila.
type arenaEntry struct {
	mode    game.ArenaMode
	members []uint32
	names   []string
	since   time.Time
}

// arenaQueue é a fila de todos os canais (Manager.arena).
type arenaQueue struct {
	mu      sync.Mutex
	entries []arenaEntry
	matches int
}

// queuedMode: o modo em que o personagem está na fila (0 = nenhum).
func (q *arenaQueue) queuedMode(id uint32) game.ArenaMode {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, e := range q.entries {
		if slices.Contains(e.members, id) {
			return e.mode
		}
	}
	return 0
}

// size: quantos times esperam no modo.
func (q *arenaQueue) size(mode game.ArenaMode) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for _, e := range q.entries {
		if e.mode == mode {
			n++
		}
	}
	return n
}

// add põe o time na fila ("" = entrou; senão o motivo).
func (q *arenaQueue) add(e arenaEntry) string {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, o := range q.entries {
		for _, id := range e.members {
			if slices.Contains(o.members, id) {
				return "Alguém do time já está na fila da arena."
			}
		}
	}
	q.entries = append(q.entries, e)
	return ""
}

// remove tira da fila o time em que o personagem está (true = estava).
func (q *arenaQueue) remove(id uint32) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, e := range q.entries {
		if slices.Contains(e.members, id) {
			q.entries = slices.Delete(q.entries, i, i+1)
			return true
		}
	}
	return false
}

// takePair tira da fila os dois primeiros times do modo (false = ainda não há dois).
func (q *arenaQueue) takePair(mode game.ArenaMode) (a, b arenaEntry, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	first := -1
	for i, e := range q.entries {
		if e.mode != mode {
			continue
		}
		if first < 0 {
			first = i
			continue
		}
		a, b = q.entries[first], e
		q.entries = slices.Delete(q.entries, i, i+1)
		q.entries = slices.Delete(q.entries, first, first+1)
		q.matches++
		return a, b, true
	}
	return a, b, false
}

// ---------- Mestre da Arena (na cidade) ----------

// arenaMasterID: o Mestre da Arena, o último dos ids fixos de NPC (talk.go, lastNPCID).
func arenaMasterID() uint32 {
	return talkerFirstID + uint32(len(game.SpawnedTalkers())+len(teleportStones)-legacyStones)
}

// spawnArenaMaster põe o Mestre na praça (só nas zonas com a cidade).
func (z *Zone) spawnArenaMaster() {
	pos := z.freeSpot(arenaMasterSpot)
	z.addNPC(&entity{
		id: arenaMasterID(), kind: pb.EntityKind_ENTITY_KIND_NPC,
		name: "Brann", model: "barbarian", level: 1, scale: 1,
		pos: pos, dest: pos, stats: game.Stats{MaxHP: 1}, hp: 1,
		npc: &npcState{title: "Mestre da Arena", role: pb.NpcRole_NPC_ROLE_ARENA,
			look: &game.Appearance{Skin: 2, Hair: 3, Outfit: 2, Head: "barbarian"}},
	})
}

// ArenaCmd repassa o pedido da janela da arena.
func (z *Zone) ArenaCmd(ctx context.Context, id uint32, m *pb.ArenaCmd) {
	z.post(ctx, socialDone{fn: func() { z.onArenaCmd(id, m) }})
}

func (z *Zone) onArenaCmd(id uint32, m *pb.ArenaCmd) {
	p := z.players[id]
	if p == nil || !z.spendAction(p) || z.mgr == nil {
		return
	}
	switch m.Kind {
	case pb.ArenaCmdKind_ARENA_CMD_OPEN:
		if z.arenaMaster(p, m.NpcId) == nil {
			return
		}
		z.sendArenaWindow(p, m.NpcId)
	case pb.ArenaCmdKind_ARENA_CMD_QUEUE:
		if n := z.arenaMaster(p, m.NpcId); n != nil {
			z.arenaJoinQueue(p, game.ArenaMode(m.Mode))
			z.sendArenaWindow(p, n.id)
		}
	case pb.ArenaCmdKind_ARENA_CMD_LEAVE:
		if z.mgr.arena.remove(p.id) {
			z.system(p, "Você saiu da fila da arena.")
			z.arenaNotifyTeam(p.id, "%s tirou o time da fila da arena.", p.name)
		}
		z.sendArenaState(p, false, m.NpcId)
	}
}

// arenaMaster: o Mestre perto do jogador (nil = longe ou não é ele).
func (z *Zone) arenaMaster(p *entity, npcID uint32) *entity {
	n := z.npcs[npcID]
	if n == nil || n.npc.role != pb.NpcRole_NPC_ROLE_ARENA {
		return nil
	}
	if dist(p.pos, n.pos) > arenaRange {
		z.system(p, "Chegue mais perto do Mestre da Arena.")
		return nil
	}
	return n
}

// arenaJoinQueue: o jogador (ou o líder, no 3x3) põe o time na fila.
func (z *Zone) arenaJoinQueue(p *entity, mode game.ArenaMode) {
	m := z.mgr
	switch {
	case !mode.Valid():
		return
	case p.dead:
		z.system(p, "Você está morto.")
		return
	case p.level < game.ArenaMinLevel:
		z.system(p, fmt.Sprintf("A arena é a partir do nível %d.", game.ArenaMinLevel))
		return
	case m.arena.queuedMode(p.id) != 0:
		z.system(p, "Você já está na fila da arena.")
		return
	}
	e := arenaEntry{mode: mode, members: []uint32{p.id}, names: []string{p.name}, since: z.cfg.Clock()}
	if mode == game.ArenaTrios {
		pid := m.bus.partyOf(p.id)
		v, ok := m.bus.party(pid)
		switch {
		case pid == 0 || !ok:
			z.system(p, "O 3x3 é para um grupo de 3: monte o grupo e o líder entra na fila.")
			return
		case v.leader != p.id:
			z.system(p, "Só o líder do grupo põe o time na fila do 3x3.")
			return
		case len(v.members) != int(game.ArenaTrios):
			z.system(p, fmt.Sprintf("O 3x3 é para um grupo de exatamente 3 (o seu tem %d).", len(v.members)))
			return
		}
		e.members, e.names = nil, nil
		for _, id := range v.members {
			o := z.players[id]
			if o == nil {
				z.system(p, "Todo o grupo precisa estar aqui, na cidade.")
				return
			}
			if o.level < game.ArenaMinLevel {
				z.system(p, fmt.Sprintf("%s está abaixo do nível %d.", o.name, game.ArenaMinLevel))
				return
			}
			e.members, e.names = append(e.members, id), append(e.names, o.name)
		}
	}
	if why := m.arena.add(e); why != "" {
		z.system(p, why)
		return
	}
	z.system(p, fmt.Sprintf("Você entrou na fila da arena (%s). A partida começa quando outro time entrar.", mode.Label()))
	if mode == game.ArenaTrios {
		z.arenaNotifyTeam(p.id, "%s pôs o time na fila do 3x3 da arena.", p.name)
	}
	z.track(p, "arena_queue", mode.Label(), int64(len(e.members)), nil)
}

// arenaNotifyTeam avisa os outros do grupo que estão nesta zona.
func (z *Zone) arenaNotifyTeam(from uint32, format, name string) {
	p := z.players[from]
	if p == nil || p.pl.party == nil {
		return
	}
	for _, id := range p.pl.party.members {
		if o := z.players[id]; o != nil && id != from {
			z.system(o, fmt.Sprintf(format, name))
		}
	}
}

// sendArenaWindow: pontuação, fila e ranking (o banco responde pelo worker).
func (z *Zone) sendArenaWindow(p *entity, npcID uint32) {
	z.sendArenaState(p, true, npcID) // na hora, sem o banco
	season := game.ArenaSeason(z.cfg.Clock())
	id := p.id
	z.queueArenaJob(func(ctx context.Context, b ArenaBackend) func() {
		mine, err := b.ArenaRatingsOf(ctx, season, id)
		if err != nil {
			z.log.Warn("arena ratings", "err", err)
			return nil
		}
		ranks := map[game.ArenaMode][]ArenaRank{}
		for _, mode := range game.ArenaModes {
			r, err := b.ArenaRanking(ctx, season, mode, game.ArenaRankingLimit)
			if err != nil {
				z.log.Warn("arena ranking", "err", err)
				return nil
			}
			ranks[mode] = r
		}
		return func() {
			if p := z.players[id]; p != nil {
				p.pl.arenaSeen = &arenaSeen{season: season, mine: mine, ranks: ranks}
				z.sendArenaState(p, true, npcID)
			}
		}
	})
}

// arenaSeen: a última leitura do banco para a janela do jogador.
type arenaSeen struct {
	season int
	mine   map[game.ArenaMode]ArenaRecord
	ranks  map[game.ArenaMode][]ArenaRank
}

// sendArenaState manda a janela (open) ou só o estado da fila e da partida.
func (z *Zone) sendArenaState(p *entity, open bool, npcID uint32) {
	season := game.ArenaSeason(z.cfg.Clock())
	st := &pb.ArenaState{Open: open, NpcId: npcID, Season: uint32(season), SeasonLabel: game.ArenaSeasonLabel(season)}
	seen := p.pl.arenaSeen
	if seen != nil && seen.season != season {
		seen = nil
	}
	queued := game.ArenaMode(0)
	if z.mgr != nil {
		queued = z.mgr.arena.queuedMode(p.id)
	}
	for _, mode := range game.ArenaModes {
		info := &pb.ArenaModeInfo{Mode: uint32(mode), Rating: game.ArenaStartRating, Queued: queued == mode}
		if z.mgr != nil {
			info.QueueSize = uint32(z.mgr.arena.size(mode))
		}
		if seen != nil {
			if r, ok := seen.mine[mode]; ok {
				info.Rating, info.Wins, info.Losses = int32(r.Rating), uint32(r.Wins), uint32(r.Losses)
			}
			rk := &pb.ArenaRanking{Mode: uint32(mode)}
			for _, r := range seen.ranks[mode] {
				rk.Rows = append(rk.Rows, &pb.ArenaRank{Name: r.Name, Class: r.Class, Level: uint32(r.Level),
					Rating: int32(r.Rating), Wins: uint32(r.Wins), Losses: uint32(r.Losses)})
			}
			st.Rankings = append(st.Rankings, rk)
		}
		st.Modes = append(st.Modes, info)
	}
	if z.arena != nil {
		st.Match = z.arena.matchPB(z, p.id)
	}
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Arena{Arena: st}})
}

// ---------- Formar as partidas (a zona principal, 1×/s) ----------

// arenaMatchmake: com dois times na fila de um modo, cria a cópia e manda os dois para lá.
// Roda na goroutine do mapa principal (Canal 1).
func (z *Zone) arenaMatchmake() {
	if z.mgr == nil || z.mgr.Home() != z {
		return
	}
	m := z.mgr
	for _, mode := range game.ArenaModes {
		for {
			a, b, ok := m.arena.takePair(mode)
			if !ok {
				break
			}
			in, err := m.newArenaInstance(mode, [2]arenaEntry{a, b}, z.cfg.Clock())
			if err != nil {
				m.log.Error("arena instance", "err", err)
				for _, e := range []arenaEntry{a, b} { // volta para a fila
					m.arena.add(e)
				}
				return
			}
			for team, e := range []arenaEntry{a, b} {
				for i, id := range e.members {
					spot := arenaSpot(team, i, len(e.members))
					if from := m.bus.zoneOf(id); from != nil {
						from.post(m.runCtx(), socialDone{fn: func() { from.sendToArena(id, in, spot) }})
					}
				}
			}
		}
	}
}

// arenaSpot: onde o membro i do time aparece (os times frente a frente, no eixo x).
func arenaSpot(team, i, n int) Vec2 {
	x := -arenaSideX
	if team == 1 {
		x = arenaSideX
	}
	z := (float64(i) - float64(n-1)/2) * arenaTeamSpread
	return Vec2{X: x, Z: z}
}

// sendToArena leva o jogador desta zona para a partida (goroutine da zona dele).
func (z *Zone) sendToArena(id uint32, in *Instance, at Vec2) {
	p := z.players[id]
	switch {
	case p == nil:
		return
	case p.dead || p.pl.trade != nil || p.pl.channel != nil || p.pl.pshop != nil || z.cfg.Instance != "":
		z.system(p, "Sua partida na arena começou, mas você não podia ir agora (morto, viajando, trocando ou com a loja aberta). O time luta sem você.")
		return
	}
	z.system(p, "Sua partida na arena vai começar!")
	z.crossZone(p, in.zone, at, "arena")
}

// newArenaInstance cria a cópia da arena para os dois times.
func (m *Manager) newArenaInstance(mode game.ArenaMode, teams [2]arenaEntry, now time.Time) (*Instance, error) {
	h := &m.inst
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.live == nil {
		h.live = map[string]*Instance{}
	}
	if h.max == 0 {
		h.max = instanceMax
	}
	if len(h.live) >= h.max {
		return nil, fmt.Errorf("cópias cheias (%d)", h.max)
	}
	h.seq++
	id := fmt.Sprintf("%s#%d", ArenaMap, h.seq)
	cfg, err := ArenaConfig(h.base, id)
	if err != nil {
		return nil, err
	}
	in, err := m.startInstanceLocked(cfg, ArenaMap, "arena", teams[0].members[0], now)
	if err != nil {
		return nil, err
	}
	run := &arenaRun{mode: mode, season: game.ArenaSeason(now), start: now, winner: -1,
		team: map[uint32]int{}, names: map[uint32]string{}, out: map[uint32]bool{}, arrived: map[uint32]bool{}}
	for t, e := range teams {
		run.teams[t] = slices.Clone(e.members)
		for i, id := range e.members {
			run.team[id] = t
			run.names[id] = e.names[i]
			in.entered[id] = true
		}
	}
	in.zone.arena = run
	in.zone.dg.emptyTTL = arenaEmptyTTL
	return in, nil
}

// ArenaConfig: a cópia da arena (o mapa pequeno, sem monstros, sem cidade e sem portões).
func ArenaConfig(base Config, instance string) (Config, error) {
	w, err := world.LoadMap(ArenaMap)
	if err != nil {
		return base, err
	}
	cfg := base
	cfg.Map, cfg.MapTitle, cfg.Instance = ArenaMap, "Arena de Vel'Aran", instance
	cfg.World, cfg.Size, cfg.SafeRadius = w, w.Size, 0
	cfg.Spawns, cfg.Gates = nil, nil
	cfg.NoTown, cfg.NoRares = true, true
	cfg.InvasionChance = 0
	cfg.Clans, cfg.Bosses, cfg.DevCommands = nil, nil, false
	cfg.Dungeons, cfg.Channel, cfg.Capacity = nil, 0, 0
	return cfg, nil
}

// ---------- A partida (na cópia) ----------

// arenaRun é a partida: só a goroutine da cópia mexe.
type arenaRun struct {
	mode     game.ArenaMode
	season   int
	teams    [2][]uint32
	team     map[uint32]int
	names    map[uint32]string
	start    time.Time
	phase    pb.ArenaPhase
	phaseEnd time.Time
	arrived  map[uint32]bool
	out      map[uint32]bool // morreu ou saiu do jogo: fora até o fim
	winner   int             // -1 = não acabou (ou empate, com done)
	changes  map[uint32]ArenaChange
	recorded bool
	sentHome bool
}

// alive: quantos do time estão de pé na arena.
func (r *arenaRun) alive(z *Zone, team int) int {
	n := 0
	for _, id := range r.teams[team] {
		if p := z.players[id]; p != nil && !p.dead && !r.out[id] && r.arrived[id] {
			n++
		}
	}
	return n
}

// hpShare: a soma das frações de vida de quem está de pé no time (desempate do tempo).
func (r *arenaRun) hpShare(z *Zone, team int) float64 {
	s := 0.0
	for _, id := range r.teams[team] {
		if p := z.players[id]; p != nil && !p.dead && !r.out[id] && r.arrived[id] && p.stats.MaxHP > 0 {
			s += p.hp / p.stats.MaxHP
		}
	}
	return s
}

func (r *arenaRun) matchPB(z *Zone, viewer uint32) *pb.ArenaMatch {
	m := &pb.ArenaMatch{Mode: uint32(r.mode), Phase: r.phase, MyTeam: uint32(r.team[viewer]), Winner: int32(r.winner)}
	if r.phase != pb.ArenaPhase_ARENA_PHASE_DONE {
		m.Winner = -1
	}
	if left := r.phaseEnd.Sub(z.cfg.Clock()); left > 0 {
		m.SecondsLeft = uint32(math.Ceil(left.Seconds()))
	}
	for t, ids := range r.teams {
		for _, id := range ids {
			p := z.players[id]
			alive := p != nil && !p.dead && !r.out[id] && r.arrived[id]
			m.Fighters = append(m.Fighters, &pb.ArenaFighter{Id: id, Name: r.names[id], Team: uint32(t), Alive: alive})
		}
	}
	if c, ok := r.changes[viewer]; ok {
		m.RatingBefore, m.RatingAfter = int32(c.Before), int32(c.After)
	}
	return m
}

// arenaBroadcast manda o estado da partida a todos na cópia.
func (z *Zone) arenaBroadcast() {
	for _, id := range z.sortedPlayerIDs() {
		z.sendArenaState(z.players[id], false, 0)
	}
}

// arenaOnJoin: alguém da partida chegou (ou voltou depois de cair).
func (z *Zone) arenaOnJoin(p *entity) {
	r := z.arena
	if r == nil {
		return
	}
	if _, ok := r.team[p.id]; !ok { // não é desta partida (não deveria acontecer): volta
		z.afterFlush(func() { z.arenaSendHome(p) })
		return
	}
	r.arrived[p.id] = true
	if r.phase == pb.ArenaPhase_ARENA_PHASE_NONE {
		r.phase, r.phaseEnd = pb.ArenaPhase_ARENA_PHASE_COUNTDOWN, r.start.Add(game.ArenaCountdown)
	}
	p.hp, p.mp = p.stats.MaxHP, p.stats.MaxMP
	p.pl.statsDirty = true
	z.systemAfterFlush(p, fmt.Sprintf("Arena %s: a luta começa em %d s. Só o seu time é aliado.", r.mode.Label(), int(game.ArenaCountdown.Seconds())))
	z.afterFlush(z.arenaBroadcast)
}

// arenaCanHit: as regras de ataque dentro da arena (só na luta, só no outro time).
func (z *Zone) arenaCanHit(a, b *entity) (bool, string) {
	r := z.arena
	ta, okA := r.team[a.id]
	tb, okB := r.team[b.id]
	switch {
	case !okA || !okB || r.out[a.id] || r.out[b.id]:
		return false, ""
	case r.phase != pb.ArenaPhase_ARENA_PHASE_FIGHT:
		return false, "A luta ainda não começou."
	case ta == tb:
		return false, "Você não pode atacar alguém do seu time."
	}
	return true, ""
}

// arenaAllies: na arena, só o time é aliado.
func (z *Zone) arenaAllies(a, b *entity) bool {
	ta, okA := z.arena.team[a.id]
	tb, okB := z.arena.team[b.id]
	return okA && okB && ta == tb
}

// arenaDeath: quem morre na arena fica fora até o fim (sem perder nada).
func (z *Zone) arenaDeath(dead, killer *entity) {
	r := z.arena
	r.out[dead.id] = true
	z.forgetTarget(dead.id)
	msg := "Você caiu na arena. Fica fora até o fim da partida."
	if killer != nil && killer.pl != nil && killer != dead {
		msg = fmt.Sprintf("%s derrubou você na arena. Fica fora até o fim da partida.", killer.name)
	}
	z.system(dead, msg)
	z.track(dead, "arena_death", r.mode.Label(), 0, nil)
	z.afterFlush(z.arenaBroadcast)
}

// arenaOnLeave: saiu do jogo (ou da cópia) no meio: conta como fora.
func (z *Zone) arenaOnLeave(p *entity) {
	if r := z.arena; r != nil && r.phase != pb.ArenaPhase_ARENA_PHASE_DONE {
		if _, ok := r.team[p.id]; ok {
			r.out[p.id] = true
		}
	}
}

// updateArena roda 1×/s na cópia: as fases, o fim e a volta à cidade.
func (z *Zone) updateArena() {
	r := z.arena
	if r == nil {
		return
	}
	now := z.cfg.Clock()
	switch r.phase {
	case pb.ArenaPhase_ARENA_PHASE_NONE:
		// Ninguém chegou: depois da preparação, a partida acaba sem resultado.
		if now.Sub(r.start) > game.ArenaCountdown+5*time.Second {
			z.arenaFinish(-1, "Ninguém chegou à arena.")
		}
	case pb.ArenaPhase_ARENA_PHASE_COUNTDOWN:
		if now.Before(r.phaseEnd) {
			return
		}
		a, b := r.alive(z, 0), r.alive(z, 1)
		switch {
		case a == 0 && b == 0:
			z.arenaFinish(-1, "Nenhum time chegou à arena.")
		case a == 0:
			z.arenaFinish(1, "O outro time não chegou: vitória por W.O.")
		case b == 0:
			z.arenaFinish(0, "O outro time não chegou: vitória por W.O.")
		default:
			r.phase, r.phaseEnd = pb.ArenaPhase_ARENA_PHASE_FIGHT, now.Add(game.ArenaFight)
			z.announceAll("Lutem!")
			z.arenaBroadcast()
		}
	case pb.ArenaPhase_ARENA_PHASE_FIGHT:
		a, b := r.alive(z, 0), r.alive(z, 1)
		switch {
		case a == 0 && b == 0:
			z.arenaFinish(-1, "Os dois times caíram: empate.")
		case a == 0:
			z.arenaFinish(1, "")
		case b == 0:
			z.arenaFinish(0, "")
		case !now.Before(r.phaseEnd):
			ha, hb := r.hpShare(z, 0), r.hpShare(z, 1)
			switch {
			case math.Abs(ha-hb) < 1e-6:
				z.arenaFinish(-1, "Acabou o tempo, com a mesma vida: empate.")
			case ha > hb:
				z.arenaFinish(0, "Acabou o tempo: vence quem tem mais vida.")
			default:
				z.arenaFinish(1, "Acabou o tempo: vence quem tem mais vida.")
			}
		}
	case pb.ArenaPhase_ARENA_PHASE_DONE:
		if !r.sentHome && !now.Before(r.phaseEnd) {
			r.sentHome = true
			for _, id := range z.sortedPlayerIDs() {
				p := z.players[id]
				z.afterFlush(func() { z.arenaSendHome(p) })
			}
		}
	}
}

// arenaFinish encerra a luta (winner -1 = sem vencedor) e manda gravar a pontuação.
func (z *Zone) arenaFinish(winner int, why string) {
	r := z.arena
	r.phase, r.phaseEnd, r.winner = pb.ArenaPhase_ARENA_PHASE_DONE, z.cfg.Clock().Add(game.ArenaAfter), winner
	for _, id := range z.sortedPlayerIDs() {
		p := z.players[id]
		t, ok := r.team[id]
		var text string
		switch {
		case !ok:
			continue
		case winner < 0:
			text = "Partida sem vencedor. A pontuação não muda."
		case t == winner:
			text = "Vitória na arena!"
		default:
			text = "Derrota na arena."
		}
		if why != "" {
			text = why + " " + text
		}
		z.system(p, text)
	}
	extra := map[string]any{"winner": winner, "team0": len(r.teams[0]), "team1": len(r.teams[1]),
		"seconds": int64(z.cfg.Clock().Sub(r.start).Seconds())}
	z.track(nil, "arena_match", r.mode.Label(), int64(winner), extra)
	z.arenaBroadcast()
	if winner < 0 || r.recorded {
		return
	}
	r.recorded = true
	res := ArenaResult{Season: r.season, Mode: r.mode, Teams: [2][]uint32{slices.Clone(r.teams[0]), slices.Clone(r.teams[1])}, Winner: winner}
	z.queueArenaJob(func(ctx context.Context, b ArenaBackend) func() {
		changes, err := b.RecordArenaMatch(ctx, res)
		if err != nil {
			z.log.Error("arena record", "err", err)
			return nil
		}
		return func() {
			if z.arena != r {
				return
			}
			r.changes = map[uint32]ArenaChange{}
			for _, c := range changes {
				r.changes[c.ID] = c
				if p := z.players[c.ID]; p != nil {
					z.system(p, fmt.Sprintf("Pontuação da arena (%s): %d → %d.", r.mode.Label(), c.Before, c.After))
				}
			}
			z.arenaBroadcast()
		}
	})
}

// arenaSendHome: vida e mana cheias e de volta à cidade, ao lado do Mestre.
func (z *Zone) arenaSendHome(p *entity) {
	if p == nil || z.players[p.id] != p || p.pl.hop.out != nil || z.mgr == nil {
		return
	}
	if p.dead {
		p.dead = false
		z.frame.revives = append(z.frame.revives, &pb.Revive{Id: p.id, X: float32(p.pos.X), Z: float32(p.pos.Z)})
	}
	p.hp, p.mp = p.stats.MaxHP, p.stats.MaxMP
	p.pl.killedBy = ""
	dest := z.mgr.Arrival(world.StarterMap)
	if dest == nil {
		dest = z.mgr.Home()
	}
	z.crossZone(p, dest, Vec2{X: arenaMasterSpot.X + 2.5, Z: arenaMasterSpot.Z}, "arena_exit") // ao lado do Mestre, não em cima
}
