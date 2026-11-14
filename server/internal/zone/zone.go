// Package zone contém a simulação autoritativa de uma zona do mapa.
//
// Uma única goroutine (Run) é dona de todo o estado da zona. Conexões falam com ela
// apenas por canal (Join, Leave, Move, Chat, Attack, Skill, Respawn), então não há
// mutex nem condição de corrida. O tempo da simulação (z.now) avança por tick, e o
// sorteio usa um gerador com semente: os testes ficam determinísticos.
package zone

import (
	"context"
	"log/slog"
	"maps"
	"math"
	"math/rand/v2"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/world"
)

// Peer é o lado "conexão" de um jogador, visto pela zona.
type Peer interface {
	// Send enfileira bytes já serializados sem bloquear. Retorna false se a fila estiver cheia.
	Send(msg []byte) bool
	// Close derruba a conexão (cliente lento, comportamento inválido…).
	Close(reason string)
}

type Config struct {
	Size           float64          // lado do quadrado jogável, em metros
	TickRate       int              // ticks por segundo
	MaxMovesPerSec float64          // limite de pedidos de movimento/ação por jogador
	ChatPerSec     float64          // mensagens de chat por segundo (reposição)
	ChatBurst      float64          // rajada máxima de mensagens
	SaveEvery      time.Duration    // intervalo do salvamento periódico
	SafeRadius     float64          // raio da cidade (sem combate)
	Spawns         []game.Spawn     // monstros da zona
	Seed           uint64           // semente do sorteio (posições, dano…)
	World          *world.World     // obstáculos e caminhos (nil = campo aberto)
	Clans          []ClanData       // clãs salvos (carregados na partida)
	Bosses         []BossData       // renascimento dos raid bosses (carregado na partida)
	DevCommands    bool             // comandos de teste no chat (/raid); só com DEV_LOGIN
	Clock          func() time.Time // relógio do ciclo dia/noite (nil = time.Now)
	Version        string           // versão do servidor (vai nos relatos de /bug)
	// Eventos (events.go e invasion.go).
	InvasionChance float64        // chance de invasão da cidade a cada noite (0 = só por GM)
	XP2Days        []time.Weekday // dias de XP em dobro pelo horário (nil = só por GM)
	EventLocation  *time.Location // fuso desses dias (nil = Brasília, UTC−3)
	Events         []EventData    // eventos de GM salvos (carregados na partida)
	// Zonas (manager.go): id e nome do mapa, portões (gate.go) e mapa sem a cidade (sem
	// os NPCs da praça, as pedras de teleporte e o armazém).
	Map      string // "" = starter
	MapTitle string
	Gates    []GateDef
	NoTown   bool
}

