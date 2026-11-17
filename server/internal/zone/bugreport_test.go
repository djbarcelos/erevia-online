package zone

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPrepareBugThroughZoneSharesChatCooldown(t *testing.T) {
	z := newTestZone(make(chan []SavedState, 16)) // ao parar, a zona salva quem está online
	z.cfg.Version = "abc123"
	mod := newFakeMod()
	z.SetModBackend(mod)
	p := &fakePeer{}
	z.handle(joinEvent{info: JoinInfo{ID: 1, Name: "Ana", Class: "mage", AccountID: 9, X: 12, Z: -8, UserAgent: "Firefox", Peer: p}})
	pos := z.players[1].pos // antes do Run: depois, só a goroutine da zona mexe nos jogadores
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { z.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	// Outra conta, personagem offline ou id inválido: não está online.
	for _, c := range []struct{ acc, char int64 }{{10, 1}, {9, 2}, {9, 0}, {9, 1 << 40}} {
		if _, err := z.PrepareBug(context.Background(), c.acc, c.char, "fiquei preso na pedra"); !errors.Is(err, ErrBugNotOnline) {
			t.Fatalf("account %d char %d: err = %v, want ErrBugNotOnline", c.acc, c.char, err)
		}
	}
	if _, err := z.PrepareBug(context.Background(), 9, 1, "curto"); !errors.Is(err, ErrBugTooShort) {
		t.Fatalf("short: err = %v", err)
	}

	r, err := z.PrepareBug(context.Background(), 9, 1, "fiquei preso na pedra do portão")
	if err != nil {
		t.Fatal(err)
	}
	if r.AccountID != 9 || r.CharacterID != 1 || r.Name != "Ana" || r.Version != "abc123" || r.UserAgent != "Firefox" ||
		r.X != pos.X || r.Z != pos.Z || r.Text != "fiquei preso na pedra do portão" {
		t.Fatalf("report = %+v", r)
	}
	if len(mod.bugs) != 0 {
		t.Fatal("PrepareBug must not save (the API does)")
	}

	// O intervalo vale nos dois caminhos: logo depois, nem o HTTP nem o chat.
	var cd *BugCooldownError
	_, err = z.PrepareBug(context.Background(), 9, 1, "outro problema aqui agora")
	if !errors.As(err, &cd) || cd.Seconds < 29 || cd.Seconds > 30 {
		t.Fatalf("cooldown: err = %v", err)
	}
	if !strings.HasPrefix(BugErrorText(err), "Espere ") {
		t.Fatalf("cooldown text = %q", BugErrorText(err))
	}
}

func TestBugErrorTexts(t *testing.T) {
	for err, want := range map[error]string{
		ErrBugTooShort:                "Uso: /bug",
		ErrBugNotOnline:               "Entre no jogo",
		&BugCooldownError{Seconds: 7}: "Espere 7 s",
		ErrZoneBusy:                   "Não foi possível",
	} {
		if got := BugErrorText(err); !strings.HasPrefix(got, want) {
			t.Fatalf("BugErrorText(%v) = %q, want prefix %q", err, got, want)
		}
	}
}
