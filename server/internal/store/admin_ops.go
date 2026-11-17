package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// Anúncios agendados pelo portal (Operação). A zona guarda a fila em memória e solta cada
// um na hora; aqui eles ficam para sobreviver a um reinício (o cmd/zone carrega os que
// ainda não saíram).

// ScheduledAnnouncement é uma linha de scheduled_announcements.
type ScheduledAnnouncement struct {
	ID     int64
	At     time.Time
	Title  string
	Text   string
	ByName string
}

// AddScheduledAnnouncement grava um anúncio agendado e devolve o id.
func (s *Store) AddScheduledAnnouncement(ctx context.Context, at time.Time, title, text string, by int64, byName string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `INSERT INTO scheduled_announcements (at, title, text, created_by, created_by_name)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`, at, title, text, by, byName).Scan(&id)
	return id, err
}

// CancelScheduledAnnouncement marca o anúncio como cancelado (false = não havia um pendente).
func (s *Store) CancelScheduledAnnouncement(ctx context.Context, id int64) (bool, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE scheduled_announcements SET cancelled_at = now() WHERE id = $1 AND cancelled_at IS NULL`, id)
	return tag.RowsAffected() > 0, err
}

// PendingAnnouncements: os agendados que ainda não saíram nem foram cancelados.
func (s *Store) PendingAnnouncements(ctx context.Context) ([]ScheduledAnnouncement, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, at, title, text, created_by_name FROM scheduled_announcements
		WHERE cancelled_at IS NULL AND at > now() ORDER BY at, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (ScheduledAnnouncement, error) {
		var a ScheduledAnnouncement
		return a, r.Scan(&a.ID, &a.At, &a.Title, &a.Text, &a.ByName)
	})
}
