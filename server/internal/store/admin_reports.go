package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Portal de administração, seção Relatos: os relatos de /bug (bug_reports) e os erros do
// navegador (client_errors). Só leitura, exceto marcar um relato como resolvido.

// BugFilter filtra a lista de relatos. Campos vazios não filtram.
type BugFilter struct {
	Status  string // "open" (padrão), "resolved" ou "all"
	Version string // começo do commit (7 letras bastam)
	Region  string // nome da região (erevia_region)
	Query   string // trecho do texto ou do nome do personagem
	Before  int64  // paginação: só ids menores que este
	Limit   int
}

// BugRow é um relato como o portal mostra.
type BugRow struct {
	ID             int64      `json:"id"`
	At             time.Time  `json:"at"`
	AccountID      int64      `json:"accountId"`
	CharacterID    *int64     `json:"characterId"`
	CharacterName  string     `json:"characterName"`
	Level          int        `json:"level"`
	X              float64    `json:"x"`
	Z              float64    `json:"z"`
	Region         string     `json:"region"`
	Version        string     `json:"version"`
	UserAgent      string     `json:"userAgent"`
	Text           string     `json:"text"`
	Resolved       bool       `json:"resolved"`
	ResolvedAt     *time.Time `json:"resolvedAt"`
	ResolvedBy     *int64     `json:"resolvedBy"`
	ResolvedByName string     `json:"resolvedByName"`
	ResolvedNote   string     `json:"resolvedNote"`
	// Shot: o print do /bug completo (bug_report_shots, migração 00026); nil sem print.
	Shot *BugShotInfo `json:"shot"`
}

// BugShotInfo é o tamanho do print de um relato (a imagem sai por
// GET /api/admin/bug-reports/{id}/shot).
type BugShotInfo struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

const bugCols = `id, at, account_id, character_id, character_name, level, x, z, erevia_region(x, z), version,
	user_agent, text, resolved, resolved_at, resolved_by, resolved_by_name, resolved_note,
	(SELECT s.width FROM bug_report_shots s WHERE s.report_id = bug_reports.id),
	(SELECT s.height FROM bug_report_shots s WHERE s.report_id = bug_reports.id)`

func scanBug(r pgx.CollectableRow) (BugRow, error) {
	var b BugRow
	var x, z float32
	var sw, sh *int
	err := r.Scan(&b.ID, &b.At, &b.AccountID, &b.CharacterID, &b.CharacterName, &b.Level, &x, &z, &b.Region,
		&b.Version, &b.UserAgent, &b.Text, &b.Resolved, &b.ResolvedAt, &b.ResolvedBy, &b.ResolvedByName, &b.ResolvedNote, &sw, &sh)
	if sw != nil && sh != nil {
		b.Shot = &BugShotInfo{Width: *sw, Height: *sh}
	}
	b.X, b.Z = float64(x), float64(z)
	return b, err
}

// likePattern escapa % e _ para um ILIKE de "contém".
func likePattern(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(s) + "%"
}

// ListBugs devolve os relatos do filtro, do mais novo ao mais velho.
func (s *Store) ListBugs(ctx context.Context, f BugFilter) ([]BugRow, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	var resolved *bool
	switch f.Status {
	case "all":
	case "resolved":
		v := true
		resolved = &v
	default:
		v := false
		resolved = &v
	}
	var q *string
	if t := strings.TrimSpace(f.Query); t != "" {
		p := likePattern(t)
		q = &p
	}
	rows, err := s.pool.Query(ctx, `SELECT `+bugCols+` FROM bug_reports
		WHERE ($1::boolean IS NULL OR resolved = $1)
		  AND ($2 = '' OR version LIKE $2 || '%')
		  AND ($3 = '' OR erevia_region(x, z) = $3)
		  AND ($4::text IS NULL OR text ILIKE $4 OR character_name ILIKE $4)
		  AND ($5 = 0 OR id < $5)
		ORDER BY id DESC LIMIT $6`,
		resolved, strings.TrimSpace(f.Version), f.Region, q, f.Before, f.Limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanBug)
}

