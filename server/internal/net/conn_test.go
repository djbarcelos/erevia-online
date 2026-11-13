package net

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
	"erevia/server/internal/zone"
)

// fakeBackend: o cookie "acc" diz a conta; personagens fixos por conta.
type fakeBackend struct {
	chars map[int64][]Character
}

func (f *fakeBackend) Authenticate(r *http.Request) (int64, error) {
	c, err := r.Cookie("acc")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(c.Value, 10, 64)
}

func (f *fakeBackend) LoadCharacter(_ context.Context, accountID int64, id uint32) (Character, error) {
	for _, c := range f.chars[accountID] {
		if c.ID == id {
			return c, nil
		}
	}
	return Character{}, ErrCharacterNotFound
}

func startServer(t *testing.T) (url string, ctx context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := zone.DefaultConfig()
	cfg.SafeRadius = 0 // os testes lutam perto da origem
	cfg.Spawns = []game.Spawn{{Monster: "skeleton_minion", Count: 1, Fixed: true, X: 18, Z: 0, LevelMin: 1, Respawn: 5}}
	z := zone.New(cfg, log, nil)
	go z.Run(ctx)

	backend := &fakeBackend{chars: map[int64][]Character{
		1: {{ID: 10, Name: "Alice", Class: "mage", X: 1, Z: 2}},
		2: {{ID: 20, Name: "Bob", Class: "knight", Items: game.StarterKit("knight")}},
	}}
	srv := httptest.NewServer(NewServer(z, backend, log, []string{"*"}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http"), ctx
}

func dialAs(t *testing.T, ctx context.Context, url string, account int64) (*websocket.Conn, error) {
	t.Helper()
	h := http.Header{}
	h.Set("Cookie", "acc="+strconv.FormatInt(account, 10))
	c, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: h})
	if err == nil {
		// O navegador não limita o tamanho das mensagens; a biblioteca limita a 32 KB por
		// padrão, e o Welcome (catálogo de skills com os textos de cada rank, itens…) já
		// passa disso com o 5º degrau da árvore.
		c.SetReadLimit(1 << 20)
		t.Cleanup(func() { c.CloseNow() })
	}
	return c, err
}

func join(t *testing.T, ctx context.Context, url string, account int64, charID uint32) *websocket.Conn {
	t.Helper()
	c, err := dialAs(t, ctx, url, account)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	send(t, ctx, c, &pb.ClientMessage{Msg: &pb.ClientMessage_Join{Join: &pb.Join{CharacterId: charID}}})
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

func isWelcome(m *pb.ServerMessage) bool { return m.GetWelcome() != nil }

func TestUnauthenticatedIsRejected(t *testing.T) {
	url, ctx := startServer(t)
	c, _, err := websocket.Dial(ctx, url, nil)
	if err == nil {
		c.CloseNow()
		t.Fatal("dial without session should fail")
	}
}

func TestCannotJoinWithOtherAccountsCharacter(t *testing.T) {
	url, ctx := startServer(t)
	c := join(t, ctx, url, 2, 10) // conta 2 tentando usar personagem da conta 1
	_, _, err := c.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("expected policy violation, got %v", err)
	}
}

