// Package zone contém a simulação autoritativa de uma zona do mapa.
//
// Uma única goroutine (Run) é dona de todo o estado da zona. Conexões falam com ela
// apenas por canal (Join, Leave, Move, Chat), então não há mutex nem condição de corrida.
package zone

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"erevia/server/internal/pb"
)

// Peer é o lado "conexão" de um jogador, visto pela zona.
type Peer interface {
	// Send enfileira bytes já serializados sem bloquear. Retorna false se a fila estiver cheia.
	Send(msg []byte) bool
	// Close derruba a conexão (cliente lento, comportamento inválido…).
	Close(reason string)
}

type Config struct {
	Size           float64       // lado do quadrado jogável, em metros
	TickRate       int           // ticks por segundo
	Speed          float64       // metros por segundo
	MaxMovesPerSec float64       // limite de pedidos de movimento por jogador
	ChatPerSec     float64       // mensagens de chat por segundo (reposição)
	ChatBurst      float64       // rajada máxima de mensagens
	SaveEvery      time.Duration // intervalo do salvamento periódico de posições
}

func DefaultConfig() Config {
	return Config{
		Size: 200, TickRate: 20, Speed: 5, MaxMovesPerSec: 10,
		ChatPerSec: 1, ChatBurst: 3, SaveEvery: 30 * time.Second,
	}
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9]{3,16}$`)

// ValidName informa se o nome do personagem é aceito (3–16 letras/números).
func ValidName(name string) bool { return nameRe.MatchString(name) }

// JoinInfo é o personagem já carregado e validado por quem chama.
type JoinInfo struct {
	ID    uint32
	Name  string
	Class string
	X, Z  float64
	Peer  Peer
}

// SavedPosition é enviada ao canal de persistência.
type SavedPosition struct {
	ID   uint32
	X, Z float64
}

type player struct {
	id      uint32
	name    string
	class   string
	peer    Peer
	pos     Vec2
	target  Vec2
	changed bool // destino mudou neste tick: precisa ir no snapshot mesmo parado

	moveTokens float64 // token bucket para limitar MoveTo
	chatTokens float64 // token bucket para limitar chat
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

func (joinEvent) isEvent()  {}
func (leaveEvent) isEvent() {}
func (moveEvent) isEvent()  {}
func (chatEvent) isEvent()  {}

type Zone struct {
	cfg     Config
	log     *slog.Logger
	events  chan event
	players map[uint32]*player
	byName  map[string]uint32 // nome em minúsculas → id
	tick    uint32
	saves   chan<- []SavedPosition
}

// New cria a zona. saves pode ser nil (sem persistência, ex.: testes).
func New(cfg Config, log *slog.Logger, saves chan<- []SavedPosition) *Zone {
	return &Zone{
		cfg:     cfg,
		log:     log,
		events:  make(chan event, 4096),
		players: make(map[uint32]*player),
		byName:  make(map[string]uint32),
		saves:   saves,
	}
}

// Join coloca o personagem na zona. Se ele já estiver online, a conexão antiga cai.
func (z *Zone) Join(ctx context.Context, info JoinInfo) {
	z.post(ctx, joinEvent{info: info})
}

// Leave remove o jogador se peer ainda for a conexão dele. Repetir não tem efeito.
func (z *Zone) Leave(ctx context.Context, id uint32, peer Peer) {
	z.post(ctx, leaveEvent{id: id, peer: peer})
}

// Move registra o pedido do jogador de andar até (x, z). A zona valida.
func (z *Zone) Move(ctx context.Context, id uint32, x, z2 float64) {
	z.post(ctx, moveEvent{id: id, x: x, z: z2})
}

// Chat envia uma mensagem: geral se to == "", sussurro caso contrário.
func (z *Zone) Chat(ctx context.Context, id uint32, text, to string) {
	z.post(ctx, chatEvent{id: id, text: text, to: to})
}

func (z *Zone) post(ctx context.Context, e event) {
	select {
	case z.events <- e:
	case <-ctx.Done():
	}
}

// Run executa o loop da zona até ctx ser cancelado. Ao terminar, salva todas as
// posições e fecha o canal de persistência.
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
		if p, ok := z.players[e.id]; ok && p.peer == e.peer {
			z.removePlayer(e.id)
		}
	case moveEvent:
		z.onMove(e)
	case chatEvent:
		z.onChat(e)
	}
}

func (z *Zone) onJoin(info JoinInfo) {
	// Mesmo personagem conectado em outro lugar: derruba a sessão antiga.
	if old, ok := z.players[info.ID]; ok {
		z.sendTo(old, &pb.ServerMessage{Msg: &pb.ServerMessage_Kicked{Kicked: &pb.Kicked{
			Reason: "Este personagem entrou em outro lugar.",
		}}})
		old.peer.Close("relogin")
		z.removePlayer(info.ID)
	}

	pos := clampToZone(Vec2{X: info.X, Z: info.Z}, z.cfg.Size/2)
	p := &player{
		id: info.ID, name: info.Name, class: info.Class, peer: info.Peer,
		pos: pos, target: pos,
		moveTokens: z.cfg.MaxMovesPerSec,
		chatTokens: z.cfg.ChatBurst,
	}
	z.players[p.id] = p
	z.byName[strings.ToLower(p.name)] = p.id

	// Avisa os outros que alguém chegou.
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Spawn{Spawn: &pb.Spawn{Entity: z.entity(p)}}}, p.id)

	// Diz ao novo jogador quem ele é e quem já está aqui (inclui ele mesmo).
	entities := make([]*pb.Entity, 0, len(z.players))
	for _, other := range z.players {
		entities = append(entities, z.entity(other))
	}
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Welcome{Welcome: &pb.Welcome{
		YourId:   p.id,
		TickRate: uint32(z.cfg.TickRate),
		ZoneSize: float32(z.cfg.Size),
		Entities: entities,
	}}})

	z.log.Info("player joined", "id", p.id, "name", p.name, "online", len(z.players))
}

func (z *Zone) onMove(e moveEvent) {
	p, ok := z.players[e.id]
	if !ok {
		return
	}
	if p.moveTokens < 1 {
		return // excedeu o limite de pedidos: descarta
	}
	p.moveTokens--
	p.target = clampToZone(Vec2{X: e.x, Z: e.z}, z.cfg.Size/2)
	p.changed = true
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
	if p.chatTokens < 1 {
		z.system(p, "Calma! Você está enviando mensagens rápido demais.")
		return
	}
	p.chatTokens--

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

func (z *Zone) system(p *player, text string) {
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
	z.save([]SavedPosition{{ID: p.id, X: p.pos.X, Z: p.pos.Z}}, false)
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Despawn{Despawn: &pb.Despawn{Id: id}}}, 0)
	z.log.Info("player left", "id", id, "name", p.name, "online", len(z.players))
}

// update avança a simulação um tick e envia o delta para todos.
func (z *Zone) update(dt float64) {
	z.tick++
	var states []*pb.EntityState

	for _, p := range z.players {
		p.moveTokens = min(z.cfg.MaxMovesPerSec, p.moveTokens+z.cfg.MaxMovesPerSec*dt)
		p.chatTokens = min(z.cfg.ChatBurst, p.chatTokens+z.cfg.ChatPerSec*dt)

		newPos, moved := step(p.pos, p.target, z.cfg.Speed*dt)
		if moved || p.changed {
			p.pos = newPos
			p.changed = false
			states = append(states, &pb.EntityState{
				Id: p.id,
				X:  float32(p.pos.X), Z: float32(p.pos.Z),
				TargetX: float32(p.target.X), TargetZ: float32(p.target.Z),
			})
		}
	}

	if len(states) == 0 {
		return
	}
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Snapshot{Snapshot: &pb.Snapshot{
		Tick: z.tick, States: states,
	}}}, 0)
}

func (z *Zone) saveAll(blocking bool) {
	if len(z.players) == 0 {
		return
	}
	batch := make([]SavedPosition, 0, len(z.players))
	for _, p := range z.players {
		batch = append(batch, SavedPosition{ID: p.id, X: p.pos.X, Z: p.pos.Z})
	}
	z.save(batch, blocking)
}

// save entrega posições à goroutine de persistência. Fora do desligamento nunca
// bloqueia: se o canal estiver cheio, o próximo salvamento periódico cobre.
func (z *Zone) save(batch []SavedPosition, blocking bool) {
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

func (z *Zone) entity(p *player) *pb.Entity {
	return &pb.Entity{
		Id: p.id, Name: p.name, Class: p.class,
		X: float32(p.pos.X), Z: float32(p.pos.Z),
		TargetX: float32(p.target.X), TargetZ: float32(p.target.Z),
		Speed: float32(z.cfg.Speed),
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
		if !p.peer.Send(data) {
			slow = append(slow, id)
		}
	}
	for _, id := range slow {
		z.dropSlow(id)
	}
}

func (z *Zone) sendTo(p *player, msg *pb.ServerMessage) {
	data, err := proto.Marshal(msg)
	if err != nil {
		z.log.Error("marshal", "err", err)
		return
	}
	if !p.peer.Send(data) {
		z.dropSlow(p.id)
	}
}

// dropSlow derruba um cliente cuja fila de envio encheu, para ele não travar a zona.
func (z *Zone) dropSlow(id uint32) {
	p, ok := z.players[id]
	if !ok {
		return
	}
	p.peer.Close("send queue full")
	z.removePlayer(id)
}
