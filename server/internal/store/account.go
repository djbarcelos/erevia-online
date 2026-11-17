package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DeletedAccount diz o que sumiu ao apagar uma conta (a zona usa para derrubar quem
// estiver online e acertar os clãs em memória do mesmo jeito que o banco ficou).
type DeletedAccount struct {
	AccountID  int64
	Characters []int64
	// Clans: clãs que eram liderados por um personagem da conta.
	Clans []ClanSuccession
}

// ClanSuccession: o novo líder de um clã cujo líder foi apagado (LeaderID 0 = o clã
// ficou vazio e foi desfeito).
type ClanSuccession struct {
	ClanID   int64
	LeaderID int64
}

// DeleteAccount apaga a conta e tudo o que é dela, numa transação: identidades
// (Google/Discord), sessões, personagens, itens, armazém e participação em clãs (tudo em
// cascata). Antes, o clã liderado por um personagem da conta passa ao membro de cargo
// mais alto (o mais antigo no clã, no empate) ou é desfeito se não sobrar ninguém, e o
// nome do personagem sai do registro dos raid bosses.
//
// Um salvamento atrasado da zona depois disso não recria nada: SaveStates só atualiza
// linhas que existem. Conta inexistente devolve ErrNotFound.
func (s *Store) DeleteAccount(ctx context.Context, accountID int64) (DeletedAccount, error) {
	var out DeletedAccount
	var err error
	// Um salvamento ou operação de clã ao mesmo tempo pode travar em ordem diferente
	// (deadlock): o Postgres aborta uma das duas, e esta tenta de novo.
	for attempt := 1; attempt <= 3; attempt++ {
		out, err = s.deleteAccount(ctx, accountID)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || (pgErr.Code != "40P01" && pgErr.Code != "40001") {
			break
		}
	}
	return out, err
}

func (s *Store) deleteAccount(ctx context.Context, accountID int64) (DeletedAccount, error) {
	out := DeletedAccount{AccountID: accountID}
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// Trava os personagens primeiro, na mesma ordem do SaveStates (personagens, depois
		// a conta). Enquanto isso, ninguém cria clã nem passa a liderança para eles.
		rows, err := tx.Query(ctx, `SELECT id, lower(name) FROM characters WHERE account_id = $1 ORDER BY id FOR UPDATE`, accountID)
		if err != nil {
			return err
		}
		var names []string
		var id int64
		var name string
		if _, err := pgx.ForEachRow(rows, []any{&id, &name}, func() error {
			out.Characters = append(out.Characters, id)
			names = append(names, name)
			return nil
		}); err != nil {
			return err
		}

		if len(out.Characters) > 0 {
			if err := passClans(ctx, tx, out.Characters, &out.Clans); err != nil {
				return err
			}
			// O registro dos raid bosses guarda o nome de quem mais causou dano.
			if _, err := tx.Exec(ctx, `UPDATE world_bosses SET top_name = '' WHERE lower(top_name) = ANY($1)`, names); err != nil {
				return err
			}
			// Cartas enviadas ficam com quem recebeu (from_id vira NULL pela chave), mas sem
			// o nome do personagem apagado.
			if _, err := tx.Exec(ctx, `UPDATE mail SET from_name = 'Personagem apagado' WHERE from_id = ANY($1)`, out.Characters); err != nil {
				return err
			}
		}

		// Erros do cliente (client_errors, sem chave estrangeira): ficam, mas sem dono.
		if err := anonymizeClientErrors(ctx, tx, accountID); err != nil {
			return err
		}

		tag, err := tx.Exec(ctx, `DELETE FROM accounts WHERE id = $1`, accountID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		// Telemetria (game_events) não tem chave estrangeira: os eventos ficam, para as
		// estatísticas agregadas, mas sem a conta e os personagens. Roda DEPOIS do DELETE:
		// um lote que InsertEvents estava gravando trava a linha da conta (FOR KEY SHARE),
		// então o DELETE espera esse lote terminar, e este UPDATE (com um snapshot novo)
		// também pega os eventos dele.
		if _, err := tx.Exec(ctx,
			`UPDATE game_events SET account_id = NULL, character_id = NULL
			 WHERE account_id = $1 OR character_id = ANY($2)`, accountID, out.Characters); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return DeletedAccount{}, err
	}
	return out, nil
}

// passClans passa a liderança dos clãs liderados por chars ao melhor membro que fica
// (cargo mais alto, depois quem entrou primeiro) ou desfaz o clã que ficaria vazio.
func passClans(ctx context.Context, tx pgx.Tx, chars []int64, out *[]ClanSuccession) error {
	rows, err := tx.Query(ctx, `SELECT id FROM clans WHERE leader_id = ANY($1) ORDER BY id FOR UPDATE`, chars)
	if err != nil {
		return err
	}
	clans, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return err
	}
	for _, clanID := range clans {
		var next int64
		err := tx.QueryRow(ctx, `SELECT character_id FROM clan_members
			WHERE clan_id = $1 AND character_id <> ALL($2)
			ORDER BY rank DESC, joined_at, character_id LIMIT 1`, clanID, chars).Scan(&next)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if _, err := tx.Exec(ctx, `DELETE FROM clans WHERE id = $1`, clanID); err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			if _, err := tx.Exec(ctx, `UPDATE clan_members SET rank = $2 WHERE character_id = $1`, next, ClanRankLeader); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE clans SET leader_id = $2 WHERE id = $1`, clanID, next); err != nil {
				return err
			}
		}
		*out = append(*out, ClanSuccession{ClanID: clanID, LeaderID: next})
	}
	return nil
}
