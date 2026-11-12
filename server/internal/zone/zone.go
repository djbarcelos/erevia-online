// Package zone contém a simulação autoritativa de uma zona do mapa.
//
// Uma única goroutine (Run) é dona de todo o estado da zona. Conexões falam com ela
// apenas por canal (Join, Leave, Move), então não há mutex nem condição de corrida.
package zone

import (
	"context"
	"log/slog"
	"math"
	"math/rand/v2"
	"regexp"
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
	Size           float64 // lado do quadrado jogável, em metros
	TickRate       int     // ticks por segundo
	Speed          float64 // metros por segundo
	MaxMovesPerSec float64 // limite de pedidos de movimento por jogador
	SpawnRadius    float64
}

func DefaultConfig() Config {
	return Config{Size: 200, TickRate: 20, Speed: 5, MaxMovesPerSec: 10, SpawnRadius: 8}
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9]{3,16}$`)

// ValidName informa se o nome do personagem é aceito (3–16 letras/números).
func ValidName(name string) bool { return nameRe.MatchString(name) }

type player struct {
	id      uint32
	name    string
	peer    Peer
	pos     Vec2
	target  Vec2
	changed bool // destino mudou neste tick: precisa ir no snapshot mesmo parado

	moveTokens float64 // token bucket para limitar MoveTo
}

type event interface{ isEvent() }

type joinEvent struct {
	id   uint32
	name string
	peer Peer
}
type leaveEvent struct{ id uint32 }
type moveEvent struct {
	id   uint32
	x, z float64
}

func (joinEvent) isEvent()  {}
func (leaveEvent) isEvent() {}
func (moveEvent) isEvent()  {}

type Zone struct {
	cfg     Config
	log     *slog.Logger
	events  chan event
	players map[uint32]*player
	tick    uint32
}

func New(cfg Config, log *slog.Logger) *Zone {
	return &Zone{
		cfg:     cfg,
		log:     log,
		events:  make(chan event, 4096),
		players: make(map[uint32]*player),
	}
}

// Join coloca um jogador na zona. O id é gerado por quem chama (único no processo).
func (z *Zone) Join(ctx context.Context, id uint32, name string, peer Peer) {
	z.post(ctx, joinEvent{id: id, name: name, peer: peer})
}

// Leave remove o jogador. Chamar para um id que já saiu não tem efeito.
func (z *Zone) Leave(ctx context.Context, id uint32) {
	z.post(ctx, leaveEvent{id: id})
}

// Move registra o pedido do jogador de andar até (x, z). A zona valida.
func (z *Zone) Move(ctx context.Context, id uint32, x, z2 float64) {
	z.post(ctx, moveEvent{id: id, x: x, z: z2})
}

func (z *Zone) post(ctx context.Context, e event) {
	select {
	case z.events <- e:
	case <-ctx.Done():
	}
}

// Run executa o loop da zona até ctx ser cancelado.
func (z *Zone) Run(ctx context.Context) {
	dt := time.Second / time.Duration(z.cfg.TickRate)
	ticker := time.NewTicker(dt)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case e := <-z.events:
			z.handle(e)
		case <-ticker.C:
			z.update(dt.Seconds())
		}
	}
}

func (z *Zone) handle(e event) {
	switch e := e.(type) {
	case joinEvent:
		z.onJoin(e)
	case leaveEvent:
		z.removePlayer(e.id)
	case moveEvent:
		z.onMove(e)
	}
}

func (z *Zone) onJoin(e joinEvent) {
	angle := rand.Float64() * 2 * math.Pi
	r := rand.Float64() * z.cfg.SpawnRadius
	pos := Vec2{X: r * math.Cos(angle), Z: r * math.Sin(angle)}

	p := &player{
		id: e.id, name: e.name, peer: e.peer,
		pos: pos, target: pos,
		moveTokens: z.cfg.MaxMovesPerSec,
	}
	z.players[p.id] = p

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

func (z *Zone) removePlayer(id uint32) {
	p, ok := z.players[id]
	if !ok {
		return
	}
	delete(z.players, id)
	z.broadcast(&pb.ServerMessage{Msg: &pb.ServerMessage_Despawn{Despawn: &pb.Despawn{Id: id}}}, 0)
	z.log.Info("player left", "id", id, "name", p.name, "online", len(z.players))
}

// update avança a simulação um tick e envia o delta para todos.
func (z *Zone) update(dt float64) {
	z.tick++
	var states []*pb.EntityState

	for _, p := range z.players {
		p.moveTokens = min(z.cfg.MaxMovesPerSec, p.moveTokens+z.cfg.MaxMovesPerSec*dt)

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

func (z *Zone) entity(p *player) *pb.Entity {
	return &pb.Entity{
		Id: p.id, Name: p.name,
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
