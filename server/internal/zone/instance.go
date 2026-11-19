package zone

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/world"
)

// Masmorras instanciadas (tunnel#140, roadmap seção 8): uma cópia do mapa por grupo, criada
// quando o líder entra e destruída quando ninguém está nela.
//
// Cada cópia é uma Zone de verdade (a mesma simulação, a mesma goroutine por zona), criada
// e rodada pelo Manager. Por ser uma zona à parte, o isolamento é o de sempre: os monstros,
// a AOI, o saque e o chat local não saem dela. Fora do mapa principal, só o que o barramento
// leva a todas as zonas chega lá: o grupo, o sussurro e o chat de grupo. O chat geral não
// entra nem sai (broadcastWorld).
//
// Vida da cópia:
//   - nasce quando o líder do grupo (ou quem está sozinho) usa a porta da igreja;
//   - os outros membros do grupo entram pela mesma porta até instanceJoinWindow depois;
//     quem já entrou volta quando quiser, enquanto a cópia existir;
//   - some instanceEmptyTTL depois de ficar vazia, ou com instanceMaxAge de vida: avisa aos 5
//     min do fim e, na hora, leva quem estiver lá para a porta da igreja;
//   - ao fechar, anota a telemetria (duração e mortes) em game_events.
//
// O estado do personagem é o de sempre (o portão): a cópia grava Zone = o id do mapa
// ("catacumbas"); quem entra de novo sem a cópia (cópia fechada, reinício do servidor) volta
// na porta da igreja (Manager.homeSpot).
const (
	instanceJoinWindow = 2 * time.Minute
	instanceEmptyTTL   = 5 * time.Minute
	instanceMaxAge     = 45 * time.Minute
	instanceWarnBefore = 5 * time.Minute
	instanceCloseGrace = 3 * time.Second // depois de fechar a porta, espera a última entrega chegar
	instanceMax        = 24              // cópias ao mesmo tempo no processo
)

// dungeonDef é uma masmorra (os dados que não vêm do mapa).
type dungeonDef struct {
	id, title string
	minLevel  int
	maxLevel  int
	partyMax  int
	// door: a porta no mapa principal (a igreja da cidade). exit: onde o grupo aparece ao sair.
	door GateDef
	exit Vec2
}

var dungeonDefs = map[string]dungeonDef{
	game.CatacombsID: {
		id: game.CatacombsID, title: "Catacumbas de Vel'Aran",
		minLevel: game.CatacombsMinLevel, maxLevel: game.CatacombsMaxLevel, partyMax: game.CatacombsPartyMax,
		// A igreja fica a 135° e 25 m do poço, de frente para a praça. A porta é a do portão dela.
		door: GateDef{Name: "Porta das Catacumbas", X: -13.8, Z: 13.8, Dungeon: game.CatacombsID, Label: "Catacumbas de Vel'Aran (nível 12 a 16)"},
		exit: Vec2{X: -12.0, Z: 12.0},
	},
}

