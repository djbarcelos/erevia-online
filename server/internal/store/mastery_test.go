package store_test

import (
	"context"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

func TestMasteryRoundTripAndRanking(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	mk := func(name string) int64 {
		acc, _ := s.LoginIdentity(ctx, "dev", name, name)
		c, err := s.CreateCharacter(ctx, acc, name, "knight", game.DefaultLook("knight"))
		if err != nil {
			t.Fatal(err)
		}
		if c.Mastery.Points != 0 || c.Mastery.Ranks != nil {
			t.Fatalf("personagem novo com Maestria: %+v", c.Mastery)
		}
		return c.ID
	}
	dono, outro := mk("Dono"), mk("Outro")
	m := game.MasteryState{Points: 12, XP: 4321, Rested: 999, Day: 20770, RespecDay: 20769,
		Ranks: map[string]int{"wayfarer": 5, "light_step": 5, "way_home": 2}}
	xp := game.XPLevelStart(30)
	if err := s.SaveStates(ctx, []store.CharacterState{
		{CharacterID: dono, Level: 30, XP: xp, HP: -1, MP: -1, Mastery: m, Seq: 1},
		{CharacterID: outro, Level: 30, XP: xp + 50, HP: -1, MP: -1, Mastery: game.MasteryState{Points: 3}, Seq: 1},
	}); err != nil {
		t.Fatal(err)
	}
	acc, _ := s.LoginIdentity(ctx, "dev", "Dono", "Dono")
	got, err := s.CharacterForAccount(ctx, acc, dono)
	if err != nil {
		t.Fatal(err)
	}
	g := got.Mastery
	if g.Points != 12 || g.XP != 4321 || g.Rested != 999 || g.Day != 20770 || g.RespecDay != 20769 || g.Ranks["way_home"] != 2 || len(g.Ranks) != 3 {
		t.Fatalf("Maestria carregada: %+v", g)
	}

	// No ranking de nível, no 30 a Maestria desempata antes da XP, e vai na linha.
	r, err := s.Rankings(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Level) < 2 || r.Level[0].CharacterID != dono || r.Level[0].Mastery != 12 || r.Level[1].Mastery != 3 || r.LevelPos[outro] != 2 {
		t.Fatalf("ranking: %+v", r.Level)
	}
}
