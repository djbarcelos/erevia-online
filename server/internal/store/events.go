package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
)

// GameEvent é uma linha de game_events (telemetria).
type GameEvent struct {
	At          time.Time
	Kind        string
	CharacterID int64
	AccountID   int64
	Level       int
	X, Z        float64
	Ref         string
	Amount      int64
	Extra       map[string]any
}

// InsertEvents grava um lote de eventos de uma vez (COPY).
func (s *Store) InsertEvents(ctx context.Context, events []GameEvent) error {
	rows := make([][]any, 0, len(events))
	for _, e := range events {
		var extra []byte
		if len(e.Extra) > 0 {
			var err error
			if extra, err = json.Marshal(e.Extra); err != nil {
				return err
			}
		}
		var char, acc *int64
		if e.CharacterID > 0 {
			char = &e.CharacterID
		}
		if e.AccountID > 0 {
			acc = &e.AccountID
		}
		rows = append(rows, []any{e.At, e.Kind, char, acc, int32(e.Level), float32(e.X), float32(e.Z), e.Ref, e.Amount, extra})
	}
	_, err := s.pool.CopyFrom(ctx, pgx.Identifier{"game_events"},
		[]string{"at", "kind", "character_id", "account_id", "level", "x", "z", "ref", "amount", "extra"}, pgx.CopyFromRows(rows))
	return err
}

// QueryRowForTest roda uma consulta de uma linha (testes das views do painel).
func (s *Store) QueryRowForTest(ctx context.Context, sql string, dest ...any) error {
	return s.pool.QueryRow(ctx, sql).Scan(dest...)
}