func TestTwoPlayersMoveAndChat(t *testing.T) {
	url, ctx := startServer(t)

	a := join(t, ctx, url, 1, 10)
	welcomeA := waitFor(t, ctx, a, "welcome A", isWelcome).GetWelcome()
	if welcomeA.YourId != 10 {
		t.Fatalf("your_id = %d, want character id 10", welcomeA.YourId)
	}
	var self *pb.Entity
	for _, e := range welcomeA.Entities {
		if e.Id == 10 {
			self = e
		}
	}
	// A árvore inteira da classe (15 skills) mais as 6 dos dois caminhos da especialização,
	// com as 2 iniciais aprendidas, e a barra padrão.
	if len(welcomeA.Skills) != 15+6 || len(welcomeA.Branches) != 3 {
		t.Fatalf("welcome should carry the class tree, got %d skills / %d branches", len(welcomeA.Skills), len(welcomeA.Branches))
	}
	if welcomeA.Progress == nil || len(welcomeA.Progress.Ranks) != 2 || len(welcomeA.Progress.PerPoint) != 6 {
		t.Fatalf("welcome progress wrong: %+v", welcomeA.Progress)
	}
	if welcomeA.Hotbar == nil || len(welcomeA.Hotbar.Slots) != 10 || welcomeA.Hotbar.Slots[0] == "" {
		t.Fatalf("welcome hotbar wrong: %+v", welcomeA.Hotbar)
	}
	if self == nil || self.X != 1 || self.Z != 2 || self.Model != "mage" || self.Kind != pb.EntityKind_ENTITY_KIND_PLAYER {
		t.Fatalf("saved position/class not used: %+v", self)
	}

	b := join(t, ctx, url, 2, 20)
	waitFor(t, ctx, b, "welcome B", isWelcome)
	waitFor(t, ctx, a, "spawn Bob", func(m *pb.ServerMessage) bool {
		return m.GetSpawn().GetEntity().GetName() == "Bob"
	})

	// Movimento
	send(t, ctx, a, &pb.ClientMessage{Msg: &pb.ClientMessage_MoveTo{MoveTo: &pb.MoveTo{X: 20, Z: 0}}})
	waitFor(t, ctx, b, "snapshot with Alice", func(m *pb.ServerMessage) bool {
		for _, s := range m.GetSnapshot().GetStates() {
			if s.Id == 10 && s.TargetX == 20 {
				return true
			}
		}
		return false
	})

	// Chat geral
	send(t, ctx, a, &pb.ClientMessage{Msg: &pb.ClientMessage_ChatSend{ChatSend: &pb.ChatSend{Text: "olá"}}})
	got := waitFor(t, ctx, b, "general chat", func(m *pb.ServerMessage) bool { return m.GetChat() != nil }).GetChat()
	if got.Text != "olá" || got.FromName != "Alice" || got.Channel != pb.ChatChannel_CHAT_CHANNEL_GENERAL {
		t.Fatalf("chat = %+v", got)
	}

	// Sussurro
	send(t, ctx, b, &pb.ClientMessage{Msg: &pb.ClientMessage_ChatSend{ChatSend: &pb.ChatSend{Text: "psiu", To: "alice"}}})
	w := waitFor(t, ctx, a, "whisper", func(m *pb.ServerMessage) bool {
		return m.GetChat().GetChannel() == pb.ChatChannel_CHAT_CHANNEL_WHISPER
	}).GetChat()
	if w.Text != "psiu" || w.FromName != "Bob" || w.ToName != "Alice" {
		t.Fatalf("whisper = %+v", w)
	}

	// Alice sai; Bob recebe Despawn.
	a.Close(websocket.StatusNormalClosure, "")
	waitFor(t, ctx, b, "despawn Alice", func(m *pb.ServerMessage) bool {
		return m.GetDespawn() != nil && m.GetDespawn().Id == 10
	})
}

func TestReloginKicksFirstConnection(t *testing.T) {
	url, ctx := startServer(t)
	first := join(t, ctx, url, 1, 10)
	waitFor(t, ctx, first, "welcome 1", isWelcome)

	second := join(t, ctx, url, 1, 10)
	waitFor(t, ctx, second, "welcome 2", isWelcome)

	waitFor(t, ctx, first, "kicked", func(m *pb.ServerMessage) bool { return m.GetKicked() != nil })
	_, _, err := first.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("first connection should be closed, got %v", err)
	}

	// A segunda continua viva: responde ao ping.
	send(t, ctx, second, &pb.ClientMessage{Msg: &pb.ClientMessage_Ping{Ping: &pb.Ping{ClientTime: 1}}})
	waitFor(t, ctx, second, "pong", func(m *pb.ServerMessage) bool { return m.GetPong() != nil })
}

func TestAttackMonsterOverWebSocket(t *testing.T) {
	url, ctx := startServer(t)
	c := join(t, ctx, url, 2, 20) // Bob, cavaleiro, em (0,0)
	w := waitFor(t, ctx, c, "welcome", isWelcome).GetWelcome()
	var monster *pb.Entity
	for _, e := range w.Entities {
		if e.Kind == pb.EntityKind_ENTITY_KIND_MONSTER {
			monster = e
		}
	}
	if monster == nil || monster.Level != 1 || monster.MaxHp == 0 {
		t.Fatalf("welcome should include the monster: %+v", monster)
	}
	waitFor(t, ctx, c, "self stats", func(m *pb.ServerMessage) bool { return m.GetSelfStats().GetLevel() == 1 })

	send(t, ctx, c, &pb.ClientMessage{Msg: &pb.ClientMessage_Attack{Attack: &pb.Attack{TargetId: monster.Id}}})
	hit := waitFor(t, ctx, c, "combat event", func(m *pb.ServerMessage) bool {
		for _, ev := range m.GetSnapshot().GetCombat() {
			if ev.SourceId == 20 && ev.TargetId == monster.Id {
				return true
			}
		}
		return false
	})
	if hit == nil {
		t.Fatal("no combat")
	}
	// Mata o lacaio (50 de vida) e ganha XP.
	waitFor(t, ctx, c, "xp gained", func(m *pb.ServerMessage) bool { return m.GetSelfStats().GetXp() > 0 })
}
