package store_test

import (
	"context"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

func TestAchievementsAndTitleRoundTrip(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "ach", "ach")
	c, err := s.CreateCharacter(ctx, acc, "Conquista", "knight", game.DefaultLook("knight"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.CharacterForAccount(ctx, acc, c.ID)
	if err != nil || len(got.Achievements.Unlocked) != 0 || got.Title != "" {
		t.Fatalf("new character achievements = %+v title %q, %v", got.Achievements, got.Title, err)
	}

	st := game.AchievementState{Unlocked: map[string]int64{"first_kill": 1700000000000}, Counts: map[string]int{"bones_100": 37}}
	if err := s.SaveStates(ctx, []store.CharacterState{{CharacterID: c.ID, Level: 2, XP: 200, HP: -1, MP: -1,
		Achievements: st, Title: "first_kill", Seq: 1}}); err != nil {
		t.Fatal(err)
	}
	got, err = s.CharacterForAccount(ctx, acc, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Achievements.Unlocked["first_kill"] != 1700000000000 || got.Achievements.Counts["bones_100"] != 37 || got.Title != "first_kill" {
		t.Fatalf("after save: %+v title %q", got.Achievements, got.Title)
	}
}

func TestRankingsOrderPositionsAndClans(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	type spec struct {
		name         string
		level        int
		xp           int64
		wins         int
		title        string
		banned, clan bool
	}
	specs := []spec{
		{name: "Alto", level: 15, xp: 9000, wins: 1, title: "level_10", clan: true},
		{name: "Medio", level: 15, xp: 8000, wins: 5, clan: true},
		{name: "Baixo", level: 3, xp: 300},
		{name: "Banido", level: 20, xp: 99999, wins: 9, banned: true},
	}
	ids := map[string]int64{}
	var states []store.CharacterState
	for i, sp := range specs {
		acc, _ := s.LoginIdentity(ctx, "dev", sp.name, sp.name)
		c, err := s.CreateCharacter(ctx, acc, sp.name, "knight", game.DefaultLook("knight"))
		if err != nil {
			t.Fatal(err)
		}
		ids[sp.name] = c.ID
		states = append(states, store.CharacterState{CharacterID: c.ID, Level: sp.level, XP: sp.xp, HP: -1, MP: -1,
			PvPKills: sp.wins, Title: sp.title, Seq: int64(i + 1)})
	}
	if err := s.SaveStates(ctx, states); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetBanned(ctx, "Banido", true, "teste"); err != nil {
		t.Fatal(err)
	}
	clanA, err := s.CreateClan(ctx, "Lobos", ids["Alto"])
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyClanOp(ctx, store.ClanOp{Kind: store.ClanOpJoin, ClanID: clanA, CharacterID: ids["Medio"]}); err != nil {
		t.Fatal(err)
	}
	clanB, err := s.CreateClan(ctx, "Corvos", ids["Baixo"])
	if err != nil {
		t.Fatal(err)
	}

	r, err := s.Rankings(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	// Nível: banido fora, empate de nível desfeito pela XP, lista cortada no limite.
	if len(r.Level) != 2 || r.Level[0].Name != "Alto" || r.Level[1].Name != "Medio" {
		t.Fatalf("level ranking = %+v", r.Level)
	}
	if r.Level[0].ClanName != "Lobos" || r.Level[0].TitleID != "level_10" || r.Level[0].Class != "knight" {
		t.Fatalf("level row details = %+v", r.Level[0])
	}
	if r.LevelPos[ids["Baixo"]] != 3 || r.LevelPos[ids["Alto"]] != 1 || r.LevelPos[ids["Banido"]] != 0 {
		t.Fatalf("level positions = %v", r.LevelPos)
	}
	// PvP: só quem tem vitórias, por vitórias.
	if len(r.PvP) != 2 || r.PvP[0].Name != "Medio" || r.PvP[0].PvPKills != 5 || r.PvP[1].Name != "Alto" {
		t.Fatalf("pvp ranking = %+v", r.PvP)
	}
	if r.PvPPos[ids["Baixo"]] != 0 || r.PvPPos[ids["Medio"]] != 1 {
		t.Fatalf("pvp positions = %v", r.PvPPos)
	}
	// Clãs: soma dos níveis (15 + 15 contra 3), com média e líder.
	if len(r.Clans) != 2 || r.Clans[0].Name != "Lobos" || r.Clans[0].Members != 2 || r.Clans[0].LevelSum != 30 ||
		r.Clans[0].LevelAvg != 15 || r.Clans[0].Leader != "Alto" {
		t.Fatalf("clan ranking = %+v", r.Clans)
	}
	if r.ClanPos[clanA] != 1 || r.ClanPos[clanB] != 2 {
		t.Fatalf("clan positions = %v", r.ClanPos)
	}
}
