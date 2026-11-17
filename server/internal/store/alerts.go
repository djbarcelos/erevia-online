package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Consultas dos alertas no Discord (server/internal/alerts). O estado dos avisos fica em
// alert_state (migração 00029); o resto é leitura das tabelas que já existem.

// AlertMark é uma linha de alert_state.
type AlertMark struct {
	Kind, Key  string
	SentAt     time.Time
	Pos        int64
	Suppressed int
	Info       map[string]any
}

// AlertEvent é um evento de game_events lido pelos alertas. Nas consultas agrupadas,
// Amount é a soma do grupo, Count o número de linhas e o resto vem da ocorrência
// indicada em cada consulta.
type AlertEvent struct {
	At          time.Time
	Kind, Ref   string
	Amount      int64
	Count       int64
	CharacterID int64
	Level       int
	X, Z        float64
	Region      string
	Extra       map[string]any
}

// AlertClientError é a primeira linha (versão) de um erro do cliente.
type AlertClientError struct {
	Hash, Kind, Message, Stack, Version, Region, UserAgent string
	Count                                                  int64
	FirstSeen, LastSeen                                    time.Time
}

// ClientErrorTotal soma um erro do cliente em todas as versões.
type ClientErrorTotal struct {
	Hash, Kind, Message, Version string // tipo, mensagem e versão da linha mais recente
	Total                        int64
	FirstSeen, LastSeen          time.Time
}

// AlertBug é um relato de /bug ainda não avisado.
type AlertBug struct {
	ID, AccountID, CharacterID int64
	At                         time.Time
	Name                       string
	Level                      int
	X, Z                       float64
	Region, Version, UserAgent string
	Text                       string
}

// AlertBootstrap prepara os alertas na primeira vez (uma só, para sempre): tudo o que já
// existe conta como visto (os erros de antes não viram "erro novo") e os relatos de antes
// não são avisados. Devolve true se foi agora.
func (s *Store) AlertBootstrap(ctx context.Context, at time.Time) (bool, error) {
	first := false
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `INSERT INTO alert_state (kind, key, sent_at, pos)
			VALUES ('cursor', 'bootstrap', $1, (SELECT coalesce(max(id), 0) FROM bug_reports))
			ON CONFLICT DO NOTHING`, at)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		first = true
		if _, err := tx.Exec(ctx, `INSERT INTO alert_state (kind, key, sent_at)
			SELECT DISTINCT 'seen', 'client:' || hash, $1::timestamptz FROM client_errors
			ON CONFLICT DO NOTHING`, at); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO alert_state (kind, key, sent_at)
			SELECT DISTINCT 'seen', 'server:' || ref, $1::timestamptz FROM game_events WHERE kind = 'server_error'
			ON CONFLICT DO NOTHING`, at)
		return err
	})
	return first, err
}

const alertMarkCols = `kind, key, sent_at, pos, suppressed, info`

func scanAlertMark(row pgx.Row) (AlertMark, error) {
	var m AlertMark
	err := row.Scan(&m.Kind, &m.Key, &m.SentAt, &m.Pos, &m.Suppressed, &m.Info)
	return m, err
}

// AlertMark lê uma linha de alert_state (ok=false se não existe).
func (s *Store) AlertMark(ctx context.Context, kind, key string) (AlertMark, bool, error) {
	m, err := scanAlertMark(s.pool.QueryRow(ctx,
		`SELECT `+alertMarkCols+` FROM alert_state WHERE kind = $1 AND key = $2`, kind, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return AlertMark{}, false, nil
	}
	return m, err == nil, err
}

// LatestAlertMark: a linha mais recente (sent_at) do tipo, fora a chave except.
func (s *Store) LatestAlertMark(ctx context.Context, kind, except string) (AlertMark, bool, error) {
	m, err := scanAlertMark(s.pool.QueryRow(ctx,
		`SELECT `+alertMarkCols+` FROM alert_state WHERE kind = $1 AND key <> $2 ORDER BY sent_at DESC LIMIT 1`, kind, except))
	if errors.Is(err, pgx.ErrNoRows) {
		return AlertMark{}, false, nil
	}
	return m, err == nil, err
}

// SaveAlertMark grava (ou substitui) a linha inteira.
func (s *Store) SaveAlertMark(ctx context.Context, m AlertMark) error {
	var info []byte
	if m.Info != nil {
		var err error
		if info, err = json.Marshal(m.Info); err != nil {
			return err
		}
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO alert_state (kind, key, sent_at, pos, suppressed, info)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)
		ON CONFLICT (kind, key) DO UPDATE SET sent_at = EXCLUDED.sent_at, pos = EXCLUDED.pos,
			suppressed = EXCLUDED.suppressed, info = EXCLUDED.info`,
		m.Kind, m.Key, m.SentAt, m.Pos, m.Suppressed, info)
	return err
}

