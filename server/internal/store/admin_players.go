package store

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"erevia/server/internal/game"
)

// Portal de administração, seção Jogadores: busca, ficha, histórico de moderação e a
// transação das ações em personagens offline. Quem decide se o personagem está offline é
// a zona (zone.AdminEdit, que trava o personagem antes); aqui só se grava.

const (
	maxSearchResults = 50
	sheetEvents      = 40
	sheetSessions    = 10
	modHistoryLimit  = 100
)

// Identity é um login ligado à conta (Google, Discord ou dev).
type Identity struct {
	Provider    string    `json:"provider"`
	Subject     string    `json:"subject"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`
}

// PlayerHit é uma linha da busca.
type PlayerHit struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Class      string     `json:"class"`
	Level      int        `json:"level"`
	AccountID  int64      `json:"accountId"`
	GM         bool       `json:"gm"`
	Banned     bool       `json:"banned"`
	MutedUntil *time.Time `json:"mutedUntil"`
	LastSeenAt *time.Time `json:"lastSeenAt"`
	Identities []Identity `json:"identities"`
}

// likeEscape escapa % e _ para o LIKE.
func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// SearchPlayers acha personagens pelo nome (parte dele), pelo id do personagem ou da
// conta, ou pela identidade (id do Google/Discord, "google:123" ou o nome no provedor).
func (s *Store) SearchPlayers(ctx context.Context, q string) ([]PlayerHit, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return []PlayerHit{}, nil
	}
	num, _ := strconv.ParseInt(strings.TrimPrefix(q, "#"), 10, 64)
	like := "%" + likeEscape(q) + "%"
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.name, c.class, c.level, c.account_id, a.gm, a.banned, a.muted_until, c.last_seen_at
		FROM characters c JOIN accounts a ON a.id = c.account_id
		WHERE c.name ILIKE $1
		   OR ($2::bigint > 0 AND (c.id = $2 OR a.id = $2))
		   OR EXISTS (SELECT 1 FROM identities i WHERE i.account_id = a.id
		              AND (i.subject = $3 OR i.provider || ':' || i.subject = $3 OR i.display_name ILIKE $1))
		ORDER BY (lower(c.name) = lower($3)) DESC, c.last_seen_at DESC NULLS LAST, c.id
		LIMIT $4`, like, num, q, maxSearchResults)
	if err != nil {
		return nil, err
	}
	hits, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (PlayerHit, error) {
		var h PlayerHit
		err := r.Scan(&h.ID, &h.Name, &h.Class, &h.Level, &h.AccountID, &h.GM, &h.Banned, &h.MutedUntil, &h.LastSeenAt)
		h.Identities = []Identity{}
		return h, err
	})
	if err != nil || len(hits) == 0 {
		return hits, err
	}
	accounts := make([]int64, 0, len(hits))
	for _, h := range hits {
		accounts = append(accounts, h.AccountID)
	}
	ids, err := s.identities(ctx, accounts)
	if err != nil {
		return nil, err
	}
	for i := range hits {
		if l := ids[hits[i].AccountID]; l != nil {
			hits[i].Identities = l
		}
	}
	return hits, nil
}

func (s *Store) identities(ctx context.Context, accounts []int64) (map[int64][]Identity, error) {
	rows, err := s.pool.Query(ctx, `SELECT account_id, provider, subject, display_name, created_at FROM identities
		WHERE account_id = ANY($1) ORDER BY created_at`, accounts)
	if err != nil {
		return nil, err
	}
	out := map[int64][]Identity{}
	var acc int64
	var i Identity
	_, err = pgx.ForEachRow(rows, []any{&acc, &i.Provider, &i.Subject, &i.DisplayName, &i.CreatedAt}, func() error {
		out[acc] = append(out[acc], i)
		return nil
	})
	return out, err
}

// SheetAccount é a conta dona do personagem.
type SheetAccount struct {
	ID          int64      `json:"id"`
	CreatedAt   time.Time  `json:"createdAt"`
	LastLoginAt time.Time  `json:"lastLoginAt"`
	GM          bool       `json:"gm"`
	Banned      bool       `json:"banned"`
	BanReason   string     `json:"banReason"`
	MutedUntil  *time.Time `json:"mutedUntil"`
	Identities  []Identity `json:"identities"`
	Characters  []AltChar  `json:"characters"` // todos os personagens da conta
}

type AltChar struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Class string `json:"class"`
	Level int    `json:"level"`
}

// SheetClan é o clã do personagem no banco.
type SheetClan struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Rank int    `json:"rank"`
}

// Sheet é a ficha de um personagem como está no banco (a API junta o que a zona sabe).
type Sheet struct {
	Character  Character
	CreatedAt  time.Time
	LastSeenAt *time.Time
	Account    SheetAccount
	Clan       *SheetClan
}

// PlayerSheet carrega a ficha do personagem (sem entregar o kit inicial: só lê).
func (s *Store) PlayerSheet(ctx context.Context, id int64) (Sheet, error) {
	var out Sheet
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+characterCols+` FROM characters WHERE id = $1`, id)
		if err != nil {
			return err
		}
		if out.Character, err = pgx.CollectExactlyOneRow(rows, scanCharacter); err != nil {
			return err
		}
		c := &out.Character
		if err := tx.QueryRow(ctx, `SELECT created_at, last_seen_at FROM characters WHERE id = $1`, id).
			Scan(&out.CreatedAt, &out.LastSeenAt); err != nil {
			return err
		}
		if c.Items, err = loadItems(ctx, tx, c.ID); err != nil {
			return err
		}
		if c.Storage, err = loadStorage(ctx, tx, c.AccountID); err != nil {
			return err
		}
		a := &out.Account
		a.ID = c.AccountID
		if err := tx.QueryRow(ctx, `SELECT created_at, last_login_at, gm, banned, ban_reason, muted_until FROM accounts WHERE id = $1`, c.AccountID).
			Scan(&a.CreatedAt, &a.LastLoginAt, &a.GM, &a.Banned, &a.BanReason, &a.MutedUntil); err != nil {
			return err
		}
		c.GM, c.MutedUntil = a.GM, a.MutedUntil
		rows, err = tx.Query(ctx, `SELECT id, name, class, level FROM characters WHERE account_id = $1 ORDER BY id`, c.AccountID)
		if err != nil {
			return err
		}
		if a.Characters, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (AltChar, error) {
			var x AltChar
			return x, r.Scan(&x.ID, &x.Name, &x.Class, &x.Level)
		}); err != nil {
			return err
		}
		var cl SheetClan
		err = tx.QueryRow(ctx, `SELECT cl.id, cl.name, m.rank FROM clan_members m JOIN clans cl ON cl.id = m.clan_id
			WHERE m.character_id = $1`, id).Scan(&cl.ID, &cl.Name, &cl.Rank)
		switch {
		case err == nil:
			out.Clan = &cl
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Sheet{}, ErrNotFound
	}
	if err != nil {
		return Sheet{}, err
	}
	ids, err := s.identities(ctx, []int64{out.Account.ID})
	if err != nil {
		return Sheet{}, err
	}
	out.Account.Identities = ids[out.Account.ID]
	if out.Account.Identities == nil {
		out.Account.Identities = []Identity{}
	}
	return out, nil
}

// EventRow é um evento de game_events como o portal mostra.
type EventRow struct {
	At     time.Time      `json:"at"`
	Kind   string         `json:"kind"`
	Level  int            `json:"level"`
	X      float64        `json:"x"`
	Z      float64        `json:"z"`
	Ref    string         `json:"ref"`
	Amount int64          `json:"amount"`
	Extra  map[string]any `json:"extra"`
}

// Session é uma sessão de jogo (do evento session start/end).
type Session struct {
	Start   time.Time  `json:"start"`
	End     *time.Time `json:"end"` // nil = em andamento (ou o servidor caiu antes do fim)
	Seconds int64      `json:"seconds"`
}

// CharacterEvents devolve as últimas sessões e os últimos eventos do personagem (sem o FPS
// e as próprias sessões, que já vêm à parte).
func (s *Store) CharacterEvents(ctx context.Context, id int64) ([]Session, []EventRow, error) {
	rows, err := s.pool.Query(ctx, `SELECT at, ref, amount FROM game_events WHERE character_id = $1 AND kind = 'session'
		ORDER BY at DESC, id DESC LIMIT $2`, id, sheetSessions*2+1)
	if err != nil {
		return nil, nil, err
	}
	type sess struct {
		at     time.Time
		ref    string
		amount int64
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (sess, error) {
		var x sess
		return x, r.Scan(&x.at, &x.ref, &x.amount)
	})
	if err != nil {
		return nil, nil, err
	}
	// Do mais novo para o mais velho: um "end" fecha o "start" anterior (o fim traz a duração).
	sessions := []Session{}
	for i := 0; i < len(list) && len(sessions) < sheetSessions; i++ {
		e := list[i]
		switch e.ref {
		case "end":
			end := e.at
			start := end.Add(-time.Duration(e.amount) * time.Second)
			if i+1 < len(list) && list[i+1].ref == "start" {
				start = list[i+1].at
				i++
			}
			sessions = append(sessions, Session{Start: start, End: &end, Seconds: e.amount})
		case "start":
			sessions = append(sessions, Session{Start: e.at, Seconds: int64(time.Since(e.at).Seconds())})
		}
	}
	rows, err = s.pool.Query(ctx, `SELECT at, kind, level, x, z, ref, amount, extra FROM game_events
		WHERE character_id = $1 AND kind NOT IN ('fps', 'session') ORDER BY at DESC, id DESC LIMIT $2`, id, sheetEvents)
	if err != nil {
		return nil, nil, err
	}
	events, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (EventRow, error) {
		var e EventRow
		var x, z float32
		err := r.Scan(&e.At, &e.Kind, &e.Level, &x, &z, &e.Ref, &e.Amount, &e.Extra)
		e.X, e.Z = float64(x), float64(z)
		return e, err
	})
	if events == nil {
		events = []EventRow{}
	}
	return sessions, events, err
}

// AccountTarget é o alvo do gm_log de uma ação na conta sem personagem (busca por conta).
func AccountTarget(accountID int64) string { return "conta:" + strconv.FormatInt(accountID, 10) }

// ModerationHistory: as ações de GM (jogo e portal) contra a conta, do gm_log: o alvo é o
// nome de um personagem da conta ou "conta:<id>".
func (s *Store) ModerationHistory(ctx context.Context, accountID int64) ([]GMLogRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT g.id, g.at, g.gm_account_id, g.gm_name, g.command, g.target, g.detail FROM gm_log g
		WHERE lower(g.target) IN (SELECT lower(name) FROM characters WHERE account_id = $1)
		   OR g.target = $2
		ORDER BY g.at DESC, g.id DESC LIMIT $3`, accountID, AccountTarget(accountID), modHistoryLimit)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (GMLogRow, error) {
		var g GMLogRow
		return g, r.Scan(&g.ID, &g.At, &g.GMAccountID, &g.GMName, &g.Command, &g.Target, &g.Detail)
	})
	if out == nil {
		out = []GMLogRow{}
	}
	return out, err
}

