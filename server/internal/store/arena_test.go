package store_test

import (
	"context"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

// Arena (tunnel#81): a partida aplica o Elo pela média dos times, começa em 1000 e o ranking
// é por temporada e modo.
func TestArenaMatchAndRanking(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	var ids []int64
	for i, name := range []string{"Arenaum", "Arenadois", "Arenatres", "Arenaquatro"} {
		acc, _ := s.LoginIdentity(ctx, "dev", name, name)
		c, err := s.CreateCharacter(ctx, acc, name, []string{"knight", "mage", "rogue", "barbarian"}[i], game.DefaultLook("knight"))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, c.ID)
	}
	const season = 202611
	got, err := s.RecordArenaMatch(ctx, store.ArenaResult{Season: season, Mode: game.ArenaSolo, Teams: [2][]int64{{ids[0]}, {ids[1]}}, Winner: 0})
	if err != nil || len(got) != 2 {
		t.Fatal(got, err)
	}
	if got[0].Before != 1000 || got[0].After != 1016 || got[1].After != 984 {
		t.Fatalf("primeira partida (iguais): %+v", got)
	}
	// O mesmo vence de novo: ganha menos (já estava acima).
	got, _ = s.RecordArenaMatch(ctx, store.ArenaResult{Season: season, Mode: game.ArenaSolo, Teams: [2][]int64{{ids[0]}, {ids[1]}}, Winner: 0})
	if d := got[0].After - got[0].Before; d <= 0 || d >= 16 {
		t.Fatalf("o favorito ganha menos de 16: %+v", got)
	}
	// 3x3 é outra pontuação; e outra temporada começa de novo.
	if _, err := s.RecordArenaMatch(ctx, store.ArenaResult{Season: season, Mode: game.ArenaTrios,
		Teams: [2][]int64{{ids[2], ids[3], ids[0]}, {ids[1]}}, Winner: 1}); err != nil {
		t.Fatal(err)
	}
	rank, err := s.ArenaRanking(ctx, season, game.ArenaSolo, 10)
	if err != nil || len(rank) != 2 || rank[0].CharacterID != ids[0] || rank[0].Wins != 2 || rank[1].Losses != 2 || rank[0].Name != "Arenaum" {
		t.Fatalf("ranking 1x1: %+v %v", rank, err)
	}
	mine, err := s.ArenaRatingsOf(ctx, season, ids[0])
	if err != nil || mine[game.ArenaSolo].Rating <= 1016 || mine[game.ArenaTrios].Losses != 1 {
		t.Fatalf("pontuações de Arenaum: %+v %v", mine, err)
	}
	if r, _ := s.ArenaRanking(ctx, season+1, game.ArenaSolo, 10); len(r) != 0 {
		t.Fatal("a temporada seguinte começa vazia")
	}
	if _, err := s.RecordArenaMatch(ctx, store.ArenaResult{Season: season, Mode: 2, Teams: [2][]int64{{ids[0]}, {ids[1]}}}); err == nil {
		t.Fatal("modo inválido")
	}
}
