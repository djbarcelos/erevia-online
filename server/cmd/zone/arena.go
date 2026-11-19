package main

import (
	"context"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/zone"
)

// arenaStore liga a arena da zona ao banco (tunnel#81).
type arenaStore struct{ st *store.Store }

func (a arenaStore) RecordArenaMatch(ctx context.Context, r zone.ArenaResult) ([]zone.ArenaChange, error) {
	var teams [2][]int64
	for i, t := range r.Teams {
		for _, id := range t {
			teams[i] = append(teams[i], int64(id))
		}
	}
	changes, err := a.st.RecordArenaMatch(ctx, store.ArenaResult{Season: r.Season, Mode: r.Mode, Teams: teams, Winner: r.Winner})
	if err != nil {
		return nil, err
	}
	out := make([]zone.ArenaChange, len(changes))
	for i, c := range changes {
		out[i] = zone.ArenaChange{ID: uint32(c.CharacterID), Before: c.Before, After: c.After}
	}
	return out, nil
}

func (a arenaStore) ArenaRanking(ctx context.Context, season int, mode game.ArenaMode, limit int) ([]zone.ArenaRank, error) {
	rows, err := a.st.ArenaRanking(ctx, season, mode, limit)
	if err != nil {
		return nil, err
	}
	out := make([]zone.ArenaRank, len(rows))
	for i, r := range rows {
		out[i] = zone.ArenaRank{Name: r.Name, Class: r.Class, Level: r.Level, Rating: r.Rating, Wins: r.Wins, Losses: r.Losses}
	}
	return out, nil
}

func (a arenaStore) ArenaRatingsOf(ctx context.Context, season int, id uint32) (map[game.ArenaMode]zone.ArenaRecord, error) {
	m, err := a.st.ArenaRatingsOf(ctx, season, int64(id))
	if err != nil {
		return nil, err
	}
	out := map[game.ArenaMode]zone.ArenaRecord{}
	for mode, r := range m {
		out[mode] = zone.ArenaRecord{Rating: r.Rating, Wins: r.Wins, Losses: r.Losses}
	}
	return out, nil
}
