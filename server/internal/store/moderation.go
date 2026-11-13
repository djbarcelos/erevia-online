package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// GMEntry é uma linha do registro de ações de GM.
type GMEntry struct {
	GMAccountID int64
	GMName      string
	Command     string
	Target      string
	Detail      string
}

// BugReport é um relato feito com /bug.
type BugReport struct {
	AccountID     int64
	CharacterID   int64
	CharacterName string
	Level         int
	X, Z          float64
	Version       string
	UserAgent     string
	Text          string
}

// accountOfCharacter acha a conta dona do personagem (nome sem diferenciar maiúsculas).
func (s *Store) accountOfCharacter(ctx context.Context, name string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `SELECT account_id FROM characters WHERE lower(name) = lower($1)`, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

// SetBanned bane (ou desbane) a conta dona do personagem e devolve o id da conta.
// Banida, a conta não autentica mais (AccountBySession) nem com sessões antigas.
func (s *Store) SetBanned(ctx context.Context, characterName string, banned bool, reason string) (int64, error) {
	id, err := s.accountOfCharacter(ctx, characterName)
	if err != nil {
		return 0, err
	}
	if !banned {
		reason = ""
	}
	_, err = s.pool.Exec(ctx, `UPDATE accounts SET banned = $2, ban_reason = $3 WHERE id = $1`, id, banned, reason)
	return id, err
}

// SetMute silencia a conta até "until" (zero = tira o silêncio).
func (s *Store) SetMute(ctx context.Context, accountID int64, until time.Time) error {
	var v *time.Time
	if !until.IsZero() {
		v = &until
	}
	_, err := s.pool.Exec(ctx, `UPDATE accounts SET muted_until = $2 WHERE id = $1`, accountID, v)
	return err
}

// LogGM grava uma ação de GM.
func (s *Store) LogGM(ctx context.Context, e GMEntry) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO gm_log (gm_account_id, gm_name, command, target, detail) VALUES ($1, $2, $3, $4, $5)`,
		e.GMAccountID, e.GMName, e.Command, e.Target, e.Detail)
	return err
}

// SaveBugReport grava um relato de /bug.
func (s *Store) SaveBugReport(ctx context.Context, b BugReport) error {
	var charID *int64
	if b.CharacterID > 0 {
		charID = &b.CharacterID
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO bug_reports (account_id, character_id, character_name, level, x, z, version, user_agent, text)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		b.AccountID, charID, b.CharacterName, b.Level, float32(b.X), float32(b.Z), b.Version, b.UserAgent, b.Text)
	return err
}