// SuppressAlert conta um aviso segurado pelo limite (a linha já existe).
func (s *Store) SuppressAlert(ctx context.Context, kind, key string) error {
	_, err := s.pool.Exec(ctx, `UPDATE alert_state SET suppressed = suppressed + 1 WHERE kind = $1 AND key = $2`, kind, key)
	return err
}

// SetAlertPos muda só o pos (o último sinal de uma subida).
func (s *Store) SetAlertPos(ctx context.Context, kind, key string, pos int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE alert_state SET pos = $3 WHERE kind = $1 AND key = $2`, kind, key, pos)
	return err
}

// MarkAlertSeen marca chaves como vistas (kind seen), sem mexer nas que já estão.
func (s *Store) MarkAlertSeen(ctx context.Context, keys []string, at time.Time) error {
	if len(keys) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO alert_state (kind, key, sent_at)
		SELECT 'seen', k, $2::timestamptz FROM unnest($1::text[]) AS k ON CONFLICT DO NOTHING`, keys, at)
	return err
}

// PruneAlertMarks apaga os avisos com prazo (pico, invariante do dia, tick lento, subidas,
// avisos de subida) mais velhos que before. seen, bug e cursor ficam.
func (s *Store) PruneAlertMarks(ctx context.Context, before time.Time) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM alert_state
		WHERE kind IN ('spike', 'invariant', 'slow_tick', 'boot', 'boot_alert') AND sent_at < $1`, before)
	return err
}

// UnseenClientErrors: erros do cliente com atividade desde since cujo hash ainda não foi
// visto (em versão nenhuma). Uma linha por hash: a primeira versão em que apareceu.
func (s *Store) UnseenClientErrors(ctx context.Context, since time.Time, limit int) ([]AlertClientError, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT ON (c.hash) c.hash, c.kind, c.message, c.stack, c.version,
			c.region, c.user_agent, c.count, c.first_seen, c.last_seen
		FROM client_errors c
		WHERE c.last_seen > $1
		  AND NOT EXISTS (SELECT 1 FROM alert_state s WHERE s.kind = 'seen' AND s.key = 'client:' || c.hash)
		ORDER BY c.hash, c.first_seen
		LIMIT $2`, since, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (AlertClientError, error) {
		var e AlertClientError
		err := r.Scan(&e.Hash, &e.Kind, &e.Message, &e.Stack, &e.Version, &e.Region, &e.UserAgent,
			&e.Count, &e.FirstSeen, &e.LastSeen)
		return e, err
	})
}

// ClientErrorTotals: a contagem total (todas as versões) de cada erro do cliente que teve
// atividade desde since.
func (s *Store) ClientErrorTotals(ctx context.Context, since time.Time) ([]ClientErrorTotal, error) {
	rows, err := s.pool.Query(ctx, `SELECT hash, sum(count)::bigint, min(first_seen), max(last_seen),
			(array_agg(kind ORDER BY last_seen DESC))[1],
			(array_agg(message ORDER BY last_seen DESC))[1],
			(array_agg(version ORDER BY last_seen DESC))[1]
		FROM client_errors
		WHERE hash IN (SELECT hash FROM client_errors WHERE last_seen > $1)
		GROUP BY hash`, since)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (ClientErrorTotal, error) {
		var t ClientErrorTotal
		err := r.Scan(&t.Hash, &t.Total, &t.FirstSeen, &t.LastSeen, &t.Kind, &t.Message, &t.Version)
		return t, err
	})
}

const alertEventCols = `at, kind, ref, amount, coalesce(character_id, 0), level, x, z, erevia_region(x, z), extra`