func DefaultConfig() Config {
	return Config{
		Size: 300, TickRate: 20, MaxMovesPerSec: 10,
		ChatPerSec: 1, ChatBurst: 3, SaveEvery: 30 * time.Second,
		SafeRadius: 40, Spawns: game.DefaultSpawns(), Seed: 20261112,
		InvasionChance: defaultInvasionChance, XP2Days: []time.Weekday{time.Saturday, time.Sunday},
	}
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9]{3,16}$`)

// ValidName informa se o nome do personagem é aceito (3–16 letras/números).
func ValidName(name string) bool { return nameRe.MatchString(name) }

// JoinInfo é o personagem já carregado e validado por quem chama.
type JoinInfo struct {
	ID     uint32
	Name   string
	Class  string
	X, Z   float64
	XP     int64
	HP, MP int // <= 0 = cheio
	Gold   int64
	Items  []game.ItemRecord
	// Progressão: pontos de atributo gastos, ranks da árvore (nil = só as iniciais) e
	// barra de atalhos (nil = padrão).
	Attrs  game.Attrs
	Ranks  map[uint32]int
	Hotbar []string
	Emblem string // cor do emblema ("" = padrão da classe)
	Look   game.Appearance
	// Fase 4: conta (armazém) e PvP.
	AccountID int64
	Karma     int
	PKs       int
	PvPKills  int
	// ClanLeftAt: quando saiu do último clã (zero = nunca).
	ClanLeftAt time.Time
	// Storage: o armazém da conta como está no banco (nil = vazio).
	Storage *StorageData
	// Moderação: GM da conta, silêncio no chat (zero = livre) e navegador (para o /bug).
	GM         bool
	MutedUntil time.Time
	UserAgent  string
	Quests     game.QuestState
	// Conquistas desbloqueadas/progresso e a conquista do título em uso ("" = nenhum).
	Achievements game.AchievementState
	Title        string
	// Especialização (segunda classe; "" = nenhuma).
	Spec string
	// Maestria (nível máximo): barra, bônus diário e pontos da árvore.
	Mastery game.MasteryState
	// LoadedAt: quando quem chama começou a ler o personagem do banco (o portal recusa
	// um Join carregado antes de uma mudança sua; ver admin_actions.go).
	LoadedAt time.Time
	Peer     Peer
	// Zone: o mapa gravado no banco (o gerente escolhe a zona; "" = starter).
	Zone string
	hop  *hopIn // só na entrega do portão (gate.go)
}

// SavedState é enviado ao canal de persistência.
type SavedState struct {
	ID       uint32
	X, Z     float64
	Level    int
	XP       int64
	HP, MP   int // -1 = cheio
	Gold     int64
	Items    []game.ItemRecord
	Attrs    game.Attrs
	Ranks    map[uint32]int
	Hotbar   []string
	Karma    int
	PKs      int
	PvPKills int
	Look     game.Appearance
	Quests   game.QuestState
	// Conquistas e título (id da conquista), salvos junto com o personagem.
	Achievements game.AchievementState
	Title        string
	Spec         string // especialização ("" = nenhuma)
	Mastery      game.MasteryState
	// Storage: o armazém da conta, só quando mudou (vai na mesma transação).
	Storage *StorageSave
	// Seq cresce a cada salvamento: o banco ignora um estado mais velho que o já gravado.
	Seq int64
	// Zone: o mapa onde o personagem está (no portão, o de destino).
	Zone string
}

// recentState é o último estado de quem saiu. Ao voltar, vale mais que o banco (o
// salvamento pode ainda não ter terminado): relogar rápido não perde nem duplica itens.
type recentState struct {
	state SavedState
	at    time.Time
}

// recentKeep: por quanto tempo a zona lembra o estado de quem saiu.
const recentKeep = 10 * time.Minute

type event interface{ isEvent() }

type joinEvent struct{ info JoinInfo }
type leaveEvent struct {
	id   uint32
	peer Peer // só remove se ainda for a mesma conexão (evita derrubar um relogin)
}
type moveEvent struct {
	id   uint32
	x, z float64
}
type chatEvent struct {
	id       uint32
	text, to string
	channel  pb.ChatChannel
}
type attackEvent struct {
	id, target uint32
	force      bool
}
type skillEvent struct {
	id, skill, target uint32
	force             bool
}
type respawnEvent struct{ id uint32 }

// stopEvent: soltou o WASD/joystick. O servidor para onde está; o cliente não sabe o ponto
// exato (latência), e mandar um destino faria o personagem voltar e virar.
type stopEvent struct{ id uint32 }

func (joinEvent) isEvent()    {}
func (leaveEvent) isEvent()   {}
func (moveEvent) isEvent()    {}
func (stopEvent) isEvent()    {}
func (chatEvent) isEvent()    {}
func (attackEvent) isEvent()  {}
func (skillEvent) isEvent()   {}
func (respawnEvent) isEvent() {}

// frame acumula o que aconteceu desde o último snapshot.
type frame struct {
	combat   []*pb.CombatEvent
	effects  []*pb.EffectEvent
	deaths   []uint32
	revives  []*pb.Revive
	levelUps []*pb.LevelUp
	ends     []*pb.EffectEnd
	casts    []*pb.SkillCast
	// Avisos no chão das habilidades de monstro (começo, golpe e cancelamento).
	telegraphs []*pb.Telegraph
}

type Zone struct {
	cfg      Config
	log      *slog.Logger
	events   chan event
	players  map[uint32]*entity
	monsters map[uint32]*entity
	byName   map[string]uint32 // nome em minúsculas → id
	tick     uint32
	now      float64 // segundos de simulação
	rng      *rand.Rand
	frame    frame
	// Mensagens que só podem sair depois do snapshot do tick (saque e XP de uma morte
	// chegariam ao cliente antes do golpe que matou).
	afterSnapshot []func()
	saves         chan<- []SavedState
	night         bool // ciclo dia/noite (ver game.DayPhase)
	// Recargas de quem saiu (personagem → skill → quando fica pronta, no relógio de
	// parede): sair e entrar de novo não zera as recargas.
	savedCooldowns map[uint32]map[uint32]time.Time
	npcs           map[uint32]*entity
	ground         map[uint32]*groundItem
	groundSeq      uint32
	recent         map[uint32]recentState
	objects        map[uint32]*entity // objetos temporários (estandartes)
	objectSeq      uint32
	nextAura       float64
	pending        map[uint32]SavedState // salvamentos que não couberam na fila
	lastSeq        int64
	parties        map[uint32]*party
	partySeq       uint32
	invites        map[uint32]*invite
	inviteSeq      uint32
	clans          map[int64]*clan
	clanOf         map[int64]*clan     // personagem → clã (online ou não)
	clanLeft       map[int64]time.Time // saídas recentes (o banco pode ainda não ter gravado)
	clanCreating   map[uint32]*clanCreate
	clanJobs       chan clanJob
	clanBackend    ClanBackend
	localClanSeq   int64
	storages       map[int64]*storage // conta → armazém
	stats          tickStats
	aoiDirty       bool // alguém entrou, saiu ou renasceu: atualizar a AOI já neste tick

	modJobs       chan modJob
	modBackend    ModBackend
	telemetry     chan GameEvent
	eventSink     EventSink
	droppedEvents int
	xpTally       xpTallies     // XP por fonte, gravada a cada minuto (xptally.go)
	stopped       chan struct{} // fechado quando a zona para (a telemetria espera o fim das sessões)
	telemetryDone chan struct{} // fechado quando a telemetria gravou o que faltava (TelemetryDone)
	telemetryOnce sync.Once
	bossBackend   BossBackend
	bossJobs      chan BossData
	telegraphs    []*telegraph
	telegraphSeq  uint32
	summonSeq     uint32
	socialJobs    chan socialJob
	socialBackend SocialBackend
	mailSent      map[uint32][]time.Time // cartas enviadas na última hora, por remetente

	rankingBackend RankingBackend
	ranking        *rankingCache // último cálculo do ranking (nil = ainda não chegou)

	gone       map[uint32]bool // personagens de contas apagadas (ver account.go)
	invariants invariantGuard  // checagens do estado (ver invariants.go)

	errs zoneErrors // panics, tick lento e erros do banco (ver errors.go)

	inv          *invasion // invasão da cidade em andamento (nil = nenhuma)
	xp2          xp2State
	eventBackend EventBackend
	eventJobs    chan EventData

	paths []*entity   // fila de caminhos longos (ver pathqueue.go)
	adm   *adminState // portal de administração (admin_actions.go e admin_ops.go)

	mgr *Manager // zonas do processo e barramento (nil = zona sozinha, como nos testes)
}

// New cria a zona e seus monstros. saves pode ser nil (sem persistência, ex.: testes).
func New(cfg Config, log *slog.Logger, saves chan<- []SavedState) *Zone {
	z := &Zone{
		cfg:            cfg,
		log:            log,
		events:         make(chan event, 4096),
		objects:        make(map[uint32]*entity),
		players:        make(map[uint32]*entity),
		monsters:       make(map[uint32]*entity),
		byName:         make(map[string]uint32),
		savedCooldowns: map[uint32]map[uint32]time.Time{},
		npcs:           map[uint32]*entity{},
		ground:         map[uint32]*groundItem{},
		recent:         map[uint32]recentState{},
		pending:        map[uint32]SavedState{},
		parties:        map[uint32]*party{},
		invites:        map[uint32]*invite{},
		clans:          map[int64]*clan{},
		clanOf:         map[int64]*clan{},
		clanLeft:       map[int64]time.Time{},
		clanCreating:   map[uint32]*clanCreate{},
		clanJobs:       make(chan clanJob, clanJobQueue),
		storages:       map[int64]*storage{},
		modJobs:        make(chan modJob, modJobQueue),
		telemetry:      make(chan GameEvent, telemetryQueue),
		stopped:        make(chan struct{}),
		telemetryDone:  make(chan struct{}),
		bossJobs:       make(chan BossData, 16),
		eventJobs:      make(chan EventData, 16),
		socialJobs:     make(chan socialJob, socialJobQueue),
		mailSent:       map[uint32][]time.Time{},
		rng:            rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^0x9e3779b97f4a7c15)),
		saves:          saves,
	}
	if z.cfg.Clock == nil {
		z.cfg.Clock = time.Now
	}
	z.night = game.IsNight(game.DayPhase(z.cfg.Clock()))
	z.spawnMonsters()
	if !cfg.NoTown {
		z.spawnNPCs()
	}
	z.spawnGates()
	z.loadClans(cfg.Clans)
	z.loadEvents(cfg.Events)
	return z
}

// ---------- API usada pelas conexões (thread-safe via canal) ----------

func (z *Zone) Join(ctx context.Context, info JoinInfo) { z.post(ctx, joinEvent{info: info}) }
func (z *Zone) Leave(ctx context.Context, id uint32, peer Peer) {
	z.post(ctx, leaveEvent{id: id, peer: peer})
}
func (z *Zone) Move(ctx context.Context, id uint32, x, z2 float64) {
	z.post(ctx, moveEvent{id: id, x: x, z: z2})
}
func (z *Zone) Stop(ctx context.Context, id uint32) { z.post(ctx, stopEvent{id: id}) }
func (z *Zone) Chat(ctx context.Context, id uint32, text, to string, channel pb.ChatChannel) {
	z.post(ctx, chatEvent{id: id, text: text, to: to, channel: channel})
}

// Social repassa um pedido de grupo, clã, resposta a convite, amigos ou correio
// (*pb.PartyCmd, *pb.ClanCmd, *pb.InviteReply, *pb.FriendCmd, *pb.MailCmd).
func (z *Zone) Social(ctx context.Context, id uint32, msg any) {
	z.post(ctx, socialEvent{id: id, msg: msg})
}
func (z *Zone) Attack(ctx context.Context, id, target uint32, force bool) {
	z.post(ctx, attackEvent{id: id, target: target, force: force})
}
func (z *Zone) UseSkill(ctx context.Context, id, skill, target uint32, force bool) {
	z.post(ctx, skillEvent{id: id, skill: skill, target: target, force: force})
}
func (z *Zone) Respawn(ctx context.Context, id uint32) { z.post(ctx, respawnEvent{id: id}) }

// ClientStats repassa a amostra de desempenho do cliente (telemetria).
func (z *Zone) ClientStats(ctx context.Context, id uint32, msg *pb.ClientStats) {
	z.post(ctx, clientStatsEvent{id: id, msg: msg})
}

type clientStatsEvent struct {
	id  uint32
	msg *pb.ClientStats
}

func (clientStatsEvent) isEvent() {}

// Trade repassa um comando de troca entre jogadores.
func (z *Zone) Trade(ctx context.Context, id uint32, msg *pb.TradeCmd) {
	z.post(ctx, tradeEvent{id: id, msg: msg})
}

// ItemAction repassa um pedido de item (*pb.PickUp, *pb.MoveItem, *pb.UseItem,
// *pb.DestroyItem, *pb.Buy, *pb.Sell).
func (z *Zone) ItemAction(ctx context.Context, id uint32, msg any) {
	z.post(ctx, itemEvent{id: id, msg: msg})
}

func (z *Zone) post(ctx context.Context, e event) {
	select {
	case z.events <- e:
	case <-ctx.Done():
	}
}

// Run executa o loop da zona até ctx ser cancelado. Ao terminar, salva todos os
// jogadores e fecha o canal de persistência.
func (z *Zone) Run(ctx context.Context) {
	dt := time.Second / time.Duration(z.cfg.TickRate)
	ticker := time.NewTicker(dt)
	defer ticker.Stop()
	saveTicker := time.NewTicker(z.cfg.SaveEvery)
	defer saveTicker.Stop()
	// Workers com recover: um panic é registrado e o worker recomeça (ver errors.go).
	z.goWorker(ctx, "clan", z.runClanWorker)
	z.goWorker(ctx, "mod", z.runModWorker)
	z.goWorker(ctx, "telemetry", z.runTelemetryWorker)
	z.goWorker(ctx, "boss", z.runBossWorker)
	z.goWorker(ctx, "social", z.runSocialWorker)
	z.goWorker(ctx, "ranking", z.runRankingWorker)
	z.goWorker(ctx, "event", z.runEventWorker)

	for {
		select {
		case <-ctx.Done():
			z.runSystem(sysXPTally, z.endSessions) // fim de sessão e XP de quem estava online (telemetry.go)
			close(z.stopped)
			z.saveAll(true)
			if z.saves != nil {
				close(z.saves)
			}
			return
		case e := <-z.events:
			z.stats.events++
			z.handleSafe(e)
		case <-ticker.C:
			start := time.Now()
			z.update(dt.Seconds())
			took := time.Since(start)
			z.stats.add(took)
			z.checkSlowTick(took)
			z.flushTickStats(start)
		case <-saveTicker.C:
			z.runSystem(sysSaveAll, func() { z.saveAll(false) })
		}
	}
}

func (z *Zone) handle(e event) {
	switch e := e.(type) {
	case joinEvent:
		z.onJoin(e.info)
	case leaveEvent:
		if p, ok := z.players[e.id]; ok && p.pl.peer == e.peer {
			z.removePlayer(e.id)
		}
	case moveEvent:
		z.onMove(e)
	case stopEvent:
		z.onStop(e)
	case chatEvent:
		z.onChat(e)
	case attackEvent:
		if p := z.players[e.id]; p != nil && !p.dead && z.spendAction(p) {
			z.endEmote(p)
			z.requestAttack(p, e.target, e.force)
		}
	case skillEvent:
		if p := z.players[e.id]; p != nil && z.spendAction(p) {
			z.endEmote(p)
			z.requestSkill(p, e.skill, e.target, e.force)
		}
	case respawnEvent:
		if p := z.players[e.id]; p != nil {
			z.respawnPlayer(p)
		}
	case itemEvent:
		z.onItem(e)
	case progressEvent:
		z.onProgress(e)
	case socialEvent:
		z.onSocial(e)
	case clanCreatedEvent:
		z.onClanCreated(e)
	case modResultEvent:
		z.onModResult(e)
	case tradeEvent:
		z.onTrade(e)
	case rankingEvent:
		z.setRanking(e.data)
	case clientStatsEvent:
		if p := z.players[e.id]; p != nil {
			z.onClientStats(p, e.msg)
		}
	case socialDone:
		e.fn()

	case accountGoneEvent:
		z.onAccountGone(e.d)
	}
}

// spendAction limita pedidos de ação (movimento, ataque, skill) por jogador.
func (z *Zone) spendAction(p *entity) bool {
	if p.pl.moveTokens < 1 {
		return false
	}
	p.pl.moveTokens--
	return true
}

func (z *Zone) get(id uint32) *entity {
	switch {
	case id >= monsterIDBase:
		return z.monsters[id]
	case id >= objectIDBase:
		return z.objects[id]
	}
	return z.players[id]
}

func (z *Zone) onJoin(info JoinInfo) {
	if z.refuseGone(info) || z.refuseAdminStale(info) {
		return
	}
	// Mesmo personagem conectado em outro lugar: derruba a sessão antiga.
	if old, ok := z.players[info.ID]; ok {
		z.sendTo(old, &pb.ServerMessage{Msg: &pb.ServerMessage_Kicked{Kicked: &pb.Kicked{
			Reason: "Este personagem entrou em outro lugar.",
		}}})
		old.pl.peer.Close("relogin")
		z.removePlayer(info.ID)
	}

	// O estado de quem acabou de sair (ainda salvando) vale mais que o do banco.
	if r, ok := z.recent[info.ID]; ok {
		st := r.state
		info.X, info.Z, info.XP, info.HP, info.MP, info.Gold, info.Items = st.X, st.Z, st.XP, st.HP, st.MP, st.Gold, st.Items
		info.Attrs, info.Ranks, info.Hotbar = st.Attrs, st.Ranks, st.Hotbar
		info.Karma, info.PKs, info.PvPKills = st.Karma, st.PKs, st.PvPKills
		info.Look = st.Look
		info.Quests = st.Quests
		info.Achievements, info.Title = st.Achievements, st.Title
		info.Spec = st.Spec
		info.Mastery = st.Mastery
		delete(z.recent, info.ID)
	}

	level := game.LevelForXP(info.XP)
	pos := z.freeSpot(clampToZone(Vec2{X: info.X, Z: info.Z}, z.bounds()))
	spec := ""
	if game.SpecFor(info.Class, info.Spec) != nil {
		spec = info.Spec
	}
	attrs, ranks, hotbar := loadProgress(info.Class, spec, level, info.Attrs, info.Ranks, info.Hotbar)
	inv, skipped := loadInventory(info.Gold, info.Items)
	if skipped > 0 {
		z.log.Warn("items skipped on load", "id", info.ID, "count", skipped)
	}
	p := &entity{
		id: info.ID, kind: pb.EntityKind_ENTITY_KIND_PLAYER,
		name: info.Name, model: info.Class, level: level, scale: 1,
		pos: pos, dest: pos,
		lastCombat: -math.MaxFloat64 / 2,
		pl: &playerState{
			peer: info.Peer, class: info.Class, xp: info.XP, emblem: game.EmblemOr(info.Class, info.Emblem),
			look:       game.NormalAppearance(info.Class, info.Look),
			cooldowns:  map[uint32]float64{},
			moveTokens: z.cfg.MaxMovesPerSec, chatTokens: z.cfg.ChatBurst,
			inv:   inv,
			attrs: attrs, ranks: ranks, hotbar: hotbar, spec: spec,
			accountID: info.AccountID,
			karma:     max(0, info.Karma), pks: max(0, info.PKs), pvpKills: max(0, info.PvPKills),
			gm: info.GM, mutedUntil: info.MutedUntil, userAgent: truncateRunes(info.UserAgent, maxUserAgent),
			quests:  loadQuests(info.Quests, level),
			ach:     loadAchievements(info.Achievements, info.Title),
			mastery: loadMastery(info.Mastery),
			hop:     hopState{arrived: info.hop != nil},
		},
	}
	p.pl.sentPvp = z.pvpState(p)
	p.hp, p.mp = math.MaxFloat64, math.MaxFloat64
	z.refreshStats(p) // classe + atributos + equipamento + passivas; limita a vida/mana ao máximo
	if info.HP > 0 && float64(info.HP) < p.stats.MaxHP {
		p.hp = float64(info.HP)
	}
	if info.MP > 0 && float64(info.MP) < p.stats.MaxMP {
		p.mp = float64(info.MP)
	}
	p.sentHP, p.sentMaxHP = p.hpInt(), p.maxHPInt()
	z.restoreCooldowns(p)
	z.players[p.id] = p
	z.byName[strings.ToLower(p.name)] = p.id
	z.clanOnJoin(p, info.ClanLeftAt)
	z.storageOnJoin(p, info.Storage)

	// Os outros ficam sabendo pela AOI (updateAOI, no fim do tick), só quem está perto.
	z.aoiDirty = true
	// O novo jogador recebe quem ele é e o que está no raio dele, mais NPCs e objetos.
	entities := z.welcomeEntities(p)
	for _, n := range z.npcs {
		entities = append(entities, z.entityPB(n))
	}
	for _, o := range z.objects {
		entities = append(entities, z.entityPB(o))
	}
	// A árvore inteira da classe (ativas e passivas); o cliente sabe o rank por Progress.
	var skills []*pb.SkillInfo
	for _, s := range game.ClassTree(info.Class) {
		skills = append(skills, skillInfoPB(s))
	}
	// E as dos dois caminhos da classe (a janela da Mestra mostra as duas; a árvore, a escolhida).
	for _, sp := range game.ClassSpecs(info.Class) {
		for _, s := range game.SpecTree(sp.ID) {
			skills = append(skills, skillInfoPB(s))
		}
	}
	var branches []*pb.SkillBranch
	if c := game.Classes[info.Class]; c != nil {
		for _, b := range c.Branches {
			branches = append(branches, &pb.SkillBranch{Name: b.Name, Description: b.Description, Color: b.Color})
		}
	}
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Welcome{Welcome: &pb.Welcome{
		YourId:       p.id,
		TickRate:     uint32(z.cfg.TickRate),
		ZoneSize:     float32(z.cfg.Size),
		Entities:     entities,
		Skills:       skills,
		SafeRadius:   float32(z.cfg.SafeRadius),
		Zones:        mapZones(z.cfg.Spawns),
		ServerTimeMs: z.cfg.Clock().UnixMilli(),
		DayCycleMs:   uint32(game.DayCycle.Milliseconds()),
		SkillCatalog: skillCatalog(),
		ItemCatalog:  itemCatalog,
		Inventory:    inventoryPB(&p.pl.inv),
		GroundItems:  z.groundList(),
		Npcs:         z.npcList(),
		Progress:     z.progressPB(p),
		Hotbar:       &pb.Hotbar{Slots: p.pl.hotbar},
		Branches:     branches,
		Pvp:          z.selfPvpPB(p),
		Clan:         z.clanPB(p),
		Enchant:      enchantRulesPB(),
		Bosses:       z.bossList(),
		Specs:        specCatalog(),
		Map:          z.mapID(),
		MapTitle:     z.mapTitle(),
	}}})
	z.sendSelfStats(p)
	z.sendQuests(p, "")
	z.questLevel(p) // objetivo de nível já cumprido (ex.: subiu antes da missão existir)
	z.achOnJoin(p)
	z.masteryOnJoin(p)
	z.eventsOnJoin(p)
	z.adminOpsOnJoin(p)
	for id, ready := range p.pl.cooldowns {
		if ready > z.now {
			z.sendAnyCooldown(p, id, ready-z.now)
		}
	}

	p.pl.joinedAt = z.cfg.Clock()
	if info.hop != nil { // chegou pelo portão: a sessão é a mesma
		p.pl.joinedAt = info.hop.joinedAt
	}
	z.socialOnJoin(p)
	if info.hop == nil {
		z.track(p, "session", "start", 0, nil)
	}
	z.zonesOnJoin(p) // barramento e grupo entre zonas (gate.go)
	z.log.Info("player joined", "id", p.id, "name", p.name, "level", p.level, "online", len(z.players))
}

func (z *Zone) onMove(e moveEvent) {
	p, ok := z.players[e.id]
	if !ok || p.dead || !z.spendAction(p) {
		return
	}
	// NaN passa pelo clamp (min/max com NaN dão NaN) e deixaria o personagem fora do mapa
	// para sempre; ±Inf o clamp já leva para a borda.
	if math.IsNaN(e.x) || math.IsNaN(e.z) {
		return
	}
	p.clearAction() // clicar no chão cancela o ataque
	z.cancelChannel(p, "Você se moveu: viagem cancelada.")
	if p.stunned(z.now) {
		return
	}
	z.requestPath(p, clampToZone(Vec2{X: e.x, Z: e.z}, z.bounds()))
}

// onStop para o andar livre. Não gasta token (parar não é abusável, e perder o pedido
// deixaria o personagem seguir sozinho) e não cancela ataque nem canal.
func (z *Zone) onStop(e stopEvent) {
	p, ok := z.players[e.id]
	if !ok || p.dead || p.act.kind != actNone {
		return
	}
	p.pl.pathGoal = nil
	p.stopMoving()
}

func (z *Zone) onChat(e chatEvent) {
	p, ok := z.players[e.id]
	if !ok {
		return
	}
	text := SanitizeChat(e.text)
	if text == "" {
		return
	}
	if p.pl.chatTokens < 1 {
		z.system(p, "Calma! Você está enviando mensagens rápido demais.")
		return
	}
	p.pl.chatTokens--

	if isCommand(text, e.to, e.channel) {
		z.command(p, text)
		return
	}
	if left := z.mutedFor(p); left > 0 {
		z.system(p, "Você está silenciado por mais "+minutesText(left)+".")
		return
	}

	switch e.channel {
	case pb.ChatChannel_CHAT_CHANNEL_PARTY:
		z.partyChat(p, text)
		return
	case pb.ChatChannel_CHAT_CHANNEL_CLAN:
		z.clanChat(p, text)
		return
	}

	to := strings.TrimSpace(e.to)
	if to == "" { // todas as zonas (bus.go)
		z.broadcastAll(&pb.ServerMessage{Msg: &pb.ServerMessage_Chat{Chat: &pb.ChatMessage{
			Channel: pb.ChatChannel_CHAT_CHANNEL_GENERAL, FromId: p.id, FromName: p.name, Text: text,
		}}})
		return
	}

	targetID, ok := z.byName[strings.ToLower(to)]
	if !ok {
		if !z.whisperElsewhere(p, to, text) {
			z.system(p, to+" não está online.")
		}
		return
	}
	target := z.players[targetID]
	msg := &pb.ServerMessage{Msg: &pb.ServerMessage_Chat{Chat: &pb.ChatMessage{
		Channel: pb.ChatChannel_CHAT_CHANNEL_WHISPER, FromId: p.id, FromName: p.name, ToName: target.name, Text: text,
	}}}
	z.sendTo(target, msg)
	if target.id != p.id {
		z.sendTo(p, msg) // eco para quem enviou
	}
}

func (z *Zone) system(p *entity, text string) {
	if p.pl == nil {
		return
	}
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Chat{Chat: &pb.ChatMessage{
		Channel: pb.ChatChannel_CHAT_CHANNEL_SYSTEM, Text: text,
	}}})
}

func (z *Zone) removePlayer(id uint32) {
	p, ok := z.players[id]
	if !ok {
		return
	}
	hop := z.hopping(p) // saindo pelo portão: sessão e grupo continuam (gate.go)
	delete(z.players, id)
	z.saveCooldowns(p)
	if z.byName[strings.ToLower(p.name)] == id {
		delete(z.byName, strings.ToLower(p.name))
	}
	z.forgetTarget(id)
	z.endXPTally(p)
	if !hop {
		z.track(p, "session", "end", int64(z.cfg.Clock().Sub(p.pl.joinedAt).Seconds()), nil)
	}
	if p.pl.trade != nil {
		z.closeTrade(p.pl.trade, p.name+" saiu do jogo: troca cancelada.")
	}
	if !hop {
		z.partyLeave(p, "")
	}
	z.dropInvites(id)
	z.clanOnLeave(p)
	z.socialOnLeave(p)
	if !z.dropAfterPanic(p) { // caiu por panic: não grava o estado atual (ver errors.go)
		st := z.savedState(p)
		if !z.handOff(p, &st) { // portão: o estado segue para a outra zona, não para o recent
			z.recent[id] = recentState{state: st, at: z.cfg.Clock()}
		}
		z.save([]SavedState{st}, false)
	}
	z.zonesOnLeave(p, hop)
	// Quem o conhecia recebe o Despawn pela AOI no fim do tick.
	p.gone = true
	z.aoiDirty = true
	z.log.Info("player left", "id", id, "name", p.name, "online", len(z.players))
}

// update avança a simulação um tick e envia o delta para todos. Cada sistema roda em
// runSystem (recover e tempo); cada jogador e monstro tem o seu recover (ver errors.go).
func (z *Zone) update(dt float64) {
	z.tick++
	z.now += dt
	z.runSystem(sysDaytime, z.updateDaytime)
	z.runSystem(sysPaths, z.updatePaths)

	if z.now >= z.nextAura {
		z.nextAura = z.now + auraEvery
		z.runSystem(sysAuras, z.updateAuras)
	}
	z.runSystem(sysPlayers, func() {
		for _, p := range z.players {
			z.guardPlayer(p, func() {
				p.pl.moveTokens = math.Min(z.cfg.MaxMovesPerSec, p.pl.moveTokens+z.cfg.MaxMovesPerSec*dt)
				p.pl.chatTokens = math.Min(z.cfg.ChatBurst, p.pl.chatTokens+z.cfg.ChatPerSec*dt)
				z.step(p, dt)
				z.checkEmote(p)
			})
		}
	})
	z.runSystem(sysMonsters, func() {
		for _, m := range z.monsters {
			z.guardMonster(m, func() {
				if m.dead {
					z.updateDeadMonster(m)
					return
				}
				z.think(m)
				z.step(m, dt)
			})
		}
	})
	z.runSystem(sysTelegraphs, z.updateTelegraphs)
	z.runSystem(sysInvasion, z.updateInvasion)

	if z.tick%uint32(z.cfg.TickRate) == 0 { // 1×/s
		z.runSystem(sysGround, z.expireGround)
		z.runSystem(sysFlags, z.expireFlags)
		z.runSystem(sysInvites, z.expireInvites)
		z.runSystem(sysParties, z.syncParties)
		z.runSystem(sysLeaveTown, z.checkLeaveTown)
		z.runSystem(sysTrades, z.checkTrades)
		z.runSystem(sysXP2, z.updateXP2)
		z.runSystem(sysXPTally, z.flushXPTallies)
		z.runSystem(sysAdminOps, z.updateAdminOps)
		if len(z.pending) > 0 {
			z.runSystem(sysPendingSaves, func() { z.save(nil, false) })
		}
	}
	z.runSystem(sysInvariants, z.checkInvariantsDue) // cada jogador 1×/s, espalhado entre os ticks
	z.runSystem(sysSaveDue, z.saveDue)

	z.runSystem(sysSnapshot, z.flushSnapshot)
	z.runSystem(sysAfterSnapshot, z.runAfterSnapshot)
	z.runSystem(sysSelfStats, func() {
		for _, p := range z.players {
			if p.pl.statsDirty || p.hpInt() != p.pl.sentSelfHP || p.mpInt() != p.pl.sentMP {
				z.sendSelfStats(p)
			}
		}
	})
}

// step: efeitos, regeneração, ação e movimento de uma entidade viva.
func (z *Zone) step(e *entity, dt float64) {
	if !e.dead {
		z.tickDoTs(e) // antes de vencer: o último segundo também dói
	}
	for _, k := range e.pruneEffects(z.now) {
		z.frame.ends = append(z.frame.ends, &pb.EffectEnd{TargetId: e.id, Effect: pb.Effect(k)})
		if k == game.EffectSlow || k == game.EffectBuffSpeed {
			e.moved = true
		}
		if k == game.EffectInvisible && e.pl != nil {
			e.pl.ambushCrit = false // a invisibilidade acabou sem atacar
		}
		if e.pl != nil {
			e.pl.statsDirty = true
		}
	}
	if e.dead {
		return
	}
	if e.pl != nil {
		z.updateChannel(e)
	}
	z.regen(e, dt)
	z.runAction(e)
	if e.rooted(z.now) {
		if e.dest != e.pos {
			e.stopMoving()
		}
		return
	}
	z.advance(e, e.speed(z.now)*dt)
}

func (z *Zone) flushSnapshot() {
	snap := &pb.Snapshot{Tick: z.tick}
	collect := func(e *entity) {
		e.encState, e.encHP = e.encState[:0], e.encHP[:0]
		if e.moved && !e.dead {
			st := &pb.EntityState{
				Id: e.id,
				X:  float32(e.pos.X), Z: float32(e.pos.Z),
				TargetX: float32(e.dest.X), TargetZ: float32(e.dest.Z),
				Speed: float32(e.speed(z.now)), Teleport: e.teleport,
			}
			snap.States = append(snap.States, st)
			e.encState = encField(snapFieldStates, st)
		}
		e.moved, e.teleport = false, false
		if hp, max := e.hpInt(), e.maxHPInt(); hp != e.sentHP || max != e.sentMaxHP {
			h := &pb.HpUpdate{Id: e.id, Hp: uint32(hp), MaxHp: uint32(max)}
			snap.Hp = append(snap.Hp, h)
			e.encHP = encField(snapFieldHp, h)
			e.sentHP, e.sentMaxHP = hp, max
		}
		e.encTick = z.tick
	}
	for _, p := range z.players {
		collect(p)
	}
	for _, m := range z.monsters {
		if !m.mon.hidden {
			collect(m)
		}
	}
	snap.Combat = z.frame.combat
	snap.Effects = z.frame.effects
	snap.Deaths = z.frame.deaths
	snap.Revives = z.frame.revives
	snap.LevelUps = z.frame.levelUps
	snap.EffectEnds = z.frame.ends
	snap.Casts = z.frame.casts
	snap.Telegraphs = z.frame.telegraphs
	z.frame = frame{}

	// Spawn/Despawn de quem entrou ou saiu do raio, e cada um recebe só o que conhece.
	if z.tick%aoiEvery == 0 || z.aoiDirty {
		z.aoiDirty = false
		z.updateAOI()
	}
	if snapshotEmpty(snap) {
		return
	}
	z.sendSnapshots(snap)
}

func (z *Zone) sendSelfStats(p *entity) {
	p.pl.statsDirty = false
	p.pl.sentSelfHP = p.hpInt()
	p.pl.sentMP = p.mpInt()
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_SelfStats{SelfStats: z.selfStats(p)}})
}

func (z *Zone) savedState(p *entity) SavedState {
	z.checkInvariants(p) // nunca salva um estado quebrado sem corrigir (ou avisar)
	s := SavedState{
		ID: p.id, X: p.pos.X, Z: p.pos.Z, Level: p.level, XP: p.pl.xp, HP: p.hpInt(), MP: p.mpInt(),
		Gold: p.pl.inv.gold, Items: p.pl.inv.records(), Seq: z.nextSeq(),
		Attrs: p.pl.attrs, Ranks: maps.Clone(p.pl.ranks), Hotbar: slices.Clone(p.pl.hotbar),
		Karma: p.pl.karma, PKs: p.pl.pks, PvPKills: p.pl.pvpKills, Look: p.pl.look,
		Quests: p.pl.quests.state(), Storage: z.storageSave(p),
		Achievements: p.pl.ach.state(), Title: p.pl.ach.title, Spec: p.pl.spec,
		Mastery: p.pl.mastery.save(), Zone: z.mapID(),
	}
	if p.dead { // quem sai morto volta na cidade, inteiro (a XP já foi descontada)
		s.X, s.Z, s.HP, s.MP = 0, 0, -1, -1
		if z.cfg.NoTown {
			s.Zone = world.StarterMap
		}
	}
	if s.HP >= p.maxHPInt() {
		s.HP = -1
	}
	if s.MP >= p.maxMPInt() {
		s.MP = -1
	}
	return s
}

// nextSeq: relógio em nanossegundos, sempre crescente (vale entre reinícios do servidor).
func (z *Zone) nextSeq() int64 {
	z.lastSeq = max(z.lastSeq+1, time.Now().UnixNano())
	return z.lastSeq
}

func (z *Zone) saveAll(blocking bool) {
	wall := z.cfg.Clock()
	z.pruneStorages()
	for id, r := range z.recent {
		if wall.Sub(r.at) > recentKeep {
			delete(z.recent, id)
		}
	}
	batch := make([]SavedState, 0, len(z.players))
	for _, p := range z.players {
		p.pl.saveAt = 0
		batch = append(batch, z.savedState(p))
	}
	z.save(batch, blocking)
}

// saveDue salva logo quem trocou itens ou moedas (compra, venda, coleta).
func (z *Zone) saveDue() {
	var batch []SavedState
	for _, p := range z.players {
		if p.pl.saveAt > 0 && z.now >= p.pl.saveAt {
			p.pl.saveAt = 0
			batch = append(batch, z.savedState(p))
		}
	}
	if len(batch) > 0 {
		z.save(batch, false)
	}
}

// save entrega estados à goroutine de persistência. Fora do desligamento nunca
// bloqueia: se a fila estiver cheia, os estados ficam pendentes e são reenviados
// (o mais novo de cada personagem) na próxima tentativa.
func (z *Zone) save(batch []SavedState, blocking bool) {
	if z.saves == nil {
		return
	}
	z.noteSaved(batch) // o último estado salvo vale depois de um panic (ver errors.go)
	if len(z.pending) > 0 {
		merged := make([]SavedState, 0, len(z.pending)+len(batch))
		newer := map[uint32]bool{}
		for i, s := range batch {
			newer[s.ID] = true
			// O armazém só vai no salvamento em que mudou: o mais novo herda o do pendente.
			if old, ok := z.pending[s.ID]; ok && s.Storage == nil && old.Storage != nil {
				batch[i].Storage = old.Storage
			}
		}
		for id, s := range z.pending {
			if !newer[id] {
				merged = append(merged, s)
			}
		}
		batch = append(merged, batch...)
		clear(z.pending)
	}
	if len(batch) == 0 {
		return
	}
	if blocking {
		z.saves <- batch
		return
	}
	select {
	case z.saves <- batch:
	default:
		z.log.Warn("save queue full, retrying later", "size", len(batch))
		for _, s := range batch {
			z.pending[s.ID] = s
		}
	}
}

func (z *Zone) entityPB(e *entity) *pb.Entity {
	out := &pb.Entity{
		Id: e.id, Name: e.name, Model: e.model, Kind: e.kind,
		X: float32(e.pos.X), Z: float32(e.pos.Z),
		TargetX: float32(e.dest.X), TargetZ: float32(e.dest.Z),
		Speed: float32(e.speed(z.now)),
		Level: uint32(e.level), Hp: uint32(e.hpInt()), MaxHp: uint32(e.maxHPInt()),
		Dead: e.dead, Scale: float32(e.scale),
		Boss: e.mon != nil && e.mon.def.Boss,
		Raid: e.mon != nil && e.mon.def.Raid,
	}
	if e.mon != nil {
		out.Tint = e.mon.def.Tint
	}
	if e.pl != nil {
		w, o, g := e.pl.inv.look()
		out.Weapon, out.Offhand, out.WeaponGrade = w, o, uint32(g)
		out.Emblem = e.pl.emblem
		out.Look = lookPB(e.pl.look)
		out.WeaponEnchant = uint32(e.pl.inv.weaponEnchant())
		out.PvpState = z.pvpState(e)
		out.ClanName = z.clanName(e)
		out.Title = e.pl.ach.titleText()
		out.Spec = e.pl.spec
	}
	if e.npc != nil {
		out.Title = e.npc.title
		if e.npc.look != nil {
			out.Look = lookPB(*e.npc.look)
		}
	}
	if e.obj != nil {
		out.Title = "Estandarte"
	}
	return out
}

// afterFlush agenda fn para logo depois do próximo snapshot.
func (z *Zone) afterFlush(fn func()) {
	z.afterSnapshot = append(z.afterSnapshot, fn)
}

// systemAfterFlush: aviso do sistema que sai depois do snapshot (se o jogador continuar aqui).
func (z *Zone) systemAfterFlush(p *entity, text string) {
	z.afterFlush(func() {
		if z.players[p.id] == p {
			z.system(p, text)
		}
	})
}

func (z *Zone) runAfterSnapshot() {
	fns := z.afterSnapshot
	z.afterSnapshot = nil
	for _, fn := range fns {
		fn()
	}
}

// broadcast serializa a mensagem UMA vez e envia os mesmos bytes a todos (exceto skip).
func (z *Zone) broadcast(msg *pb.ServerMessage, skip uint32) {
	data, err := proto.Marshal(msg)
	if err != nil {
		z.log.Error("marshal", "err", err)
		return
	}
	var slow []uint32
	for id, p := range z.players {
		if id == skip {
			continue
		}
		if !p.pl.peer.Send(data) {
			slow = append(slow, id)
		}
	}
	for _, id := range slow {
		z.dropSlow(id)
	}
}

func (z *Zone) sendTo(p *entity, msg *pb.ServerMessage) {
	if p.pl == nil {
		return
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		z.log.Error("marshal", "err", err)
		return
	}
	if !p.pl.peer.Send(data) {
		z.dropSlow(p.id)
	}
}

// dropSlow derruba um cliente cuja fila de envio encheu, para ele não travar a zona.
func (z *Zone) dropSlow(id uint32) {
	p, ok := z.players[id]
	if !ok {
		return
	}
	p.pl.peer.Close("send queue full")
	z.removePlayer(id)
}
