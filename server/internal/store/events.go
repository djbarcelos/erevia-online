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
//
// A zona grava em lotes a cada poucos segundos, então um lote pode chegar depois que a
// conta foi apagada (ex.: o fim de sessão de quem foi derrubado pela exclusão). Os eventos
// de contas que não existem mais entram sem a conta e o personagem. As contas do lote ficam
// travadas (FOR KEY SHARE) até o COPY terminar: o DeleteAccount concorrente espera e depois
// anonimiza o que este lote gravou.
func (s *Store) InsertEvents(ctx context.Context, events []GameEvent) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return insertEvents(ctx, tx, events)
	})
}

func insertEvents(ctx context.Context, tx pgx.Tx, events []GameEvent) error {
	var accounts []int64
	seen := map[int64]bool{}
	for _, e := range events {
		if e.AccountID > 0 && !seen[e.AccountID] {
			seen[e.AccountID] = true
			accounts = append(accounts, e.AccountID)
		}
	}
	alive := map[int64]bool{}
	if len(accounts) > 0 {
		rows, err := tx.Query(ctx, `SELECT id FROM accounts WHERE id = ANY($1) ORDER BY id FOR KEY SHARE`, accounts)
		if err != nil {
			return err
		}
		var id int64
		if _, err := pgx.ForEachRow(rows, []any{&id}, func() error {
			alive[id] = true
			return nil
		}); err != nil {
			return err
		}
	}
	rows := make([][]any, 0, len(events))
	for _, e := range events {
		if e.AccountID > 0 && !alive[e.AccountID] {
			e.AccountID, e.CharacterID = 0, 0
		}
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
	_, err := tx.CopyFrom(ctx, pgx.Identifier{"game_events"},
		[]string{"at", "kind", "character_id", "account_id", "level", "x", "z", "ref", "amount", "extra"}, pgx.CopyFromRows(rows))
	return err
}

// QueryRowForTest roda uma consulta de uma linha (testes das views do painel).
func (s *Store) QueryRowForTest(ctx context.Context, sql string, dest ...any) error {
	return s.pool.QueryRow(ctx, sql).Scan(dest...)
}
