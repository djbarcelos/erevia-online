package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

func TestBanMuteAndGMFlag(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "a", "a")
	c, err := s.CreateCharacter(ctx, acc, "Rufia", "rogue", game.DefaultLook("rogue"))
	if err != nil {
		t.Fatal(err)
	}
	token, _ := s.CreateSession(ctx, acc)

	// Recém-criada: não é GM nem está silenciada.
	got, err := s.CharacterForAccount(ctx, acc, c.ID)
	if err != nil || got.GM || got.MutedUntil != nil {
		t.Fatalf("fresh account: gm=%v muted=%v err=%v", got.GM, got.MutedUntil, err)
	}

	// Silêncio grava e volta no carregamento; zero tira.
	until := time.Now().Add(10 * time.Minute).Truncate(time.Second)
	if err := s.SetMute(ctx, acc, until); err != nil {
		t.Fatal(err)
	}
	got, _ = s.CharacterForAccount(ctx, acc, c.ID)
	if got.MutedUntil == nil || !got.MutedUntil.Equal(until) {
		t.Fatalf("muted_until = %v, want %v", got.MutedUntil, until)
	}
	s.SetMute(ctx, acc, time.Time{})
	if got, _ = s.CharacterForAccount(ctx, acc, c.ID); got.MutedUntil != nil {
		t.Fatalf("unmute: %v", got.MutedUntil)
	}

	// Banir pelo nome (sem diferenciar maiúsculas) derruba a sessão; desbanir devolve.
	id, err := s.SetBanned(ctx, "rUFIA", true, "trapaça")
	if err != nil || id != acc {
		t.Fatalf("SetBanned = %d, %v", id, err)
	}
	if _, err := s.AccountBySession(ctx, token); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("banned account still authenticates: %v", err)
	}
	if _, err := s.SetBanned(ctx, "Rufia", false, ""); err != nil {
		t.Fatal(err)
	}
	if got, err := s.AccountBySession(ctx, token); err != nil || got != acc {
		t.Fatalf("unbanned: %d, %v", got, err)
	}
	if _, err := s.SetBanned(ctx, "Ninguem", true, ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown name: %v", err)
	}

	if err := s.LogGM(ctx, store.GMEntry{GMAccountID: acc, GMName: "Rufia", Command: "ban", Target: "Rufia", Detail: "trapaça"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveBugReport(ctx, store.BugReport{AccountID: acc, CharacterID: c.ID, CharacterName: "Rufia",
		Level: 3, X: 1.5, Z: -2, Version: "abc", UserAgent: "teste", Text: "caí no chão"}); err != nil {
		t.Fatal(err)
	}
}
