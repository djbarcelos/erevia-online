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
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

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

type Server struct {
	zone           *zone.Zone
	log            *slog.Logger
	originPatterns []string
	nextID         atomic.Uint32
}

// NewServer cria o handler de WebSocket. originPatterns segue o formato do
// coder/websocket (ex.: "localhost:*").
func NewServer(z *zone.Zone, log *slog.Logger, originPatterns []string) *Server {
	return &Server{zone: z, log: log, originPatterns: originPatterns}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: s.originPatterns})
	if err != nil {
		s.log.Warn("ws accept", "err", err, "remote", r.RemoteAddr)
		return
	}
	c.SetReadLimit(maxMessageBytes)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	sess := &session{conn: c, send: make(chan []byte, sendQueueSize), cancel: cancel}

	name, err := readJoin(ctx, c)
	if err != nil {
		s.log.Info("join rejected", "err", err, "remote", r.RemoteAddr)
		c.Close(websocket.StatusPolicyViolation, err.Error())
		return
	}

	id := s.nextID.Add(1)
	go sess.writePump(ctx)
	s.zone.Join(ctx, id, name, sess)

	s.readPump(ctx, sess, id)

	// Usa um contexto novo: ctx pode já estar cancelado, e a zona precisa saber da saída.
	leaveCtx, leaveCancel := context.WithTimeout(context.Background(), time.Second)
	s.zone.Leave(leaveCtx, id)
	leaveCancel()

	c.Close(websocket.StatusNormalClosure, "")
}

func readJoin(ctx context.Context, c *websocket.Conn) (string, error) {
	rctx, cancel := context.WithTimeout(ctx, joinTimeout)
	defer cancel()

	typ, data, err := c.Read(rctx)
	if err != nil {
		return "", err
	}
	if typ != websocket.MessageBinary {
		return "", errors.New("expected binary message")
	}
	var msg pb.ClientMessage
	if err := proto.Unmarshal(data, &msg); err != nil {
		return "", errors.New("invalid message")
	}
	join := msg.GetJoin()
	if join == nil {
		return "", errors.New("first message must be Join")
	}
	if !zone.ValidName(join.Name) {
		return "", errors.New("invalid name")
	}
	return join.Name, nil
}

func (s *Server) readPump(ctx context.Context, sess *session, id uint32) error {
	var msg pb.ClientMessage
	for {
		rctx, cancel := context.WithTimeout(ctx, idleTimeout)
		typ, data, err := sess.conn.Read(rctx)
		cancel()
		if err != nil {
			return err
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
	conn   *websocket.Conn
	send   chan []byte
	cancel context.CancelFunc
}

func (s *session) Send(msg []byte) bool {
	select {
	case s.send <- msg:
		return true
	default:
		return false
	}
}

func (s *session) Close(reason string) {
	s.cancel() // readPump e writePump terminam; o handler fecha o socket
}

func (s *session) writePump(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
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
