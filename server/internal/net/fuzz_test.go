package net

// Fuzz do caminho de entrada real (tunnel#118): bytes pelo WebSocket → readJoin/readPump
// (proto.Unmarshal e o switch que chama a zona) → zona rodando o próprio loop. Nada pode
// derrubar o processo nem travar a zona. As invariantes de ouro, itens, posição e vida
// ficam nos alvos de internal/zone (FuzzZoneBytes e FuzzZoneSemantic), que enxergam o
// estado por dentro.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/zone"
)

// fuzzConnEnv: um servidor por processo de fuzz (a zona roda o loop de verdade, a 20 Hz)
// e a conexão já dentro do jogo, reaproveitada entre as entradas.
type fuzzConnEnv struct {
	url  string
	conn *websocket.Conn
	seq  int
}

var fuzzEnv = sync.OnceValue(func() *fuzzConnEnv {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := zone.DefaultConfig()
	cfg.Spawns = nil
	cfg.SafeRadius = 0
	// Chat sem limite: cada entrada termina com um comando de sincronia (ver FuzzConnBytes).
	cfg.ChatPerSec, cfg.ChatBurst = 1e9, 1e9
	z := zone.New(cfg, log, nil)
	go z.Run(context.Background())
	backend := &fakeBackend{chars: map[int64][]Character{
		1: {{ID: 10, Name: "Alice", Class: "mage", X: 1, Z: 2, Gold: 5000, Items: game.StarterKit("mage"), AccountID: 1}},
		2: {{ID: 20, Name: "Bob", Class: "knight", X: 3, Z: 2, Gold: 100, Items: game.StarterKit("knight"), AccountID: 2}},
	}}
	srv := httptest.NewServer(NewServer(z, backend, log, []string{"*"}))
	return &fuzzConnEnv{url: "ws" + strings.TrimPrefix(srv.URL, "http")}
})

func (e *fuzzConnEnv) dial(ctx context.Context, account int64) (*websocket.Conn, error) {
	h := http.Header{}
	h.Set("Cookie", fmt.Sprintf("acc=%d", account))
	c, _, err := websocket.Dial(ctx, e.url, &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		return nil, err
	}
	c.SetReadLimit(1 << 20) // o Welcome passa dos 32 KB padrão
	return c, nil
}

// joined devolve a conexão da Alice já com o Welcome (conecta de novo se caiu).
func (e *fuzzConnEnv) joined(ctx context.Context) (*websocket.Conn, error) {
	if e.conn != nil {
		return e.conn, nil
	}
	c, err := e.dial(ctx, 1)
	if err != nil {
		return nil, err
	}
	join, _ := proto.Marshal(&pb.ClientMessage{Msg: &pb.ClientMessage_Join{Join: &pb.Join{CharacterId: 10}}})
	if err := c.Write(ctx, websocket.MessageBinary, join); err != nil {
		c.CloseNow()
		return nil, err
	}
	if _, err := fuzzWaitFor(ctx, c, func(m *pb.ServerMessage) bool { return m.GetWelcome() != nil }); err != nil {
		c.CloseNow()
		return nil, err
	}
	e.conn = c
	return c, nil
}

func (e *fuzzConnEnv) drop() {
	if e.conn != nil {
		e.conn.CloseNow()
		e.conn = nil
	}
}

func fuzzWaitFor(ctx context.Context, c *websocket.Conn, match func(*pb.ServerMessage) bool) (*pb.ServerMessage, error) {
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return nil, err
		}
		var m pb.ServerMessage
		if err := proto.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("o servidor mandou uma mensagem que não decodifica: %w", err)
		}
		if match(&m) {
			return &m, nil
		}
	}
}

// FuzzConnBytes manda os bytes como uma mensagem binária (e, em 1 de cada 8 entradas,
// também como a primeira mensagem de uma conexão nova, que passa pelo readJoin). Depois
// manda "/fuzzsyncN" e espera a resposta da zona ("Comando desconhecido"): assim cada
// entrada só termina depois que a zona tratou tudo o que veio antes. Um panic na zona
// derruba o processo; uma zona travada estoura o prazo.
func FuzzConnBytes(f *testing.F) {
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		env := fuzzEnv()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if len(data)%8 == 0 {
			c, err := env.dial(ctx, 2)
			if err != nil {
				t.Fatalf("dial: %v", err)
			}
			if c.Write(ctx, websocket.MessageBinary, data) == nil {
				// Join válido do Bob entra no jogo; o resto é recusado e fechado.
				fuzzWaitFor(ctx, c, func(m *pb.ServerMessage) bool { return m.GetWelcome() != nil })
			}
			if ctx.Err() != nil {
				t.Fatal("readJoin não respondeu nem fechou a conexão a tempo")
			}
			c.CloseNow()
		}

		c, err := env.joined(ctx)
		if err != nil {
			t.Fatalf("entrar no jogo: %v", err)
		}
		if err := c.Write(ctx, websocket.MessageBinary, data); err != nil {
			env.drop() // o servidor fechou (ex.: mensagem maior que o limite)
			return
		}
		env.seq++
		token := fmt.Sprintf("/fuzzsync%d", env.seq)
		sync, _ := proto.Marshal(&pb.ClientMessage{Msg: &pb.ClientMessage_ChatSend{ChatSend: &pb.ChatSend{Text: token}}})
		if err := c.Write(ctx, websocket.MessageBinary, sync); err != nil {
			env.drop()
			return
		}
		_, err = fuzzWaitFor(ctx, c, func(m *pb.ServerMessage) bool {
			ch := m.GetChat()
			return ch != nil && ch.Channel == pb.ChatChannel_CHAT_CHANNEL_SYSTEM && strings.Contains(ch.Text, token)
		})
		switch {
		case err == nil:
		case ctx.Err() != nil:
			t.Fatalf("a zona não respondeu em 10 s depois de %x", data)
		case errors.As(err, new(websocket.CloseError)) || errors.Is(err, io.EOF):
			env.drop() // fechada pelo servidor (limite de tamanho, expulsão…): conecta de novo
		default:
			if strings.Contains(err.Error(), "não decodifica") {
				t.Fatal(err)
			}
			env.drop()
		}
	})
}
