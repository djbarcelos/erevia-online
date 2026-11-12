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
	"math"
	"math/rand/v2"
	"regexp"
	"strings"
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
	Clock          func() time.Time // relógio do ciclo dia/noite (nil = time.Now)
}

func DefaultConfig() Config {
	return Config{
		Size: 300, TickRate: 20, MaxMovesPerSec: 10,
		ChatPerSec: 1, ChatBurst: 3, SaveEvery: 30 * time.Second,
		SafeRadius: 40, Spawns: game.DefaultSpawns(), Seed: 20261112,
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
	Peer   Peer
}

// SavedState é enviado ao canal de persistência.
type SavedState struct {
	ID     uint32
	X, Z   float64
	Level  int
	XP     int64
	HP, MP int // -1 = cheio
}

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
}
type attackEvent struct{ id, target uint32 }
type skillEvent struct{ id, skill, target uint32 }
type respawnEvent struct{ id uint32 }

func (joinEvent) isEvent()    {}
func (leaveEvent) isEvent()   {}
func (moveEvent) isEvent()    {}
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
	saves    chan<- []SavedState
	night    bool // ciclo dia/noite (ver game.DayPhase)
}

// New cria a zona e seus monstros. saves pode ser nil (sem persistência, ex.: testes).
func New(cfg Config, log *slog.Logger, saves chan<- []SavedState) *Zone {
	z := &Zone{
		cfg:      cfg,
		log:      log,
		events:   make(chan event, 4096),
		players:  make(map[uint32]*entity),
		monsters: make(map[uint32]*entity),
		byName:   make(map[string]uint32),
		rng:      rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^0x9e3779b97f4a7c15)),
		saves:    saves,
	}
	if z.cfg.Clock == nil {
		z.cfg.Clock = time.Now
	}
	z.night = game.IsNight(game.DayPhase(z.cfg.Clock()))
	z.spawnMonsters()
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
func (z *Zone) Chat(ctx context.Context, id uint32, text, to string) {
	z.post(ctx, chatEvent{id: id, text: text, to: to})
}
func (z *Zone) Attack(ctx context.Context, id, target uint32) {
	z.post(ctx, attackEvent{id: id, target: target})
}
func (z *Zone) UseSkill(ctx context.Context, id, skill, target uint32) {
	z.post(ctx, skillEvent{id: id, skill: skill, target: target})
}
func (z *Zone) Respawn(ctx context.Context, id uint32) { z.post(ctx, respawnEvent{id: id}) }

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

	for {
		select {
		case <-ctx.Done():
			z.saveAll(true)
			if z.saves != nil {
				close(z.saves)
			}
			return
		case e := <-z.events:
			z.handle(e)
		case <-ticker.C:
			z.update(dt.Seconds())
		case <-saveTicker.C:
			z.saveAll(false)
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
	case chatEvent:
		z.onChat(e)
	case attackEvent:
		if p := z.players[e.id]; p != nil && !p.dead && z.spendAction(p) {
			if t := z.monsters[e.target]; t != nil && !t.dead && !t.mon.hidden {
				if p.act.kind != actAttack || p.act.target != t.id {
					p.act = action{kind: actAttack, target: t.id}
				}
			}
		}
	case skillEvent:
		if p := z.players[e.id]; p != nil && z.spendAction(p) {
			z.requestSkill(p, e.skill, e.target)
		}
	case respawnEvent:
		if p := z.players[e.id]; p != nil {
			z.respawnPlayer(p)
		}
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
	if id >= monsterIDBase {
		return z.monsters[id]
	}
	return z.players[id]
}

func (z *Zone) onJoin(info JoinInfo) {
	// Mesmo personagem conectado em outro lugar: derruba a sessão antiga.
	if old, ok := z.players[info.ID]; ok {
		z.sendTo(old, &pb.ServerMessage{Msg: &pb.ServerMessage_Kicked{Kicked: &pb.Kicked{
			Reason: "Este personagem entrou em outro lugar.",
		}}})
		old.pl.peer.Close("relogin")
		z.removePlayer(info.ID)
	}

	level := game.LevelForXP(info.XP)
	stats := game.ClassStats(info.Class, level)
	pos := z.freeSpot(clampToZone(Vec2{X: info.X, Z: info.Z}, z.cfg.Size/2))
	p := &entity{
		id: info.ID, kind: pb.EntityKind_ENTITY_KIND_PLAYER,
		name: info.Name, model: info.Class, level: level, scale: 1,
		pos: pos, dest: pos, stats: stats,
		hp: stats.MaxHP, mp: stats.MaxMP,
		lastCombat: -math.MaxFloat64 / 2,
		pl: &playerState{
			peer: info.Peer, class: info.Class, xp: info.XP,
			cooldowns:  map[uint32]float64{},
			moveTokens: z.cfg.MaxMovesPerSec, chatTokens: z.cfg.ChatBurst,
		},
	}
	if info.HP > 0 && float64(info.HP) < stats.MaxHP {
		p.hp = float64(info.HP)
	}
	if info.MP > 0 && float64(info.MP) < stats.MaxMP {
		p.mp = float64(info.MP)
	}
	p.sentHP, p.sentMaxHP = p.hpInt(), p.maxHPInt()
	z.players[p.id] = p
	z.byName[strings.ToLower(p.name)] = p.id

	// Avisa os outros que alguém chegou.
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Spawn{Spawn: &pb.Spawn{Entity: z.entityPB(p)}}}, p.id)

	// Diz ao novo jogador quem ele é, quem já está aqui (inclui ele mesmo) e suas skills.
	entities := make([]*pb.Entity, 0, len(z.players)+len(z.monsters))
	for _, other := range z.players {
		entities = append(entities, z.entityPB(other))
	}
	for _, m := range z.monsters {
		if !m.mon.hidden {
			entities = append(entities, z.entityPB(m))
		}
	}
	var skills []*pb.SkillInfo
	for _, s := range game.ClassSkills(info.Class) {
		skills = append(skills, &pb.SkillInfo{
			Id: s.ID, Name: s.Name, Key: uint32(s.Key), MpCost: uint32(s.MPCost),
			CooldownMs: uint32(s.Cooldown * 1000), Range: float32(s.Range), Target: pb.SkillTarget(s.Target),
		})
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
	}}})
	z.sendSelfStats(p)

	z.log.Info("player joined", "id", p.id, "name", p.name, "level", p.level, "online", len(z.players))
}

