package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"erevia/server/internal/game"
)

// Arena PvP ranqueada (tunnel#81): pontuação por temporada e modo (migração 00061).

// ArenaRating é a pontuação de um personagem num modo e numa temporada.
type ArenaRating struct {
	CharacterID int64
	Name        string
	Class       string
	Level       int
	Rating      int
	Wins        int
	Losses      int
}

// ArenaResult é uma partida terminada: os dois times (ids dos personagens) e qual venceu.
type ArenaResult struct {
	Season int
	Mode   game.ArenaMode
	Teams  [2][]int64
	Winner int // 0 ou 1
}

// ArenaChange é a pontuação nova de um personagem depois da partida.
type ArenaChange struct {
	CharacterID int64
	Before      int
	After       int
}

// RecordArenaMatch aplica a partida numa transação: lê as pontuações dos participantes
// (travando as linhas), calcula o Elo pela média de cada time (game.ArenaDelta) e grava.
// Quem não tinha linha começa em game.ArenaStartRating.
func (s *Store) RecordArenaMatch(ctx context.Context, r ArenaResult) ([]ArenaChange, error) {
	if !r.Mode.Valid() || r.Winner < 0 || r.Winner > 1 || len(r.Teams[0]) == 0 || len(r.Teams[1]) == 0 {
		return nil, fmt.Errorf("partida inválida: %+v", r)
	}
	var out []ArenaChange
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		out = out[:0]
		ratings := map[int64]int{}
		var ids []int64
		for _, t := range r.Teams {
			ids = append(ids, t...)
		}
		rows, err := tx.Query(ctx, `SELECT character_id, rating FROM arena_ratings
			WHERE season = $1 AND mode = $2 AND character_id = ANY($3) FOR UPDATE`, r.Season, int(r.Mode), ids)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int64
			var rt int
			if err := rows.Scan(&id, &rt); err != nil {
				rows.Close()
				return err
			}
			ratings[id] = rt
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		avg := func(t []int64) float64 {
			sum := 0
			for _, id := range t {
				if v, ok := ratings[id]; ok {
					sum += v
				} else {
					sum += game.ArenaStartRating
				}
			}
			return float64(sum) / float64(len(t))
		}
		means := [2]float64{avg(r.Teams[0]), avg(r.Teams[1])}
		for side, t := range r.Teams {
			won := side == r.Winner
			d := game.ArenaDelta(means[side], means[1-side], won)
			win, loss := 0, 1
			if won {
				win, loss = 1, 0
			}
			for _, id := range t {
				before, ok := ratings[id]
				if !ok {
					before = game.ArenaStartRating
				}
				after := max(0, before+d)
				if _, err := tx.Exec(ctx, `INSERT INTO arena_ratings (season, mode, character_id, rating, wins, losses)
					VALUES ($1, $2, $3, $4, $5, $6)
					ON CONFLICT (season, mode, character_id) DO UPDATE SET rating = $4,
						wins = arena_ratings.wins + $5, losses = arena_ratings.losses + $6, updated_at = now()`,
					r.Season, int(r.Mode), id, after, win, loss); err != nil {
					return err
				}
				out = append(out, ArenaChange{CharacterID: id, Before: before, After: after})
			}
		}
		return nil
	})
	return out, err
}

// ArenaRanking: o topo do modo na temporada (maior pontuação; desempate: mais vitórias, id).
// Contas banidas ficam de fora.
func (s *Store) ArenaRanking(ctx context.Context, season int, mode game.ArenaMode, limit int) ([]ArenaRating, error) {
	rows, err := s.pool.Query(ctx, `SELECT r.character_id, c.name, c.class, c.level, r.rating, r.wins, r.losses
		FROM arena_ratings r JOIN characters c ON c.id = r.character_id JOIN accounts a ON a.id = c.account_id
		WHERE r.season = $1 AND r.mode = $2 AND NOT a.banned
		ORDER BY r.rating DESC, r.wins DESC, r.character_id LIMIT $3`, season, int(mode), limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (ArenaRating, error) {
		var a ArenaRating
		err := row.Scan(&a.CharacterID, &a.Name, &a.Class, &a.Level, &a.Rating, &a.Wins, &a.Losses)
		return a, err
	})
}

// ArenaRatingsOf: as pontuações do personagem na temporada, por modo (sem linha = não lutou).
func (s *Store) ArenaRatingsOf(ctx context.Context, season int, characterID int64) (map[game.ArenaMode]ArenaRating, error) {
	rows, err := s.pool.Query(ctx, `SELECT mode, rating, wins, losses FROM arena_ratings
		WHERE season = $1 AND character_id = $2`, season, characterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[game.ArenaMode]ArenaRating{}
	for rows.Next() {
		var mode int
		a := ArenaRating{CharacterID: characterID}
		if err := rows.Scan(&mode, &a.Rating, &a.Wins, &a.Losses); err != nil {
			return nil, err
		}
		out[game.ArenaMode(mode)] = a
	}
	return out, rows.Err()
}
