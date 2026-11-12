package store_test

import (
	"context"
	"errors"
	"testing"

	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

func TestLoginIdentityAndSessions(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()

	a1, err := s.LoginIdentity(ctx, "google", "sub-1", "Ana")
	if err != nil {
		t.Fatal(err)
	}
	a2, err := s.LoginIdentity(ctx, "google", "sub-1", "Ana Maria")
	if err != nil {
		t.Fatal(err)
	}
	if a1 != a2 {
		t.Fatalf("same identity should map to same account: %d vs %d", a1, a2)
	}
	other, _ := s.LoginIdentity(ctx, "discord", "sub-1", "Outro")
	if other == a1 {
		t.Fatal("different provider must be a different account")
	}
	if name, _ := s.DisplayName(ctx, a1); name != "Ana Maria" {
		t.Fatalf("display name = %q", name)
	}

	token, err := s.CreateSession(ctx, a1)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.AccountBySession(ctx, token)
	if err != nil || got != a1 {
		t.Fatalf("AccountBySession = %d, %v", got, err)
	}
	if _, err := s.AccountBySession(ctx, "invalido"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown token: %v", err)
	}
	if err := s.DeleteSession(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AccountBySession(ctx, token); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted session should be gone: %v", err)
	}
}

func TestCharacters(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "a", "a")
	acc2, _ := s.LoginIdentity(ctx, "dev", "b", "b")

	c, err := s.CreateCharacter(ctx, acc, "Aria", "mage")
	if err != nil {
		t.Fatal(err)
	}
	if c.ID == 0 || c.Name != "Aria" || c.Class != "mage" {
		t.Fatalf("bad character %+v", c)
	}

	if _, err := s.CreateCharacter(ctx, acc2, "aria", "knight"); !errors.Is(err, store.ErrNameTaken) {
		t.Fatalf("case-insensitive duplicate name: %v", err)
	}

	s.CreateCharacter(ctx, acc, "Beto", "rogue")
	s.CreateCharacter(ctx, acc, "Caio", "knight")
	if _, err := s.CreateCharacter(ctx, acc, "Dani", "barbarian"); !errors.Is(err, store.ErrTooManyCharacters) {
		t.Fatalf("4th character: %v", err)
	}

	list, err := s.ListCharacters(ctx, acc)
	if err != nil || len(list) != 3 {
		t.Fatalf("list = %v, %v", list, err)
	}

	if _, err := s.CharacterForAccount(ctx, acc2, c.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("other account must not load character: %v", err)
	}

	if err := s.SavePositions(ctx, []store.Position{{CharacterID: c.ID, X: 12.5, Z: -7.25}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.CharacterForAccount(ctx, acc, c.ID)
	if err != nil || got.X != 12.5 || got.Z != -7.25 {
		t.Fatalf("saved position = %+v, %v", got, err)
	}
}