// EnabledDungeons: as masmorras desta execução. Todas ligadas por padrão; EREVIA_DUNGEONS
// ("off" ou "nenhuma" desliga todas; senão, ids separados por vírgula) escolhe.
func EnabledDungeons(env string) []string {
	env = strings.ToLower(strings.TrimSpace(env))
	var out []string
	switch env {
	case "off", "nenhuma", "none":
		return nil
	case "":
		for id := range dungeonDefs {
			out = append(out, id)
		}
	default:
		for _, id := range strings.Split(env, ",") {
			if _, ok := dungeonDefs[strings.TrimSpace(id)]; ok {
				out = append(out, strings.TrimSpace(id))
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// dungeonDoors: as portas das masmorras ligadas, para o mapa principal.
func dungeonDoors(enabled []string) []GateDef {
	var out []GateDef
	for _, id := range enabled {
		if d, ok := dungeonDefs[id]; ok {
			g := d.door
			g.ToTitle = d.title
			out = append(out, g)
		}
	}
	slices.SortFunc(out, func(a, b GateDef) int { return strings.Compare(a.Dungeon, b.Dungeon) })
	return out
}

// DungeonConfig monta a configuração da cópia: o mundo da masmorra, os monstros de cada
// marca do gerador e as escadas e a saída como portões. Sem a cidade, sem raros, sem invasão
// e sem clãs (como as outras zonas fora do mapa principal).
func DungeonConfig(base Config, id, instance string) (Config, error) {
	d, ok := dungeonDefs[id]
	if !ok {
		return base, fmt.Errorf("masmorra %q desconhecida", id)
	}
	w, err := world.LoadMap(id)
	if err != nil {
		return base, err
	}
	if w.Dungeon == nil {
		return base, fmt.Errorf("mapa %q não é uma masmorra (sem o campo dungeon)", id)
	}
	cfg := base
	cfg.Map, cfg.MapTitle, cfg.Instance = id, d.title, instance
	cfg.World, cfg.Size, cfg.SafeRadius = w, w.Size, w.SafeRadius
	cfg.Spawns = dungeonSpawns(w.Dungeon)
	cfg.Gates = nil
	for _, s := range w.Dungeon.Stairs {
		cfg.Gates = append(cfg.Gates, GateDef{Name: s.Name, Label: "Escada", X: s.X, Z: s.Z, ArriveX: s.ToX, ArriveZ: s.ToZ, Stairs: true})
	}
	cfg.Gates = append(cfg.Gates, GateDef{
		Name: "Saída das Catacumbas", X: w.Dungeon.Entry.X, Z: w.Dungeon.Entry.Z - 2.5,
		To: world.StarterMap, ArriveX: d.exit.X, ArriveZ: d.exit.Z, ToTitle: mapDefs[world.StarterMap].title,
	})
	cfg.NoTown, cfg.NoRares = true, true
	cfg.InvasionChance = 0
	cfg.Clans, cfg.Bosses, cfg.DevCommands = nil, nil, false
	cfg.Dungeons = nil
	return cfg, nil
}

// dungeonSpawns transforma as marcas do gerador em spawns fixos, sem renascimento. A região é
// o andar (o mapa do cliente mostra só o chefe; ver mapZoneSpawns).
func dungeonSpawns(d *world.Dungeon) []game.Spawn {
	var out []game.Spawn
	for _, m := range d.Spawns {
		n := max(1, m.Count)
		sp := game.Spawn{
			Monster: m.Monster, Count: n, X: m.X, Z: m.Z, LevelMin: m.Level, LevelMax: m.Level,
			Respawn: game.DungeonNoRespawn, Region: fmt.Sprintf("Andar %d", m.Floor+1),
		}
		if n > 1 {
			sp.Area, sp.Radius = true, 1.8 // a matilha nasce junta (puxar um puxa todos), não empilhada
		} else {
			sp.Fixed = true
		}
		out = append(out, sp)
	}
	return out
}

// mapZoneSpawns: os spawns que o mapa do cliente mostra. Na masmorra, só o chefe (os
// monstros são dezenas de pontos e o grupo os descobre andando).
func (z *Zone) mapZoneSpawns() []game.Spawn {
	if z.cfg.Instance == "" {
		return z.cfg.Spawns
	}
	var out []game.Spawn
	for _, s := range z.cfg.Spawns {
		if def := game.Monsters[s.Monster]; def != nil && def.Boss {
			out = append(out, s)
		}
	}
	return out
}

// ---------- O gerente das cópias ----------

// InstanceFactory cria a zona de uma cópia (cmd/zone liga os bancos e a persistência dela) e
// devolve o canal que fecha quando a persistência terminou de gravar. nil = zona sem banco.
type InstanceFactory func(cfg Config, log *slog.Logger) (z *Zone, persisted <-chan struct{})

// instanceHost: o que o Manager guarda das cópias.
type instanceHost struct {
	mu      sync.Mutex
	base    Config
	enabled []string
	factory InstanceFactory
	live    map[string]*Instance // id da cópia → cópia
	seq     int
	max     int
	wg      sync.WaitGroup
	manual  bool // testes: a cópia não roda sozinha (quem testa trata os eventos e os ticks)
}

// Instance é uma cópia viva de uma masmorra.
type Instance struct {
	id      string
	dungeon string
	zone    *Zone
	key     string // o grupo: "p<id do grupo>" ou "s<id do personagem>" (sozinho)
	leader  uint32
	created time.Time
	cancel  context.CancelFunc
	// Guardados por instanceHost.mu:
	entered map[uint32]bool
	closing bool
}

// Entry: onde o grupo chega (andar 1).
func (i *Instance) Entry() Vec2 {
	d := i.zone.cfg.World.Dungeon
	return Vec2{X: d.Entry.X, Z: d.Entry.Z}
}

// ID da cópia ("catacumbas#3").
func (i *Instance) ID() string { return i.id }

// Zone da cópia.
func (i *Instance) Zone() *Zone { return i.zone }

// EnableDungeons liga as masmorras do processo (antes do Run). base é a configuração de
// todas as zonas; factory liga os bancos de cada cópia (nil = cópia sem banco, nos testes).
func (m *Manager) EnableDungeons(base Config, factory InstanceFactory) {
	m.inst.mu.Lock()
	defer m.inst.mu.Unlock()
	m.inst.base, m.inst.factory = base, factory
	m.inst.enabled = slices.Clone(base.Dungeons)
	if m.inst.live == nil {
		m.inst.live = map[string]*Instance{}
	}
	if m.inst.max == 0 {
		m.inst.max = instanceMax
	}
}

// Instances: as cópias vivas, em ordem de id.
func (m *Manager) Instances() []*Instance {
	m.inst.mu.Lock()
	defer m.inst.mu.Unlock()
	out := make([]*Instance, 0, len(m.inst.live))
	for _, i := range m.inst.live {
		out = append(out, i)
	}
	slices.SortFunc(out, func(a, b *Instance) int { return strings.Compare(a.id, b.id) })
	return out
}

// AllZones: as zonas fixas e as cópias vivas.
func (m *Manager) AllZones() []*Zone {
	out := slices.Clone(m.zones)
	for _, i := range m.Instances() {
		out = append(out, i.zone)
	}
	return out
}

// homeSpot: onde o personagem aparece no mapa principal quando a zona gravada não existe
// mais: o centro da praça, ou a porta da masmorra de onde ele veio.
func (m *Manager) homeSpot(saved string) (x, z float64) {
	if d, ok := dungeonDefs[saved]; ok {
		return d.exit.X, d.exit.Z
	}
	return 0, 0
}

// dungeonEntry decide se o personagem entra e em qual cópia (criando a do líder).
func (m *Manager) dungeonEntry(id uint32, dungeon string) (*Instance, string) {
	d, ok := dungeonDefs[dungeon]
	h := &m.inst
	h.mu.Lock()
	defer h.mu.Unlock()
	if !ok || !slices.Contains(h.enabled, dungeon) || h.live == nil {
		return nil, "A porta está trancada."
	}
	if m.runCtx().Err() != nil {
		return nil, "A porta está trancada."
	}
	now := m.Home().cfg.Clock()
	// Quem já entrou numa cópia viva volta para ela (queda de conexão, saída pela porta).
	for _, in := range h.live {
		if in.dungeon == dungeon && in.entered[id] && !in.closing {
			return in, ""
		}
	}
	key, leader, groupSize := fmt.Sprintf("s%d", id), true, 1
	if pid := m.bus.partyOf(id); pid != 0 {
		if v, ok := m.bus.party(pid); ok {
			key, leader, groupSize = fmt.Sprintf("p%d", pid), v.leader == id, len(v.members)
		}
	}
	for _, in := range h.live {
		if in.dungeon != dungeon || in.key != key || in.closing {
			continue
		}
		switch {
		case now.Sub(in.created) > instanceJoinWindow:
			return nil, "A porta das Catacumbas deste grupo fechou: ela fica aberta por 2 minutos depois que o líder entra."
		case len(in.entered) >= d.partyMax:
			return nil, fmt.Sprintf("As Catacumbas já têm %d aventureiros.", d.partyMax)
		}
		in.entered[id] = true
		return in, ""
	}
	if !leader {
		return nil, "Só o líder do grupo abre a porta das Catacumbas. Peça a ele para entrar primeiro."
	}
	if groupSize > d.partyMax {
		return nil, fmt.Sprintf("O grupo tem gente demais: as Catacumbas aceitam até %d.", d.partyMax)
	}
	if len(h.live) >= h.max {
		return nil, "As Catacumbas estão cheias. Tente de novo daqui a pouco."
	}
	in, err := m.newInstanceLocked(d, key, id, now)
	if err != nil {
		m.log.Error("instance create", "dungeon", dungeon, "err", err)
		return nil, "A porta está trancada."
	}
	in.entered[id] = true
	return in, ""
}

// newInstanceLocked cria e liga a cópia (h.mu preso).
func (m *Manager) newInstanceLocked(d dungeonDef, key string, leader uint32, now time.Time) (*Instance, error) {
	h := &m.inst
	h.seq++
	id := fmt.Sprintf("%s#%d", d.id, h.seq)
	cfg, err := DungeonConfig(h.base, d.id, id)
	if err != nil {
		return nil, err
	}
	return m.startInstanceLocked(cfg, d.id, key, leader, now)
}

// startInstanceLocked liga uma cópia já configurada (cfg.Instance = o id; h.mu preso): a
// masmorra e a arena (arena.go).
func (m *Manager) startInstanceLocked(cfg Config, dungeon, key string, leader uint32, now time.Time) (*Instance, error) {
	h := &m.inst
	id := cfg.Instance
	cfg.Seed = h.base.Seed + uint64(h.seq)*7919 // cada cópia sorteia o seu
	log := m.log.With("zone", id)
	var z *Zone
	var persisted <-chan struct{}
	if h.factory != nil {
		z, persisted = h.factory(cfg, log)
	} else {
		z = New(cfg, log, nil)
	}
	in := &Instance{id: id, dungeon: dungeon, zone: z, key: key, leader: leader, created: now, entered: map[uint32]bool{}}
	z.mgr = m
	z.dg = &dungeonRun{inst: in, created: now, emptySince: now}
	h.live[id] = in
	if !h.manual {
		ctx, cancel := context.WithCancel(m.runCtx())
		in.cancel = cancel
		h.wg.Add(1)
		go m.runInstance(ctx, in, persisted)
	}
	m.log.Info("instance created", "instance", id, "key", key, "leader", leader)
	return in, nil
}

// runInstance roda a cópia até ela fechar e a tira do gerente.
func (m *Manager) runInstance(ctx context.Context, in *Instance, persisted <-chan struct{}) {
	defer m.inst.wg.Done()
	z := in.zone
	z.Run(ctx)
	z.closed.Store(true) // parou: nada mais é tratado aqui (a persistência ainda termina de gravar)
	if persisted != nil {
		select {
		case <-persisted:
		case <-time.After(10 * time.Second):
			m.log.Warn("instance persistence did not finish in time", "instance", in.id)
		}
	}
	select {
	case <-z.TelemetryDone():
	case <-time.After(5 * time.Second):
	}
	m.dropInstance(in)
}

// dropInstance tira a cópia fechada do gerente.
func (m *Manager) dropInstance(in *Instance) {
	m.inst.mu.Lock()
	delete(m.inst.live, in.id)
	m.inst.mu.Unlock()
	in.zone.closed.Store(true)
	m.log.Info("instance closed", "instance", in.id)
}

// markClosing fecha a porta da cópia: ninguém mais entra (chamado pela zona dela).
func (m *Manager) markClosing(in *Instance) {
	m.inst.mu.Lock()
	in.closing = true
	m.inst.mu.Unlock()
}

// allEntered: quantos já entraram (o pico do grupo, para a telemetria).
func (m *Manager) allEntered(in *Instance) int {
	m.inst.mu.Lock()
	defer m.inst.mu.Unlock()
	return len(in.entered)
}

// ---------- A zona da cópia ----------

// dungeonRun é o estado da cópia que só a goroutine dela mexe.
type dungeonRun struct {
	inst       *Instance
	created    time.Time
	emptySince time.Time // zero = tem gente
	warned     bool
	closing    bool
	emptyTTL   time.Duration // vazia por esse tempo, fecha (0 = instanceEmptyTTL; a arena fecha logo)
	closeAt    time.Time
	reason     string
	deaths     int
	bossDown   bool
	bossAfter  time.Duration // quanto depois da criação o chefe caiu
	chests     int           // baús abertos
}

// isClosed: a cópia está fechando ou já fechou (o login não pode mais ir para ela; quem já está
// nela, online, continua até a evacuação).
func (z *Zone) isClosed() bool { return z.closed.Load() }

// useDungeonDoor: o jogador usou a porta da masmorra (a da igreja).
func (z *Zone) useDungeonDoor(p *entity, g *GateDef) {
	if z.mgr == nil {
		z.system(p, "A porta está trancada.")
		return
	}
	switch {
	case p.dead:
		z.system(p, "Você está morto.")
		return
	case p.pl.trade != nil:
		z.system(p, "Feche a troca antes de atravessar a porta.")
		return
	case z.now-p.lastCombat < gateCombatWait:
		left := int(gateCombatWait - (z.now - p.lastCombat) + 0.999)
		z.system(p, fmt.Sprintf("Você está em combate: espere %d s para atravessar a porta.", left))
		return
	}
	in, msg := z.mgr.dungeonEntry(p.id, g.Dungeon)
	if in == nil {
		z.system(p, msg)
		return
	}
	if d := dungeonDefs[g.Dungeon]; p.level < d.minLevel {
		z.system(p, fmt.Sprintf("As Catacumbas são para os níveis %d a %d: o que vive lá embaixo vai ser difícil demais para você.", d.minLevel, d.maxLevel))
	} else if p.level > d.maxLevel {
		z.system(p, fmt.Sprintf("As Catacumbas são para os níveis %d a %d: o que vive lá embaixo vai ser fácil demais para você.", d.minLevel, d.maxLevel))
	}
	z.crossZone(p, in.zone, in.Entry(), "dungeon")
}

// devDungeon: /catacumbas (só com DEV_LOGIN) leva o jogador à cópia do grupo dele sem a porta (os
// testes de arte e de FPS entram assim). As regras de grupo e de janela valem como na porta.
func (z *Zone) devDungeon(p *entity) {
	if z.mgr == nil || z.cfg.Instance != "" || p.dead {
		return
	}
	in, msg := z.mgr.dungeonEntry(p.id, game.CatacombsID)
	if in == nil {
		z.system(p, msg)
		return
	}
	z.crossZone(p, in.zone, in.Entry(), "dungeon")
}

// useStairs: a escada leva ao outro andar, na mesma cópia (o jogador some de um lado e
// aparece do outro; sem sair da zona).
func (z *Zone) useStairs(p *entity, g *GateDef) {
	if p.dead {
		z.system(p, "Você está morto.")
		return
	}
	z.cancelChannel(p, "")
	z.arrive(p, Vec2{X: g.ArriveX, Z: g.ArriveZ})
	z.track(p, "dungeon_stairs", z.mapID(), int64(p.level), nil)
}

// updateInstance roda 1×/s: a vida da cópia (avisos, fechamento e evacuação).
func (z *Zone) updateInstance() {
	r := z.dg
	if r == nil || z.mgr == nil {
		return
	}
	now := z.cfg.Clock()
	if len(z.players) > 0 {
		r.emptySince = time.Time{}
	} else if r.emptySince.IsZero() {
		r.emptySince = now
	}
	age := now.Sub(r.created)
	if !r.closing {
		switch {
		case age >= instanceMaxAge:
			z.closeInstance("tempo")
		case !r.emptySince.IsZero() && now.Sub(r.emptySince) >= cmp.Or(r.emptyTTL, instanceEmptyTTL):
			z.closeInstance("vazia")
		case !r.warned && age >= instanceMaxAge-instanceWarnBefore && len(z.players) > 0 && z.arena == nil:
			r.warned = true
			z.announceAll("As paredes tremem: as Catacumbas desmoronam em 5 minutos. Terminem e saiam.")
		}
		return
	}
	// Fechando: ninguém fica. Quem chegou no último segundo também sai.
	if len(z.players) > 0 {
		z.evacuate()
		r.closeAt = now.Add(instanceCloseGrace)
		return
	}
	if now.Before(r.closeAt) {
		return
	}
	z.finishInstance()
}

// closeInstance fecha a porta e começa a tirar todo mundo.
func (z *Zone) closeInstance(reason string) {
	r := z.dg
	r.closing, r.reason = true, reason
	z.closed.Store(true) // quem entra de novo (login) já não vem para cá: o Join cai no estado do banco
	r.closeAt = z.cfg.Clock().Add(instanceCloseGrace)
	z.mgr.markClosing(r.inst)
	if len(z.players) > 0 && z.arena == nil {
		z.announceAll("As Catacumbas desmoronam. Vocês são levados de volta à igreja.")
	}
	z.evacuate()
}

// evacuate leva todos os jogadores para a porta da igreja (depois do snapshot do tick).
func (z *Zone) evacuate() {
	home := z.mgr.Home()
	exit := dungeonDefs[z.mapID()].exit
	ids := z.sortedPlayerIDs()
	if z.arena != nil { // a arena devolve ao lado do Mestre, com vida cheia (arena.go)
		z.afterFlush(func() {
			for _, id := range ids {
				z.arenaSendHome(z.players[id])
			}
		})
		return
	}
	z.afterFlush(func() {
		for _, id := range ids {
			p := z.players[id]
			if p == nil || p.pl.hop.out != nil {
				continue
			}
			z.crossZone(p, home, exit, "dungeon_exit")
		}
	})
}

// finishInstance anota a telemetria da cópia e para a zona.
func (z *Zone) finishInstance() {
	r := z.dg
	now := z.cfg.Clock()
	extra := map[string]any{
		"instance": r.inst.id, "deaths": r.deaths, "boss_down": r.bossDown, "chests": r.chests,
		"players": z.mgr.allEntered(r.inst), "reason": r.reason,
	}
	if r.bossDown {
		extra["boss_seconds"] = int64(r.bossAfter.Seconds())
	}
	if z.arena == nil { // a arena anota a partida (arena_match) em arena.go
		z.track(nil, "dungeon", z.mapID(), int64(now.Sub(r.created).Seconds()), extra)
	}
	r.closing = false // chamada única
	z.dg = nil
	if r.inst.cancel != nil {
		r.inst.cancel()
	}
}

// announceAll: aviso do sistema para todos da cópia.
func (z *Zone) announceAll(text string) {
	for _, id := range z.sortedPlayerIDs() {
		z.system(z.players[id], text)
	}
}

// dungeonDeath conta a morte de um jogador na cópia (telemetria).
func (z *Zone) dungeonDeath(p *entity) {
	if z.dg != nil && p.pl != nil {
		z.dg.deaths++
	}
}

// dungeonBossKilled: o chefe da cópia caiu. Cada jogador que lutou (ou estava perto) abre o
// baú do dia, uma vez por personagem; quem já abriu hoje fica só com a XP e o saque comum.
func (z *Zone) dungeonBossKilled(m *entity, dealt map[uint32]float64) {
	r := z.dg
	if r == nil {
		return
	}
	r.bossDown = true
	r.bossAfter = z.cfg.Clock().Sub(r.created)
	def := game.DungeonChests[z.mapID()]
	if def == nil {
		return
	}
	day := game.ContractDay(z.cfg.Clock(), z.eventLoc())
	for _, id := range z.sortedPlayerIDs() {
		p := z.players[id]
		if p == nil || p.pl == nil {
			continue
		}
		took := dealt[id] > 0 || (!p.dead && dist(p.pos, m.pos) <= raidNear)
		if !took {
			continue
		}
		if p.pl.quests.chests[def.Dungeon] == day {
			z.systemAfterFlush(p, "Você já abriu o "+def.Name+" hoje: ele volta à meia-noite (Brasília). Aqui você ainda leva a XP e o saque do Abade.")
			continue
		}
		if z.openChest(p, def, day) {
			r.chests++
		}
	}
}

// openChest dá o baú ao jogador: moedas e itens vão direto para a bolsa; o que não couber
// fica no chão, reservado para ele. O dia fica salvo no personagem.
func (z *Zone) openChest(p *entity, def *game.DungeonChestDef, day int) bool {
	drops := game.RollDungeonChest(def, p.pl.class, p.level, z.rng)
	if p.pl.quests.chests == nil {
		p.pl.quests.chests = map[string]int{}
	}
	p.pl.quests.chests[def.Dungeon] = day
	var parts []string
	var gold int64
	var dropped []*groundItem
	for _, d := range drops {
		if d.Item.ID == game.GoldID {
			gold += int64(d.Count)
			continue
		}
		left := p.pl.inv.add(d.Item, d.Count)
		if d.Count > left {
			z.trackItem(p, true, d.Item.ID, d.Count-left, "dungeon_chest")
		}
		if left > 0 {
			if g := z.dropItem(p.pos, d.Item, left, 0, p.id, questRewardReserve); g != nil {
				dropped = append(dropped, g)
			}
		}
		parts = append(parts, fmt.Sprintf("%dx %s", d.Count, d.Item.Name))
	}
	if gold > 0 {
		p.pl.inv.gold += gold
		z.trackGold(p, "dungeon_chest", gold)
		parts = append([]string{fmt.Sprintf("%d moedas", gold)}, parts...)
	}
	z.track(p, "dungeon_chest", def.Dungeon, gold, map[string]any{"items": len(drops), "day": day})
	z.inventoryChanged(p, false)
	z.saveSoon(p)
	text := fmt.Sprintf("%s: %s.", def.Name, strings.Join(parts, ", "))
	z.systemAfterFlush(p, text)
	if len(dropped) > 0 {
		added := &pb.GroundItemsAdded{}
		for _, g := range dropped {
			added.Items = append(added.Items, z.groundPB(g))
		}
		z.afterFlush(func() {
			z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_GroundAdded{GroundAdded: added}}, 0)
		})
		z.systemAfterFlush(p, "Sua bolsa está cheia: parte do baú ficou no chão, reservada para você.")
	}
	return true
}

// ---------- Chat: o geral não atravessa a porta ----------

// broadcastGeneral: o chat geral. Dentro da cópia, fica na cópia; fora, vai a todas as zonas
// menos às cópias.
func (z *Zone) broadcastGeneral(msg *pb.ServerMessage) {
	if z.cfg.Instance != "" || z.mgr == nil {
		z.broadcast(msg, 0)
		return
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		z.log.Error("marshal", "err", err)
		return
	}
	z.mgr.bus.broadcastWorld(data)
}

// broadcastWorld manda os bytes a todos os online que não estão numa cópia.
func (b *Bus) broadcastWorld(data []byte) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, e := range b.players {
		if e.zone != nil && e.zone.cfg.Instance != "" {
			continue
		}
		send(e.peer, data)
	}
}
