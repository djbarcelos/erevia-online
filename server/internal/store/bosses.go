package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// Boss é o renascimento salvo de um raid boss.
type Boss struct {
	ID        string
	RespawnAt time.Time
	KilledAt  time.Time // zero = nunca morreu
	TopName   string
}

// LoadBosses lê o renascimento de todos os raid bosses.
func (s *Store) LoadBosses(ctx context.Context) ([]Boss, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, respawn_at, killed_at, top_name FROM world_bosses ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Boss, error) {
		var b Boss
		var killed *time.Time
		err := row.Scan(&b.ID, &b.RespawnAt, &killed, &b.TopName)
		if killed != nil {
			b.KilledAt = *killed
		}
		return b, err
	})
}

// SaveBoss grava (ou troca) o renascimento de um raid boss.
func (s *Store) SaveBoss(ctx context.Context, b Boss) error {
	var killed *time.Time
	if !b.KilledAt.IsZero() {
		killed = &b.KilledAt
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO world_bosses (id, respawn_at, killed_at, top_name) VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET respawn_at = EXCLUDED.respawn_at, killed_at = EXCLUDED.killed_at, top_name = EXCLUDED.top_name`,
		b.ID, b.RespawnAt, killed, b.TopName)
	return err
}
