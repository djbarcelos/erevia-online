// Package net cuida das conexões WebSocket dos jogadores.
//
// Cada conexão tem duas goroutines: readPump (decodifica e repassa à zona) e
// writePump (envia a partir de uma fila com limite).
package net

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/zone"
)

const (
	maxMessageBytes = 1024
	sendQueueSize   = 256
	joinTimeout     = 10 * time.Second
	idleTimeout     = 30 * time.Second // cliente manda Ping a cada ~2s
	writeTimeout    = 5 * time.Second
)

// Character é o que o backend devolve ao carregar um personagem para entrar no jogo.
type Character struct {
	ID     uint32
	Name   string
	Class  string
	X, Z   float64
	XP     int64
	HP, MP int // <= 0 = cheio
	Gold   int64
	Items  []game.ItemRecord
	Attrs  game.Attrs
	Ranks  map[uint32]int
	Hotbar []string
	Emblem string
	// Fase 4
	AccountID  int64
	Karma      int
	PKs        int
	PvPKills   int
	ClanLeftAt time.Time
}

// ErrCharacterNotFound indica personagem inexistente ou de outra conta.
var ErrCharacterNotFound = errors.New("character not found")

// Backend autentica a conexão e carrega personagens (implementado com o store).
type Backend interface {
	Authenticate(r *http.Request) (accountID int64, err error)
	LoadCharacter(ctx context.Context, accountID int64, characterID uint32) (Character, error)
}

type Server struct {
	zone           *zone.Zone
	backend        Backend
	log            *slog.Logger
	originPatterns []string
}

// NewServer cria o handler de WebSocket. originPatterns segue o formato do
// coder/websocket (ex.: "localhost:*"); requisições da mesma origem sempre passam.
func NewServer(z *zone.Zone, backend Backend, log *slog.Logger, originPatterns []string) *Server {
	return &Server{zone: z, backend: backend, log: log, originPatterns: originPatterns}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	accountID, err := s.backend.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: s.originPatterns})
	if err != nil {
		s.log.Warn("ws accept", "err", err, "remote", r.RemoteAddr)
		return
	}
	c.SetReadLimit(maxMessageBytes)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	characterID, err := readJoin(ctx, c)
	if err != nil {
		s.log.Info("join rejected", "err", err, "account", accountID)
		c.Close(websocket.StatusPolicyViolation, err.Error())
		return
	}
	char, err := s.backend.LoadCharacter(ctx, accountID, characterID)
	if err != nil {
		reason := "erro ao carregar personagem"
		if errors.Is(err, ErrCharacterNotFound) {
			reason = "personagem inválido"
		} else {
			s.log.Error("load character", "err", err)
		}
		c.Close(websocket.StatusPolicyViolation, reason)
		return
	}

	sess := &session{
		conn: c, send: make(chan []byte, sendQueueSize),
		closing: make(chan string, 1), cancel: cancel,
	}
	go sess.writePump(ctx)
	s.zone.Join(ctx, zone.JoinInfo{
		ID: char.ID, Name: char.Name, Class: char.Class, X: char.X, Z: char.Z,
		XP: char.XP, HP: char.HP, MP: char.MP, Gold: char.Gold, Items: char.Items,
		Attrs: char.Attrs, Ranks: char.Ranks, Hotbar: char.Hotbar, Emblem: char.Emblem,
		AccountID: char.AccountID, Karma: char.Karma, PKs: char.PKs, PvPKills: char.PvPKills,
		ClanLeftAt: char.ClanLeftAt, Peer: sess,
	})

	s.readPump(ctx, sess, char.ID)

	// Usa um contexto novo: ctx pode já estar cancelado, e a zona precisa saber da saída.
	leaveCtx, leaveCancel := context.WithTimeout(context.Background(), time.Second)
	s.zone.Leave(leaveCtx, char.ID, sess)
	leaveCancel()

	cancel()
	c.Close(websocket.StatusNormalClosure, "")
}

func readJoin(ctx context.Context, c *websocket.Conn) (uint32, error) {
	rctx, cancel := context.WithTimeout(ctx, joinTimeout)
	defer cancel()

	typ, data, err := c.Read(rctx)
	if err != nil {
		return 0, err
	}
	if typ != websocket.MessageBinary {
		return 0, errors.New("expected binary message")
	}
	var msg pb.ClientMessage
	if err := proto.Unmarshal(data, &msg); err != nil {
		return 0, errors.New("invalid message")
	}
	join := msg.GetJoin()
	if join == nil || join.CharacterId == 0 {
		return 0, errors.New("first message must be Join")
	}
	return join.CharacterId, nil
}

