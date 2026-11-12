package zone

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"erevia/server/internal/pb"
)

// fakePeer guarda as mensagens recebidas.
type fakePeer struct {
	mu     sync.Mutex
	msgs   []*pb.ServerMessage
	closed string
}

func (f *fakePeer) Send(data []byte) bool {
	var m pb.ServerMessage
	if err := proto.Unmarshal(data, &m); err != nil {
		panic(err)
	}
	f.mu.Lock()
	f.msgs = append(f.msgs, &m)
	f.mu.Unlock()
	return true
}

func (f *fakePeer) Close(reason string) {
	f.mu.Lock()
	f.closed = reason
	f.mu.Unlock()
}

func (f *fakePeer) chats() []*pb.ChatMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*pb.ChatMessage
	for _, m := range f.msgs {
		if c := m.GetChat(); c != nil {
			out = append(out, c)
		}
	}
	return out
}

// newTestZone cria uma zona sem rodar o loop: os testes chamam handle() direto.
// Sem monstros: os testes de combate criam os seus com newCombatZone.
func newTestZone(saves chan []SavedState) *Zone {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	var ch chan<- []SavedState
	if saves != nil {
		ch = saves
	}
	cfg := DefaultConfig()
	cfg.Spawns = nil
	return New(cfg, log, ch)
}

func join(z *Zone, id uint32, name string) *fakePeer {
	p := &fakePeer{}
	z.handle(joinEvent{info: JoinInfo{ID: id, Name: name, Class: "knight", Peer: p}})
	return p
}

func TestChatGeneralReachesEveryone(t *testing.T) {
	z := newTestZone(nil)
	a, b := join(z, 1, "Ana"), join(z, 2, "Beto")

	z.handle(chatEvent{id: 1, text: "  olá   pessoal  "})

	for _, p := range []*fakePeer{a, b} {
		c := p.chats()
		if len(c) != 1 || c[0].Text != "olá pessoal" || c[0].FromName != "Ana" ||
			c[0].Channel != pb.ChatChannel_CHAT_CHANNEL_GENERAL {
			t.Fatalf("unexpected chats: %v", c)
		}
	}
}

func TestWhisperOnlyReachesTargetAndSender(t *testing.T) {
	z := newTestZone(nil)
	a, b, c := join(z, 1, "Ana"), join(z, 2, "Beto"), join(z, 3, "Caio")

	z.handle(chatEvent{id: 1, text: "segredo", to: "beto"}) // sem diferenciar maiúsculas

	for _, p := range []*fakePeer{a, b} {
		got := p.chats()
		if len(got) != 1 || got[0].Channel != pb.ChatChannel_CHAT_CHANNEL_WHISPER || got[0].ToName != "Beto" {
			t.Fatalf("whisper not delivered: %v", got)
		}
	}
	if len(c.chats()) != 0 {
		t.Fatal("third player must not see whisper")
	}
}

func TestWhisperOfflineGetsSystemMessage(t *testing.T) {
	z := newTestZone(nil)
	a := join(z, 1, "Ana")
	z.handle(chatEvent{id: 1, text: "oi", to: "Ninguem"})
	got := a.chats()
	if len(got) != 1 || got[0].Channel != pb.ChatChannel_CHAT_CHANNEL_SYSTEM || !strings.Contains(got[0].Text, "não está online") {
		t.Fatalf("expected system message, got %v", got)
	}
}

func TestChatRateLimit(t *testing.T) {
	z := newTestZone(nil)
	a := join(z, 1, "Ana")
	for i := 0; i < 5; i++ {
		z.handle(chatEvent{id: 1, text: "spam"})
	}
	var general, system int
	for _, c := range a.chats() {
		switch c.Channel {
		case pb.ChatChannel_CHAT_CHANNEL_GENERAL:
			general++
		case pb.ChatChannel_CHAT_CHANNEL_SYSTEM:
			system++
		}
	}
	if general != 3 || system != 2 {
		t.Fatalf("burst should allow 3, got general=%d system=%d", general, system)
	}
}

func TestSanitizeChat(t *testing.T) {
	cases := map[string]string{
		"  oi  ":                 "oi",
		"a\x00b\x1bc":            "abc",
		"linha1\nlinha2":         "linha1 linha2",
		"zero​width":             "zerowidth",
		"\t\n ":                  "",
		strings.Repeat("x", 500): strings.Repeat("x", MaxChatRunes),
		"<b>html</b> é só texto": "<b>html</b> é só texto",
	}
	for in, want := range cases {
		if got := SanitizeChat(in); got != want {
			t.Errorf("SanitizeChat(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReloginKicksOldSession(t *testing.T) {
	z := newTestZone(nil)
	old := join(z, 1, "Ana")
	newer := join(z, 1, "Ana")

	if old.closed == "" {
		t.Fatal("old session should be closed")
	}
	var kicked bool
	for _, m := range old.msgs {
		if m.GetKicked() != nil {
			kicked = true
		}
	}
	if !kicked {
		t.Fatal("old session should receive Kicked")
	}

	// O Leave atrasado da conexão antiga não pode remover o novo login.
	z.handle(leaveEvent{id: 1, peer: old})
	if p, ok := z.players[1]; !ok || p.pl.peer != newer {
		t.Fatal("new session must survive old session's leave")
	}
}

func TestPositionsSavedOnLeaveAndShutdown(t *testing.T) {
	saves := make(chan []SavedState, 10)
	z := newTestZone(saves)
	a := &fakePeer{}
	z.handle(joinEvent{info: JoinInfo{ID: 7, Name: "Ana", Class: "mage", X: 3, Z: 4, Peer: a}})
	join(z, 8, "Beto")

	z.handle(leaveEvent{id: 7, peer: a})
	got := <-saves
	if len(got) != 1 || got[0] != (SavedState{ID: 7, X: 3, Z: 4, Level: 1, HP: -1, MP: -1}) {
		t.Fatalf("save on leave = %v", got)
	}

	// Desligar a zona salva quem ficou e fecha o canal.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { z.Run(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("zone did not stop")
	}
	final := <-saves
	if len(final) != 1 || final[0].ID != 8 {
		t.Fatalf("final save = %v", final)
	}
	if _, open := <-saves; open {
		t.Fatal("saves channel should be closed")
	}
}