func (z *Zone) onMove(e moveEvent) {
	p, ok := z.players[e.id]
	if !ok || p.dead || !z.spendAction(p) {
		return
	}
	p.clearAction() // clicar no chão cancela o ataque
	if p.stunned(z.now) {
		return
	}
	z.goTo(p, clampToZone(Vec2{X: e.x, Z: e.z}, z.cfg.Size/2))
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

	to := strings.TrimSpace(e.to)
	if to == "" {
		z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Chat{Chat: &pb.ChatMessage{
			Channel: pb.ChatChannel_CHAT_CHANNEL_GENERAL, FromId: p.id, FromName: p.name, Text: text,
		}}}, 0)
		return
	}

	targetID, ok := z.byName[strings.ToLower(to)]
	if !ok {
		z.system(p, to+" não está online.")
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
	delete(z.players, id)
	if z.byName[strings.ToLower(p.name)] == id {
		delete(z.byName, strings.ToLower(p.name))
	}
	for _, m := range z.monsters {
		delete(m.mon.hate, id)
		if m.act.target == id {
			m.clearAction()
		}
	}
	z.save([]SavedState{z.savedState(p)}, false)
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Despawn{Despawn: &pb.Despawn{Id: id}}}, 0)
	z.log.Info("player left", "id", id, "name", p.name, "online", len(z.players))
}

// update avança a simulação um tick e envia o delta para todos.
func (z *Zone) update(dt float64) {
	z.tick++
	z.now += dt
	z.updateDaytime()

	for _, p := range z.players {
		p.pl.moveTokens = math.Min(z.cfg.MaxMovesPerSec, p.pl.moveTokens+z.cfg.MaxMovesPerSec*dt)
		p.pl.chatTokens = math.Min(z.cfg.ChatBurst, p.pl.chatTokens+z.cfg.ChatPerSec*dt)
		z.step(p, dt)
	}
	for _, m := range z.monsters {
		if m.dead {
			z.updateDeadMonster(m)
			continue
		}
		z.think(m)
		z.step(m, dt)
	}

	z.flushSnapshot()
	for _, p := range z.players {
		if p.pl.statsDirty || p.hpInt() != p.sentHP || int(math.Floor(p.mp)) != p.pl.sentMP {
			z.sendSelfStats(p)
		}
	}
}

