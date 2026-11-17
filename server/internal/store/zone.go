package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// CharacterZone: o mapa gravado do personagem (characters.zone, migração 00050). O portal
// usa para mandar a ação sobre um personagem offline à zona do próximo login dele.
func (s *Store) CharacterZone(ctx context.Context, id int64) (string, error) {
	var zone string
	err := s.pool.QueryRow(ctx, `SELECT zone FROM characters WHERE id = $1`, id).Scan(&zone)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return zone, err
}
