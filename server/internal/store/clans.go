package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const clansNameIdxKey = "clans_name_lower_idx"

// Cargos no clã (iguais ao CHECK da tabela e ao enum do protocolo).
const (
	ClanRankMember  = 0
	ClanRankOfficer = 1
	ClanRankLeader  = 2
)

// Clan é um clã com a lista de membros (online ou não).
type Clan struct {
	ID       int64
	Name     string
	LeaderID int64
	Notice   string
	Members  []ClanMember
}

type ClanMember struct {
	CharacterID int64
	Name        string
	Class       string
	Level       int
	Rank        int
}

// LoadClans lê todos os clãs com os membros (a zona guarda tudo em memória).
func (s *Store) LoadClans(ctx context.Context) ([]Clan, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, leader_id, notice FROM clans ORDER BY id`)
	if err != nil {
		return nil, err
	}
	clans, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Clan, error) {
		var c Clan
		err := row.Scan(&c.ID, &c.Name, &c.LeaderID, &c.Notice)
		return c, err
	})
	if err != nil || len(clans) == 0 {
		return clans, err
	}
	byID := make(map[int64]int, len(clans))
	for i, c := range clans {
		byID[c.ID] = i
	}
	rows, err = s.pool.Query(ctx, `SELECT m.clan_id, m.character_id, c.name, c.class, c.level, m.rank
		FROM clan_members m JOIN characters c ON c.id = m.character_id ORDER BY m.clan_id, m.rank DESC, c.name`)
	if err != nil {
		return nil, err
	}
	var clanID int64
	var m ClanMember
	var rank int16
	_, err = pgx.ForEachRow(rows, []any{&clanID, &m.CharacterID, &m.Name, &m.Class, &m.Level, &rank}, func() error {
		if i, ok := byID[clanID]; ok {
			m.Rank = int(rank)
			clans[i].Members = append(clans[i].Members, m)
		}
		return nil
	})
	return clans, err
}

// CreateClan cria o clã com o fundador como líder. O nome já foi validado.
func (s *Store) CreateClan(ctx context.Context, name string, leaderID int64) (int64, error) {
	var id int64
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO clans (name, leader_id) VALUES ($1, $2) RETURNING id`,
			name, leaderID).Scan(&id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO clan_members (character_id, clan_id, rank) VALUES ($1, $2, $3)`,
			leaderID, id, ClanRankLeader)
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == clansNameIdxKey {
		return 0, ErrNameTaken
	}
	return id, err
}

// ClanOpKind diz o que uma ClanOp muda.
type ClanOpKind int

const (
	ClanOpJoin    ClanOpKind = iota + 1 // CharacterID entra no ClanID com Rank
	ClanOpLeave                         // CharacterID sai (LeftAt marca a espera)
	ClanOpRank                          // CharacterID muda para Rank
	ClanOpLeader                        // CharacterID vira líder do ClanID (o antigo vira oficial)
	ClanOpNotice                        // aviso do ClanID
	ClanOpDisband                       // apaga o ClanID (sem espera para os membros)
)

// ClanOp é uma mudança num clã que a zona já aplicou em memória.
type ClanOp struct {
	Kind        ClanOpKind
	ClanID      int64
	CharacterID int64
	Rank        int
	Text        string
	LeftAt      time.Time
}

// ApplyClanOp grava a mudança. É idempotente: repetir a mesma operação não estraga nada.
func (s *Store) ApplyClanOp(ctx context.Context, op ClanOp) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		switch op.Kind {
		case ClanOpJoin:
			_, err = tx.Exec(ctx, `INSERT INTO clan_members (character_id, clan_id, rank) VALUES ($1, $2, $3)
				ON CONFLICT (character_id) DO UPDATE SET clan_id = EXCLUDED.clan_id, rank = EXCLUDED.rank, joined_at = now()`,
				op.CharacterID, op.ClanID, op.Rank)
		case ClanOpLeave:
			if _, err = tx.Exec(ctx, `DELETE FROM clan_members WHERE character_id = $1`, op.CharacterID); err == nil {
				_, err = tx.Exec(ctx, `UPDATE characters SET clan_left_at = $2 WHERE id = $1`, op.CharacterID, op.LeftAt)
			}
		case ClanOpRank:
			_, err = tx.Exec(ctx, `UPDATE clan_members SET rank = $2 WHERE character_id = $1`, op.CharacterID, op.Rank)
		case ClanOpLeader:
			if _, err = tx.Exec(ctx, `UPDATE clan_members SET rank = $2 WHERE clan_id = $1 AND rank = $3`,
				op.ClanID, ClanRankOfficer, ClanRankLeader); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE clan_members SET rank = $2 WHERE character_id = $1`,
				op.CharacterID, ClanRankLeader); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE clans SET leader_id = $2 WHERE id = $1`, op.ClanID, op.CharacterID)
		case ClanOpNotice:
			_, err = tx.Exec(ctx, `UPDATE clans SET notice = $2 WHERE id = $1`, op.ClanID, op.Text)
		case ClanOpDisband:
			_, err = tx.Exec(ctx, `DELETE FROM clans WHERE id = $1`, op.ClanID) // membros saem junto (cascade)
		default:
			return errors.New("unknown clan op")
		}
		return err
	})
}
