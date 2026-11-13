package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// GMBySession devolve a conta da sessão só se ela for GM (sessão válida, conta não banida
// e accounts.gm = true). Qualquer outro caso é ErrNotFound: o portal de administração
// responde igual para quem não está logado e para quem não é GM.
func (s *Store) GMBySession(ctx context.Context, token string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		SELECT a.id FROM sessions s JOIN accounts a ON a.id = s.account_id
		WHERE s.token_hash = $1 AND s.expires_at > now() AND NOT a.banned AND a.gm`,
		hashToken(token)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

// AdminSummary é o resumo da página inicial do portal.
type AdminSummary struct {
	Accounts      int64 `json:"accounts"`
	Accounts24h   int64 `json:"accounts24h"`
	Characters    int64 `json:"characters"`
	Characters24h int64 `json:"characters24h"`
	Bugs24h       int64 `json:"bugs24h"`
	BugsOpen      int64 `json:"bugsOpen"`
}

// AdminSummary conta contas, personagens e relatos de /bug (total e últimas 24 h).
func (s *Store) AdminSummary(ctx context.Context) (AdminSummary, error) {
	var out AdminSummary
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM accounts),
			(SELECT count(*) FROM accounts WHERE created_at > now() - interval '24 hours'),
			(SELECT count(*) FROM characters),
			(SELECT count(*) FROM characters WHERE created_at > now() - interval '24 hours'),
			(SELECT count(*) FROM bug_reports WHERE at > now() - interval '24 hours'),
			(SELECT count(*) FROM bug_reports WHERE NOT resolved)`).
		Scan(&out.Accounts, &out.Accounts24h, &out.Characters, &out.Characters24h, &out.Bugs24h, &out.BugsOpen)
	return out, err
}

// SetGM marca (ou desmarca) a conta como GM. No jogo, vale a partir do próximo login; no
// portal, na próxima requisição.
func (s *Store) SetGM(ctx context.Context, accountID int64, gm bool) error {
	tag, err := s.pool.Exec(ctx, `UPDATE accounts SET gm = $2 WHERE id = $1`, accountID, gm)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// GMLogRow é uma linha do gm_log como o portal mostra.
type GMLogRow struct {
	ID          int64     `json:"id"`
	At          time.Time `json:"at"`
	GMAccountID int64     `json:"gmAccountId"`
	GMName      string    `json:"gmName"`
	Command     string    `json:"command"`
	Target      string    `json:"target"`
	Detail      string    `json:"detail"`
}

// RecentGMLog devolve as últimas ações de GM (do jogo e do portal), da mais nova à mais velha.
func (s *Store) RecentGMLog(ctx context.Context, limit int) ([]GMLogRow, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, at, gm_account_id, gm_name, command, target, detail FROM gm_log ORDER BY at DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (GMLogRow, error) {
		var g GMLogRow
		err := r.Scan(&g.ID, &g.At, &g.GMAccountID, &g.GMName, &g.Command, &g.Target, &g.Detail)
		return g, err
	})
}
