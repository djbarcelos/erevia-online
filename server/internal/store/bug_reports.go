package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// /bug completo: o print do canvas que o cliente anexa ao próprio relato (tabela
// bug_report_shots, migração 00026). O contexto do cliente vai na coluna
// bug_reports.context, gravada junto com o relato (CreateBugReport).

// ErrShotExists: o relato já tem um print (só vale um).
var ErrShotExists = errors.New("bug report already has a screenshot")

// BugShot é o print anexado a um relato.
type BugShot struct {
	At            time.Time
	Width, Height int
	JPEG          []byte
}

// AttachBugShot anexa o print ao relato reportID, se ele for da conta accountID e tiver
// sido feito há menos de maxAge. ErrNotFound: o relato não existe, é de outra conta ou
// passou do prazo (as três dão a mesma resposta, para não revelar relatos alheios);
// ErrShotExists: já tem print.
func (s *Store) AttachBugShot(ctx context.Context, reportID, accountID int64, shot BugShot, maxAge time.Duration) error {
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO bug_report_shots (report_id, width, height, jpeg)
		 SELECT id, $3, $4, $5 FROM bug_reports
		 WHERE id = $1 AND account_id = $2 AND at > now() - make_interval(secs => $6)
		 ON CONFLICT (report_id) DO NOTHING`,
		reportID, accountID, shot.Width, shot.Height, shot.JPEG, maxAge.Seconds())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var exists bool
	err = s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM bug_report_shots s JOIN bug_reports b ON b.id = s.report_id
		 WHERE s.report_id = $1 AND b.account_id = $2)`, reportID, accountID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return ErrShotExists
	}
	return ErrNotFound
}

// BugReportShot devolve o print do relato (portal de administração).
func (s *Store) BugReportShot(ctx context.Context, reportID int64) (BugShot, error) {
	var b BugShot
	err := s.pool.QueryRow(ctx, `SELECT at, width, height, jpeg FROM bug_report_shots WHERE report_id = $1`, reportID).
		Scan(&b.At, &b.Width, &b.Height, &b.JPEG)
	if errors.Is(err, pgx.ErrNoRows) {
		return BugShot{}, ErrNotFound
	}
	return b, err
}

// BugReportContext devolve o contexto do cliente gravado no relato (nil: relato sem
// contexto, ex.: /bug de um cliente antigo).
func (s *Store) BugReportContext(ctx context.Context, reportID int64) ([]byte, error) {
	var c []byte
	err := s.pool.QueryRow(ctx, `SELECT context::text FROM bug_reports WHERE id = $1`, reportID).Scan(&c)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}
