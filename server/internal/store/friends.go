package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	// ErrSelf: o personagem tentou adicionar a si mesmo ou mandar carta para si.
	ErrSelf = errors.New("cannot target yourself")
	// ErrFriendLimit: a lista de amigos está cheia.
	ErrFriendLimit = errors.New("friend list full")
	// ErrMailboxFull: o destinatário já guarda o máximo de cartas.
	ErrMailboxFull = errors.New("mailbox full")
	// ErrMailRate: o remetente mandou cartas demais na última hora.
	ErrMailRate = errors.New("too many letters")
)

// Friend é um personagem na lista de amigos de outro.
type Friend struct {
	ID    int64
	Name  string
	Class string
	Level int
}

// Friends lista quem o personagem segue, pelo nome.
func (s *Store) Friends(ctx context.Context, characterID int64) ([]Friend, error) {
	rows, err := s.pool.Query(ctx, `SELECT c.id, c.name, c.class, c.level
		FROM friends f JOIN characters c ON c.id = f.friend_id
		WHERE f.character_id = $1 ORDER BY lower(c.name)`, characterID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Friend, error) {
		var f Friend
		err := row.Scan(&f.ID, &f.Name, &f.Class, &f.Level)
		return f, err
	})
}

// characterByName acha um personagem pelo nome, sem diferenciar maiúsculas.
func characterByName(ctx context.Context, tx pgx.Tx, name string) (Friend, error) {
	var f Friend
	err := tx.QueryRow(ctx, `SELECT id, name, class, level FROM characters WHERE lower(name) = lower($1)`, name).
		Scan(&f.ID, &f.Name, &f.Class, &f.Level)
	if errors.Is(err, pgx.ErrNoRows) {
		return f, ErrNotFound
	}
	return f, err
}

// AddFriend põe o personagem "name" na lista de characterID (até max). Repetir não é erro:
// devolve o amigo que já estava lá.
func (s *Store) AddFriend(ctx context.Context, characterID int64, name string, max int) (Friend, error) {
	var out Friend
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		f, err := characterByName(ctx, tx, name)
		if err != nil {
			return err
		}
		if f.ID == characterID {
			return ErrSelf
		}
		out = f
		// Trava o dono da lista: duas inclusões ao mesmo tempo não passam do limite.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM characters WHERE id = $1 FOR UPDATE`, characterID); err != nil {
			return err
		}
		var exists bool
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*), coalesce(bool_or(friend_id = $2), false) FROM friends WHERE character_id = $1`,
			characterID, f.ID).Scan(&count, &exists); err != nil {
			return err
		}
		if exists {
			return nil
		}
		if count >= max {
			return ErrFriendLimit
		}
		_, err = tx.Exec(ctx, `INSERT INTO friends (character_id, friend_id) VALUES ($1, $2)`, characterID, f.ID)
		return err
	})
	return out, err
}

// RemoveFriend tira friendID da lista de characterID (não estar lá não é erro).
func (s *Store) RemoveFriend(ctx context.Context, characterID, friendID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM friends WHERE character_id = $1 AND friend_id = $2`, characterID, friendID)
	return err
}

// ---------- Correio ----------

// MailOut é uma carta a enviar, com os limites contra spam.
type MailOut struct {
	FromID   int64
	FromName string
	ToName   string
	Subject  string
	Body     string
	At       time.Time // hora do envio (relógio da zona)
	PerHour  int       // cartas por hora por remetente
	MaxBox   int       // cartas guardadas por destinatário
}

// MailSent diz para quem a carta foi e como ficou a caixa dele.
type MailSent struct {
	ToID   int64
	ToName string
	Unread int
	Total  int
}

// Mail é uma carta na caixa de entrada.
type Mail struct {
	ID       int64
	FromName string
	Subject  string
	Body     string
	SentAt   time.Time
	Read     bool
}

// SendMail entrega uma carta (o destinatário pode estar offline). Recusa com ErrNotFound,
// ErrSelf, ErrMailRate (remetente) ou ErrMailboxFull (destinatário).
func (s *Store) SendMail(ctx context.Context, m MailOut) (MailSent, error) {
	var out MailSent
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		to, err := characterByName(ctx, tx, m.ToName)
		if err != nil {
			return err
		}
		if to.ID == m.FromID {
			return ErrSelf
		}
		// Trava a caixa do destinatário: duas cartas ao mesmo tempo não passam do limite.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM characters WHERE id = $1 FOR UPDATE`, to.ID); err != nil {
			return err
		}
		hourAgo := m.At.Add(-time.Hour)
		if _, err := tx.Exec(ctx, `DELETE FROM mail WHERE from_id = $1 AND deleted_at IS NOT NULL AND sent_at < $2`,
			m.FromID, hourAgo); err != nil {
			return err
		}
		var sent int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM mail WHERE from_id = $1 AND sent_at > $2`, m.FromID, hourAgo).
			Scan(&sent); err != nil {
			return err
		}
		if sent >= m.PerHour {
			return ErrMailRate
		}
		var stored int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM mail WHERE to_id = $1 AND deleted_at IS NULL`, to.ID).
			Scan(&stored); err != nil {
			return err
		}
		if stored >= m.MaxBox {
			return ErrMailboxFull
		}
		if _, err := tx.Exec(ctx, `INSERT INTO mail (to_id, from_id, from_name, subject, body, sent_at) VALUES ($1, $2, $3, $4, $5, $6)`,
			to.ID, m.FromID, m.FromName, m.Subject, m.Body, m.At); err != nil {
			return err
		}
		out = MailSent{ToID: to.ID, ToName: to.Name}
		return tx.QueryRow(ctx, mailSummarySQL, to.ID).Scan(&out.Unread, &out.Total)
	})
	return out, err
}

const mailSummarySQL = `SELECT count(*) FILTER (WHERE NOT read), count(*) FROM mail WHERE to_id = $1 AND deleted_at IS NULL`

// MailSummary conta as cartas não lidas e o total guardado.
func (s *Store) MailSummary(ctx context.Context, characterID int64) (unread, total int, err error) {
	err = s.pool.QueryRow(ctx, mailSummarySQL, characterID).Scan(&unread, &total)
	return unread, total, err
}

// Inbox lista as cartas guardadas, da mais nova para a mais velha.
func (s *Store) Inbox(ctx context.Context, characterID int64) ([]Mail, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, from_name, subject, body, sent_at, read FROM mail
		WHERE to_id = $1 AND deleted_at IS NULL ORDER BY id DESC`, characterID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Mail, error) {
		var m Mail
		err := row.Scan(&m.ID, &m.FromName, &m.Subject, &m.Body, &m.SentAt, &m.Read)
		return m, err
	})
}

// MarkMailRead marca a carta como lida (só se for do personagem).
func (s *Store) MarkMailRead(ctx context.Context, characterID, mailID int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE mail SET read = true WHERE id = $1 AND to_id = $2`, mailID, characterID)
	return err
}

// DeleteMail tira a carta da caixa (só se for do personagem). A linha fica escondida até
// sair da janela de uma hora do limite de envio; as antigas do mesmo dono somem de vez.
func (s *Store) DeleteMail(ctx context.Context, characterID, mailID int64, at time.Time) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE mail SET deleted_at = $3 WHERE id = $1 AND to_id = $2 AND deleted_at IS NULL`,
			mailID, characterID, at); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM mail WHERE to_id = $1 AND deleted_at IS NOT NULL AND sent_at < $2`,
			characterID, at.Add(-time.Hour))
		return err
	})
}
