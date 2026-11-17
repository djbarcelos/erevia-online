package store_test

import (
	"context"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

func TestSpecRoundTrip(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "spec", "spec")
	c, err := s.CreateCharacter(ctx, acc, "Caminho", "knight", game.DefaultLook("knight"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Spec != "" {
		t.Fatalf("personagem novo com especialização %q", c.Spec)
	}
	st := store.CharacterState{CharacterID: c.ID, Level: 20, XP: game.XPLevelStart(20), HP: -1, MP: -1, Spec: "bastion", Seq: 1}
	if err := s.SaveStates(ctx, []store.CharacterState{st}); err != nil {
		t.Fatal(err)
	}
	got, err := s.CharacterForAccount(ctx, acc, c.ID)
	if err != nil || got.Spec != "bastion" {
		t.Fatalf("spec = %q / %v", got.Spec, err)
	}
	list, err := s.ListCharacters(ctx, acc)
	if err != nil || len(list) != 1 || list[0].Spec != "bastion" {
		t.Fatalf("lista de personagens: %+v / %v", list, err)
	}
}