// step: efeitos, regeneração, ação e movimento de uma entidade viva.
func (z *Zone) step(e *entity, dt float64) {
	for _, k := range e.pruneEffects(z.now) {
		if k == game.EffectSlow || k == game.EffectBuffSpeed {
			e.moved = true
		}
		if e.pl != nil {
			e.pl.statsDirty = true
		}
	}
	if e.dead {
		return
	}
	z.regen(e, dt)
	z.runAction(e)
	if e.stunned(z.now) {
		return
	}
	z.advance(e, e.speed(z.now)*dt)
}

func (z *Zone) flushSnapshot() {
	snap := &pb.Snapshot{Tick: z.tick}
	collect := func(e *entity) {
		if e.moved && !e.dead {
			snap.States = append(snap.States, &pb.EntityState{
				Id: e.id,
				X:  float32(e.pos.X), Z: float32(e.pos.Z),
				TargetX: float32(e.dest.X), TargetZ: float32(e.dest.Z),
				Speed: float32(e.speed(z.now)),
			})
		}
		e.moved = false
		if hp, max := e.hpInt(), e.maxHPInt(); hp != e.sentHP || max != e.sentMaxHP {
			snap.Hp = append(snap.Hp, &pb.HpUpdate{Id: e.id, Hp: uint32(hp), MaxHp: uint32(max)})
			e.sentHP, e.sentMaxHP = hp, max
		}
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
	z.frame = frame{}

	if len(snap.States)+len(snap.Hp)+len(snap.Combat)+len(snap.Effects)+len(snap.Deaths)+len(snap.Revives)+len(snap.LevelUps) == 0 {
		return
	}
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Snapshot{Snapshot: snap}}, 0)
}

func (z *Zone) sendSelfStats(p *entity) {
	p.pl.statsDirty = false
	p.pl.sentMP = int(math.Floor(p.mp))
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_SelfStats{SelfStats: z.selfStats(p)}})
}

func (z *Zone) savedState(p *entity) SavedState {
	s := SavedState{ID: p.id, X: p.pos.X, Z: p.pos.Z, Level: p.level, XP: p.pl.xp, HP: p.hpInt(), MP: int(math.Floor(p.mp))}
	if p.dead { // quem sai morto volta na cidade, inteiro (a XP já foi descontada)
		s.X, s.Z, s.HP, s.MP = 0, 0, -1, -1
	}
	if s.HP >= p.maxHPInt() {
		s.HP = -1
	}
	if float64(s.MP) >= math.Floor(p.stats.MaxMP) {
		s.MP = -1
	}
	return s
}

func (z *Zone) saveAll(blocking bool) {
	if len(z.players) == 0 {
		return
	}
	batch := make([]SavedState, 0, len(z.players))
	for _, p := range z.players {
		batch = append(batch, z.savedState(p))
	}
	z.save(batch, blocking)
}

// save entrega estados à goroutine de persistência. Fora do desligamento nunca
// bloqueia: se o canal estiver cheio, o próximo salvamento periódico cobre.
func (z *Zone) save(batch []SavedState, blocking bool) {
	if z.saves == nil {
		return
	}
	if blocking {
		z.saves <- batch
		return
	}
	select {
	case z.saves <- batch:
	default:
		z.log.Warn("save queue full, dropping batch", "size", len(batch))
	}
}

func (z *Zone) entityPB(e *entity) *pb.Entity {
	return &pb.Entity{
		Id: e.id, Name: e.name, Model: e.model, Kind: e.kind,
		X: float32(e.pos.X), Z: float32(e.pos.Z),
		TargetX: float32(e.dest.X), TargetZ: float32(e.dest.Z),
		Speed: float32(e.speed(z.now)),
		Level: uint32(e.level), Hp: uint32(e.hpInt()), MaxHp: uint32(e.maxHPInt()),
		Dead: e.dead, Scale: float32(e.scale),
		Boss: e.mon != nil && e.mon.def.Boss,
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
