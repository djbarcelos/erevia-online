package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// ServerEvent é um evento ligado por GM com prazo (ex.: XP em dobro até Until).
type ServerEvent struct {
	ID    string
	Until time.Time
}

// LoadServerEvents lê os eventos de GM salvos.
func (s *Store) LoadServerEvents(ctx context.Context) ([]ServerEvent, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, until FROM server_events ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (ServerEvent, error) {
		var e ServerEvent
		err := row.Scan(&e.ID, &e.Until)
		return e, err
	})
}

// SaveServerEvent grava (ou troca) o prazo de um evento.
func (s *Store) SaveServerEvent(ctx context.Context, e ServerEvent) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO server_events (id, until) VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET until = EXCLUDED.until, updated_at = now()`, e.ID, e.Until)
	return err
}
