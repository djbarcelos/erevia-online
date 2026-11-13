package store

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// Reproduz a produção de 13/11/2026: o banco já tinha 1–8 e a 20 (LGPD, do outro PC) quando
// chegaram 9, 10, 11… (este PC). Sem permitir fora de ordem, o servidor não subia.
func TestMigrateAppliesOutOfOrderMigrations(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL vazio: pulando teste com Postgres")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	schema := fmt.Sprintf("test_ooo_%d", time.Now().UnixNano())
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c, _ := pgx.Connect(context.Background(), base)
		c.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		c.Close(context.Background())
	})
	admin.Close(ctx)
	u, _ := url.Parse(base)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	// 1) Banco "de produção": só 00001–00008 e 00020.
	all := mustSub(migrations, "migrations")
	old := fstest.MapFS{}
	entries, _ := fs.ReadDir(all, ".")
	var late []string
	for _, e := range entries {
		n := e.Name()
		data, _ := fs.ReadFile(all, n)
		if n < "00009" || strings.HasPrefix(n, "00020") {
			old[n] = &fstest.MapFile{Data: data}
		} else {
			late = append(late, n)
		}
	}
	if len(late) == 0 {
		t.Skip("não há migrações atrasadas para testar")
	}
	db := stdlib.OpenDBFromPool(pool)
	p, err := goose.NewProvider(goose.DialectPostgres, db, old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	db.Close()

	// 2) O servidor sobe com o conjunto completo: as atrasadas entram.
	if err := migrate(ctx, pool); err != nil {
		t.Fatalf("migrate com migrações atrasadas: %v (atrasadas: %v)", err, late)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM bug_reports`).Scan(&n); err != nil {
		t.Fatalf("a 00009 (bug_reports) não foi aplicada: %v", err)
	}
	// 3) Rodar de novo não faz nada (idempotente).
	if err := migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
}