// BugFacets são as opções dos filtros (com quantos abertos e no total).
type BugFacets struct {
	Open     int64        `json:"open"`
	Resolved int64        `json:"resolved"`
	Versions []FacetCount `json:"versions"`
	Regions  []FacetCount `json:"regions"`
}

// FacetCount é uma opção de filtro com as contagens.
type FacetCount struct {
	Value string `json:"value"`
	Open  int64  `json:"open"`
	Total int64  `json:"total"`
}

// BugFacets conta os relatos por estado, versão (as 30 mais recentes) e região.
func (s *Store) BugFacets(ctx context.Context) (BugFacets, error) {
	var out BugFacets
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FILTER (WHERE NOT resolved), count(*) FILTER (WHERE resolved) FROM bug_reports`).
		Scan(&out.Open, &out.Resolved); err != nil {
		return out, err
	}
	scan := func(r pgx.CollectableRow) (FacetCount, error) {
		var f FacetCount
		return f, r.Scan(&f.Value, &f.Open, &f.Total)
	}
	rows, err := s.pool.Query(ctx, `SELECT version, count(*) FILTER (WHERE NOT resolved), count(*)
		FROM bug_reports GROUP BY version ORDER BY max(at) DESC LIMIT 30`)
	if err != nil {
		return out, err
	}
	if out.Versions, err = pgx.CollectRows(rows, scan); err != nil {
		return out, err
	}
	rows, err = s.pool.Query(ctx, `SELECT erevia_region(x, z), count(*) FILTER (WHERE NOT resolved), count(*)
		FROM bug_reports GROUP BY 1 ORDER BY 3 DESC`)
	if err != nil {
		return out, err
	}
	out.Regions, err = pgx.CollectRows(rows, scan)
	return out, err
}

// bugKnownCols são as colunas que o BugRow já mostra; o resto da linha vai em Extra.
var bugKnownCols = map[string]bool{
	"id": true, "at": true, "account_id": true, "character_id": true, "character_name": true, "level": true,
	"x": true, "z": true, "version": true, "user_agent": true, "text": true, "resolved": true,
	"resolved_at": true, "resolved_by": true, "resolved_by_name": true, "resolved_note": true,
}

// maxBugExtra: tamanho máximo (bytes do JSON) de cada campo extra devolvido ao portal.
const maxBugExtra = 16 << 10

// BugDetail devolve o relato e as colunas que a tabela tiver além das conhecidas (ex.: o
// contexto e o print do /bug completo, que outra migração acrescenta), cada uma como JSON.
// Valor grande demais (um print, por exemplo) volta como {"omitted": true, "bytes": n}.
func (s *Store) BugDetail(ctx context.Context, id int64) (BugRow, map[string]json.RawMessage, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+bugCols+` FROM bug_reports WHERE id = $1`, id)
	if err != nil {
		return BugRow{}, nil, err
	}
	b, err := pgx.CollectExactlyOneRow(rows, scanBug)
	if errors.Is(err, pgx.ErrNoRows) {
		return BugRow{}, nil, ErrNotFound
	}
	if err != nil {
		return BugRow{}, nil, err
	}
	var all map[string]json.RawMessage
	if err := s.pool.QueryRow(ctx, `SELECT to_jsonb(b) FROM bug_reports b WHERE id = $1`, id).Scan(&all); err != nil {
		return b, nil, err
	}
	extra := map[string]json.RawMessage{}
	for k, v := range all {
		if bugKnownCols[k] || string(v) == "null" {
			continue
		}
		if len(v) > maxBugExtra {
			v, _ = json.Marshal(map[string]any{"omitted": true, "bytes": len(v)})
		}
		extra[k] = v
	}
	return b, extra, nil
}

// ResolveBug marca (ou desmarca) o relato como resolvido, com quem, quando e a nota.
func (s *Store) ResolveBug(ctx context.Context, id int64, resolved bool, byAccount int64, byName, note string) (BugRow, error) {
	var rows pgx.Rows
	var err error
	if resolved {
		rows, err = s.pool.Query(ctx, `UPDATE bug_reports SET resolved = true, resolved_at = now(),
			resolved_by = $2, resolved_by_name = $3, resolved_note = $4 WHERE id = $1 RETURNING `+bugCols,
			id, byAccount, byName, note)
	} else {
		rows, err = s.pool.Query(ctx, `UPDATE bug_reports SET resolved = false, resolved_at = NULL,
			resolved_by = NULL, resolved_by_name = '', resolved_note = '' WHERE id = $1 RETURNING `+bugCols, id)
	}
	if err != nil {
		return BugRow{}, err
	}
	b, err := pgx.CollectExactlyOneRow(rows, scanBug)
	if errors.Is(err, pgx.ErrNoRows) {
		return BugRow{}, ErrNotFound
	}
	return b, err
}

// ClientErrorRow é um erro do navegador agrupado (mesmo hash) como o portal mostra.
type ClientErrorRow struct {
	ID        int64     `json:"id"` // a linha mais recente do grupo
	Hash      string    `json:"hash"`
	Kind      string    `json:"kind"`
	Message   string    `json:"message"`
	Stack     string    `json:"stack"`
	Count     int64     `json:"count"`
	Versions  []string  `json:"versions"` // em quais versões apareceu (da mais recente)
	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
	UserAgent string    `json:"userAgent"` // da última ocorrência
	Region    string    `json:"region"`
}

// TopClientErrors devolve os erros do navegador mais frequentes que aconteceram nos
// últimos `days` dias, juntando as versões do mesmo erro (version vazio = todas).
func (s *Store) TopClientErrors(ctx context.Context, version string, days, limit int) ([]ClientErrorRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := s.pool.Query(ctx, `
		SELECT (array_agg(id ORDER BY last_seen DESC))[1], hash, (array_agg(kind ORDER BY last_seen DESC))[1],
		       (array_agg(message ORDER BY first_seen))[1], (array_agg(stack ORDER BY first_seen))[1],
		       sum(count)::bigint, array_agg(version ORDER BY last_seen DESC), min(first_seen), max(last_seen),
		       (array_agg(user_agent ORDER BY last_seen DESC))[1], (array_agg(region ORDER BY last_seen DESC))[1]
		FROM client_errors
		WHERE last_seen > now() - make_interval(days => $1) AND ($2 = '' OR version LIKE $2 || '%')
		GROUP BY hash ORDER BY 6 DESC, 9 DESC LIMIT $3`, days, strings.TrimSpace(version), limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (ClientErrorRow, error) {
		var e ClientErrorRow
		err := r.Scan(&e.ID, &e.Hash, &e.Kind, &e.Message, &e.Stack, &e.Count, &e.Versions, &e.FirstSeen, &e.LastSeen,
			&e.UserAgent, &e.Region)
		return e, err
	})
}

// ClientErrorVersion resume os erros de uma versão do cliente.
type ClientErrorVersion struct {
	Version  string    `json:"version"`
	Distinct int64     `json:"distinct"` // erros diferentes
	Total    int64     `json:"total"`    // ocorrências
	LastSeen time.Time `json:"lastSeen"`
}

// ClientErrorVersions resume os erros por versão (as 20 mais recentes do período).
func (s *Store) ClientErrorVersions(ctx context.Context, days int) ([]ClientErrorVersion, error) {
	rows, err := s.pool.Query(ctx, `SELECT version, count(*), sum(count)::bigint, max(last_seen) FROM client_errors
		WHERE last_seen > now() - make_interval(days => $1) GROUP BY version ORDER BY 4 DESC LIMIT 20`, days)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (ClientErrorVersion, error) {
		var v ClientErrorVersion
		return v, r.Scan(&v.Version, &v.Distinct, &v.Total, &v.LastSeen)
	})
}
