package net

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	"erevia/server/internal/pb"
	"erevia/server/internal/zone"
)

func startServer(t *testing.T) (url string, ctx context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	z := zone.New(zone.DefaultConfig(), log)
	go z.Run(ctx)

	srv := httptest.NewServer(NewServer(z, log, []string{"*"}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http"), ctx
}

func dial(t *testing.T, ctx context.Context, url, name string) *websocket.Conn {
	t.Helper()
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { c.CloseNow() })
	send(t, ctx, c, &pb.ClientMessage{Msg: &pb.ClientMessage_Join{Join: &pb.Join{Name: name}}})
	return c
}

func send(t *testing.T, ctx context.Context, c *websocket.Conn, m *pb.ClientMessage) {
	t.Helper()
	data, _ := proto.Marshal(m)
	if err := c.Write(ctx, websocket.MessageBinary, data); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// waitFor lê mensagens até match retornar true.
func waitFor(t *testing.T, ctx context.Context, c *websocket.Conn, what string, match func(*pb.ServerMessage) bool) *pb.ServerMessage {
	t.Helper()
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("waiting for %s: %v", what, err)
		}
		var m pb.ServerMessage
		if err := proto.Unmarshal(data, &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if match(&m) {
			return &m
		}
	}
}

func TestTwoPlayersSeeEachOtherMove(t *testing.T) {
	url, ctx := startServer(t)

	a := dial(t, ctx, url, "Alice")
	welcomeA := waitFor(t, ctx, a, "welcome A", func(m *pb.ServerMessage) bool { return m.GetWelcome() != nil }).GetWelcome()
	idA := welcomeA.YourId

	b := dial(t, ctx, url, "Bob")
	welcomeB := waitFor(t, ctx, b, "welcome B", func(m *pb.ServerMessage) bool { return m.GetWelcome() != nil }).GetWelcome()
	if len(welcomeB.Entities) != 2 {
		t.Fatalf("Bob should see 2 entities on join, got %d", len(welcomeB.Entities))
	}

	// Alice recebe o Spawn do Bob.
	waitFor(t, ctx, a, "spawn Bob", func(m *pb.ServerMessage) bool {
		return m.GetSpawn() != nil && m.GetSpawn().Entity.Name == "Bob"
	})

	// Alice anda; Bob deve ver Alice se mover em direção ao destino.
	send(t, ctx, a, &pb.ClientMessage{Msg: &pb.ClientMessage_MoveTo{MoveTo: &pb.MoveTo{X: 20, Z: 0}}})
	snap := waitFor(t, ctx, b, "snapshot with Alice", func(m *pb.ServerMessage) bool {
		for _, s := range m.GetSnapshot().GetStates() {
			if s.Id == idA && s.TargetX == 20 {
				return true
			}
		}
		return false
	})
	if snap.GetSnapshot().Tick == 0 {
		t.Fatal("snapshot tick should be > 0")
	}

	// Destino fora do mapa é limitado pelo servidor.
	send(t, ctx, a, &pb.ClientMessage{Msg: &pb.ClientMessage_MoveTo{MoveTo: &pb.MoveTo{X: 9999, Z: -9999}}})
	waitFor(t, ctx, b, "clamped target", func(m *pb.ServerMessage) bool {
		for _, s := range m.GetSnapshot().GetStates() {
			if s.Id == idA && s.TargetX == 100 && s.TargetZ == -100 {
				return true
			}
		}
		return false
	})

	// Alice sai; Bob recebe Despawn.
	a.Close(websocket.StatusNormalClosure, "")
	waitFor(t, ctx, b, "despawn Alice", func(m *pb.ServerMessage) bool {
		return m.GetDespawn() != nil && m.GetDespawn().Id == idA
	})
}

func TestPingPong(t *testing.T) {
	url, ctx := startServer(t)
	c := dial(t, ctx, url, "Pinger")
	send(t, ctx, c, &pb.ClientMessage{Msg: &pb.ClientMessage_Ping{Ping: &pb.Ping{ClientTime: 123.5}}})
	m := waitFor(t, ctx, c, "pong", func(m *pb.ServerMessage) bool { return m.GetPong() != nil })
	if m.GetPong().ClientTime != 123.5 {
		t.Fatalf("pong time = %v", m.GetPong().ClientTime)
	}
}

func TestInvalidNameIsRejected(t *testing.T) {
	url, ctx := startServer(t)
	c := dial(t, ctx, url, "x")
	_, _, err := c.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("expected policy violation close, got %v", err)
	}
}