func collectAlertEvents(rows pgx.Rows, withCount bool) ([]AlertEvent, error) {
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (AlertEvent, error) {
		var e AlertEvent
		var x, z float32
		dst := []any{&e.At, &e.Kind, &e.Ref, &e.Amount, &e.CharacterID, &e.Level, &x, &z, &e.Region, &e.Extra}
		if withCount {
			dst = append(dst, &e.Count)
		}
		err := r.Scan(dst...)
		e.X, e.Z = float64(x), float64(z)
		return e, err
	})
}

// UnseenServerErrors: server_error desde since cujo hash (ref) ainda não foi visto. Uma
// linha por hash: a primeira ocorrência.
func (s *Store) UnseenServerErrors(ctx context.Context, since time.Time, limit int) ([]AlertEvent, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT ON (e.ref) `+alertEventCols+`
		FROM game_events e
		WHERE e.kind = 'server_error' AND e.at > $1
		  AND NOT EXISTS (SELECT 1 FROM alert_state s WHERE s.kind = 'seen' AND s.key = 'server:' || e.ref)
		ORDER BY e.ref, e.at
		LIMIT $2`, since, limit)
	if err != nil {
		return nil, err
	}
	return collectAlertEvents(rows, false)
}

// ErrorSpikes: server_error e db_error com mais de over ocorrências (soma de amount)
// desde since, por tipo e ref. O resto vem da ocorrência mais recente.
func (s *Store) ErrorSpikes(ctx context.Context, since time.Time, over int64) ([]AlertEvent, error) {
	rows, err := s.pool.Query(ctx, `WITH g AS (
			SELECT kind, ref, sum(amount)::bigint AS total, count(*) AS n, max(id) AS last
			FROM game_events WHERE kind IN ('server_error', 'db_error') AND at > $1
			GROUP BY kind, ref HAVING sum(amount) > $2
		)
		SELECT e.at, e.kind, e.ref, g.total, coalesce(e.character_id, 0), e.level, e.x, e.z, erevia_region(e.x, e.z), e.extra, g.n
		FROM g JOIN game_events e ON e.id = g.last
		ORDER BY g.total DESC`, since, over)
	if err != nil {
		return nil, err
	}
	return collectAlertEvents(rows, true)
}

// InvariantsSince: as regras de invariante quebradas desde since. Uma linha por regra: a
// primeira quebra, com Count = quantas desde since.
func (s *Store) InvariantsSince(ctx context.Context, since time.Time) ([]AlertEvent, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT ON (ref) `+alertEventCols+`, count(*) OVER (PARTITION BY ref)
		FROM game_events WHERE kind = 'invariant' AND at >= $1
		ORDER BY ref, at`, since)
	if err != nil {
		return nil, err
	}
	return collectAlertEvents(rows, true)
}

// SlowTicks: os slow_tick desde since, do mais velho ao mais novo.
func (s *Store) SlowTicks(ctx context.Context, since time.Time) ([]AlertEvent, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+alertEventCols+`
		FROM game_events WHERE kind = 'slow_tick' AND at > $1 ORDER BY at, id`, since)
	if err != nil {
		return nil, err
	}
	return collectAlertEvents(rows, false)
}

// PendingBugReports: relatos de /bug desde since ainda não avisados (depois do último
// avisado da conta e de tudo o que existia quando os alertas começaram), em ordem de id.
func (s *Store) PendingBugReports(ctx context.Context, since time.Time, limit int) ([]AlertBug, error) {
	rows, err := s.pool.Query(ctx, `SELECT b.id, b.account_id, coalesce(b.character_id, 0), b.at, b.character_name,
			b.level, b.x, b.z, erevia_region(b.x, b.z), b.version, b.user_agent, b.text
		FROM bug_reports b
		LEFT JOIN alert_state s ON s.kind = 'bug' AND s.key = b.account_id::text
		WHERE b.at > $1
		  AND b.id > coalesce(s.pos, 0)
		  AND b.id > (SELECT coalesce(max(pos), 0) FROM alert_state WHERE kind = 'cursor' AND key = 'bootstrap')
		ORDER BY b.id
		LIMIT $2`, since, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (AlertBug, error) {
		var b AlertBug
		var x, z float32
		err := r.Scan(&b.ID, &b.AccountID, &b.CharacterID, &b.At, &b.Name, &b.Level, &x, &z, &b.Region,
			&b.Version, &b.UserAgent, &b.Text)
		b.X, b.Z = float64(x), float64(z)
		return b, err
	})
}
