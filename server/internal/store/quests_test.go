package store_test

import (
	"context"
	"slices"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

func TestQuestsRoundTrip(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "q", "q")
	c, err := s.CreateCharacter(ctx, acc, "Quester", "knight", game.DefaultLook("knight"))
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.CharacterForAccount(ctx, acc, c.ID)
	if err != nil || len(got.Quests.Active) != 0 || len(got.Quests.Done) != 0 {
		t.Fatalf("new character quests = %+v, %v", got.Quests, err)
	}

	q := game.QuestState{Active: map[string][]int{"tut_skill": {2}}, Done: []string{"tut_gate", "tut_hunt"}}
	if err := s.SaveStates(ctx, []store.CharacterState{{CharacterID: c.ID, Level: 2, XP: 200, HP: -1, MP: -1, Quests: q, Seq: 1}}); err != nil {
		t.Fatal(err)
	}
	got, err = s.CharacterForAccount(ctx, acc, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Quests.Active["tut_skill"], []int{2}) || !slices.Equal(got.Quests.Done, q.Done) {
		t.Fatalf("quests after save = %+v", got.Quests)
	}
}
