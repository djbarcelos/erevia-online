package store_test

import (
	"context"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

// A vitalidade vai e volta do banco; o login traz a última saída e o maior nível da conta
// (a recuperação do 2º personagem), e um salvamento mais velho não desfaz o estoque.
func TestVitalityRoundTripAndAccountTop(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, err := s.LoginIdentity(ctx, "dev", "Vita", "Vita")
	if err != nil {
		t.Fatal(err)
	}
	top, err := s.CreateCharacter(ctx, acc, "Principal", "knight", game.DefaultLook("knight"))
	if err != nil {
		t.Fatal(err)
	}
	alt, err := s.CreateCharacter(ctx, acc, "Segundo", "mage", game.DefaultLook("mage"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.CharacterForAccount(ctx, acc, alt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Vitality.Stored != 0 || !got.Vitality.LastSeen.IsZero() || got.Vitality.AccountTop != 1 {
		t.Fatalf("personagem novo: %+v", got.Vitality)
	}

	if err := s.SaveStates(ctx, []store.CharacterState{
		{CharacterID: top.ID, Level: game.MaxLevel, XP: game.XPLevelStart(game.MaxLevel), HP: -1, MP: -1, Vitality: 999, Seq: 1},
		{CharacterID: alt.ID, Level: 12, XP: game.XPLevelStart(12), HP: -1, MP: -1, Vitality: 12345, Seq: 2},
	}); err != nil {
		t.Fatal(err)
	}
	// Um salvamento atrasado (Seq menor) é ignorado, a vitalidade junto.
	if err := s.SaveStates(ctx, []store.CharacterState{
		{CharacterID: alt.ID, Level: 12, XP: game.XPLevelStart(12), HP: -1, MP: -1, Vitality: 1, Seq: 1},
	}); err != nil {
		t.Fatal(err)
	}
	got, err = s.CharacterForAccount(ctx, acc, alt.ID)
	if err != nil {
		t.Fatal(err)
	}
	v := got.Vitality
	if v.Stored != 12345 || v.AccountTop != game.MaxLevel || time.Since(v.LastSeen) > time.Minute {
		t.Fatalf("vitalidade carregada: %+v", v)
	}
}
