package store_test

import (
	"context"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

// Contratos e reputação vão e voltam do banco junto com o personagem (migração 00032).
func TestContractsAndReputationRoundTrip(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "Iselda", "Iselda")
	c, err := s.CreateCharacter(ctx, acc, "Contratado", "knight", game.DefaultLook("knight"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Contracts.Taken != nil || c.Reputation.Points != nil {
		t.Fatalf("personagem novo com contratos/reputação: %+v %+v", c.Contracts, c.Reputation)
	}
	st := game.ContractState{Day: 20770, Week: 2966, Taken: map[string]*game.ContractProgress{
		"hunt_vale_golems": {N: 40},
		"weekly_pvp":       {N: 1, Seen: []uint32{7}},
		"collect_cores":    {N: 32, Done: true},
	}}
	rep := game.ReputationState{Points: map[string]int{game.FactionBastion: 1620, game.FactionGuard: 90}}
	if err := s.SaveStates(ctx, []store.CharacterState{
		{CharacterID: c.ID, Level: 30, XP: game.XPLevelStart(30), HP: -1, MP: -1, Contracts: st, Reputation: rep, Seq: time.Now().UnixNano()},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := s.CharacterForAccount(ctx, acc, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	g := got.Contracts
	if g.Day != 20770 || g.Week != 2966 || len(g.Taken) != 3 || g.Taken["hunt_vale_golems"].N != 40 ||
		!g.Taken["collect_cores"].Done || len(g.Taken["weekly_pvp"].Seen) != 1 || g.Taken["weekly_pvp"].Seen[0] != 7 {
		t.Fatalf("contratos voltaram diferentes: %+v", g)
	}
	if got.Reputation.Points[game.FactionBastion] != 1620 || got.Reputation.Points[game.FactionGuard] != 90 {
		t.Fatalf("reputação voltou diferente: %+v", got.Reputation)
	}
}
