package store_test

import (
	"context"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

// O bestiário vai junto com o personagem (coluna bestiary, migração 00031), com a cor de
// roupa exclusiva da família no visual.
func TestBestiaryRoundTrip(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "Caçador", "Caçador")
	c, err := s.CreateCharacter(ctx, acc, "Caçador", "rogue", game.DefaultLook("rogue"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Bestiary.Kills != nil || c.Bestiary.Families != nil {
		t.Fatalf("personagem novo com bestiário: %+v", c.Bestiary)
	}
	outfit := game.BestiaryFamilies[1].OutfitIndex()
	b := game.BestiaryState{
		Kills:    map[string]game.BestiaryKill{"stone_golem": {N: 512, Lo: 20, Hi: 30}, "skeleton_minion": {N: 3, Lo: 1, Hi: 2}},
		Families: map[string]int64{"golems": 1_763_000_000_000},
	}
	if err := s.SaveStates(ctx, []store.CharacterState{{CharacterID: c.ID, Level: 25, XP: game.XPLevelStart(25), HP: -1, MP: -1,
		Appearance: game.Appearance{Outfit: outfit}, Bestiary: b, Title: "bst:golems", Seq: 1}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.CharacterForAccount(ctx, acc, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	g := got.Bestiary
	if g.Kills["stone_golem"] != (game.BestiaryKill{N: 512, Lo: 20, Hi: 30}) || g.Kills["skeleton_minion"].N != 3 || g.Families["golems"] != 1_763_000_000_000 {
		t.Fatalf("bestiário carregado: %+v", g)
	}
	if got.Appearance.Outfit != outfit || got.Title != "bst:golems" {
		t.Fatalf("a cor do bestiário e o título de família voltam do banco: outfit %d, título %q", got.Appearance.Outfit, got.Title)
	}
}