// CharacterOwner devolve a conta e o nome de um personagem.
func (s *Store) CharacterOwner(ctx context.Context, id int64) (int64, string, error) {
	var acc int64
	var name string
	err := s.pool.QueryRow(ctx, `SELECT account_id, name FROM characters WHERE id = $1`, id).Scan(&acc, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", ErrNotFound
	}
	return acc, name, err
}

// SetAccountBanned bane (ou desbane) a conta pelo id.
func (s *Store) SetAccountBanned(ctx context.Context, accountID int64, banned bool, reason string) error {
	if !banned {
		reason = ""
	}
	tag, err := s.pool.Exec(ctx, `UPDATE accounts SET banned = $2, ban_reason = $3 WHERE id = $1`, accountID, banned, reason)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// ---------- Ação em personagem offline ----------

// OfflineChar é o personagem como a transação do portal o vê (e muda).
type OfflineChar struct {
	ID, AccountID int64
	Name, Class   string
	Level         int
	X, Z          float64
	Gold          int64
	Items         []game.ItemRecord
	ItemsChanged  bool // troca as linhas de character_items
}

// EditOfflineCharacter muda um personagem que não está na zona, numa transação com a linha
// travada. fn recebe o estado atual, muda o que precisar e devolve a telemetria a gravar
// junto (game_events, na mesma transação). seq vai para o save_seq (GREATEST): um
// salvamento mais velho que chegasse depois não desfaz a mudança. Erro de fn desfaz tudo.
//
// Só chame com o personagem travado pela zona (zone.AdminEdit com Where = offline): sem a
// trava, um salvamento da zona em andamento poderia sobrescrever a mudança.
func (s *Store) EditOfflineCharacter(ctx context.Context, id, seq int64, fn func(c *OfflineChar) ([]GameEvent, error)) (OfflineChar, error) {
	var c OfflineChar
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var kit bool
		var x, z float32
		err := tx.QueryRow(ctx, `SELECT id, account_id, name, class, level, x, z, gold, kit_given FROM characters WHERE id = $1 FOR UPDATE`, id).
			Scan(&c.ID, &c.AccountID, &c.Name, &c.Class, &c.Level, &x, &z, &c.Gold, &kit)
		if err != nil {
			return err
		}
		c.X, c.Z = float64(x), float64(z)
		if !kit { // como no primeiro login: o kit entra antes, para não ocupar o lugar dele
			ch := Character{ID: c.ID, Class: c.Class}
			if err := giveStarterKit(ctx, tx, &ch, game.StarterKit(c.Class)); err != nil {
				return err
			}
			c.Gold = ch.Gold
		}
		if c.Items, err = loadItems(ctx, tx, c.ID); err != nil {
			return err
		}
		events, err := fn(&c)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE characters SET x = $2, z = $3, gold = $4, save_seq = GREATEST(save_seq, $5::bigint) WHERE id = $1`,
			c.ID, c.X, c.Z, max(c.Gold, 0), seq); err != nil {
			return err
		}
		if c.ItemsChanged {
			if _, err := tx.Exec(ctx, `DELETE FROM character_items WHERE character_id = $1`, c.ID); err != nil {
				return err
			}
			rows := make([][]any, 0, len(c.Items))
			for _, it := range c.Items {
				rows = append(rows, []any{c.ID, placeOf(it), int16(it.Slot), it.ItemID, int32(it.Count), int16(it.Enchant)})
			}
			if _, err := tx.CopyFrom(ctx, pgx.Identifier{"character_items"},
				[]string{"character_id", "place", "slot", "item_id", "count", "enchant"}, pgx.CopyFromRows(rows)); err != nil {
				return err
			}
		}
		if len(events) == 0 {
			return nil
		}
		now := time.Now()
		for i := range events {
			e := &events[i]
			e.At, e.CharacterID, e.AccountID, e.Level, e.X, e.Z = now, c.ID, c.AccountID, c.Level, c.X, c.Z
		}
		return insertEvents(ctx, tx, events)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}