func (s *Server) readPump(ctx context.Context, sess *session, id uint32) {
	var msg pb.ClientMessage
	for {
		rctx, cancel := context.WithTimeout(ctx, idleTimeout)
		typ, data, err := sess.conn.Read(rctx)
		cancel()
		if err != nil {
			return
		}
		if typ != websocket.MessageBinary {
			continue
		}
		msg.Reset()
		if err := proto.Unmarshal(data, &msg); err != nil {
			continue
		}

		switch m := msg.Msg.(type) {
		case *pb.ClientMessage_MoveTo:
			s.zone.Move(ctx, id, float64(m.MoveTo.X), float64(m.MoveTo.Z))
		case *pb.ClientMessage_ChatSend:
			s.zone.Chat(ctx, id, m.ChatSend.Text, m.ChatSend.To, m.ChatSend.Channel)
		case *pb.ClientMessage_Attack:
			s.zone.Attack(ctx, id, m.Attack.TargetId, m.Attack.Force)
		case *pb.ClientMessage_UseSkill:
			s.zone.UseSkill(ctx, id, m.UseSkill.SkillId, m.UseSkill.TargetId, m.UseSkill.Force)
		case *pb.ClientMessage_Respawn:
			s.zone.Respawn(ctx, id)
		case *pb.ClientMessage_PickUp:
			s.zone.ItemAction(ctx, id, m.PickUp)
		case *pb.ClientMessage_MoveItem:
			s.zone.ItemAction(ctx, id, m.MoveItem)
		case *pb.ClientMessage_UseItem:
			s.zone.ItemAction(ctx, id, m.UseItem)
		case *pb.ClientMessage_DestroyItem:
			s.zone.ItemAction(ctx, id, m.DestroyItem)
		case *pb.ClientMessage_Buy:
			s.zone.ItemAction(ctx, id, m.Buy)
		case *pb.ClientMessage_Sell:
			s.zone.ItemAction(ctx, id, m.Sell)
		case *pb.ClientMessage_AllocateAttrs:
			s.zone.ProgressAction(ctx, id, m.AllocateAttrs)
		case *pb.ClientMessage_LearnSkills:
			s.zone.ProgressAction(ctx, id, m.LearnSkills)
		case *pb.ClientMessage_SetHotbar:
			s.zone.ProgressAction(ctx, id, m.SetHotbar)
		case *pb.ClientMessage_PartyCmd:
			s.zone.Social(ctx, id, m.PartyCmd)
		case *pb.ClientMessage_ClanCmd:
			s.zone.Social(ctx, id, m.ClanCmd)
		case *pb.ClientMessage_InviteReply:
			s.zone.Social(ctx, id, m.InviteReply)
		case *pb.ClientMessage_Ping:
			// Respondido aqui mesmo, sem passar pela zona, para medir só a rede.
			pong, _ := proto.Marshal(&pb.ServerMessage{Msg: &pb.ServerMessage_Pong{
				Pong: &pb.Pong{ClientTime: m.Ping.ClientTime},
			}})
			sess.Send(pong)
		}
	}
}

// session implementa zone.Peer.
type session struct {
	conn    *websocket.Conn
	send    chan []byte
	closing chan string
	cancel  context.CancelFunc
}

func (s *session) Send(msg []byte) bool {
	select {
	case s.send <- msg:
		return true
	default:
		return false
	}
}

// Close pede ao writePump que envie o que está na fila (ex.: Kicked) e feche o
// socket com o motivo. Fechar o socket faz o readPump terminar.
func (s *session) Close(reason string) {
	select {
	case s.closing <- reason:
	default: // já está fechando
	}
}

func (s *session) writePump(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case reason := <-s.closing:
			s.flush(500 * time.Millisecond)
			s.conn.Close(websocket.StatusPolicyViolation, reason)
			s.cancel()
			return
		case msg := <-s.send:
			wctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := s.conn.Write(wctx, websocket.MessageBinary, msg)
			cancel()
			if err != nil {
				s.cancel()
				return
			}
		}
	}
}

// flush envia o que sobrou na fila, com prazo total curto.
func (s *session) flush(limit time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	for {
		select {
		case msg := <-s.send:
			if s.conn.Write(ctx, websocket.MessageBinary, msg) != nil {
				return
			}
		default:
			return
		}
	}
}
