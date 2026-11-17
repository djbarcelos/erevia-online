package store

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Os dois PCs usam faixas de números de migração reservadas, então uma de número menor
// pode chegar à produção depois de uma maior (ex.: a 00009 depois da 00020). O servidor
// tem que aplicá-la e subir, em vez de recusar a migração "fora de ordem".
func TestMigrateAcceptsOutOfOrder(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL vazio: pulando teste com Postgres")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schema := fmt.Sprintf("test_ooo_%d", time.Now().UnixNano())
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer admin.Close(context.Background())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	sql := func(table string) *fstest.MapFile {
		return &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE " + table + " (id int);\n-- +goose Down\nDROP TABLE " + table + ";\n")}
	}
	// Produção hoje: 00001 e 00020 aplicadas.
	first := fstest.MapFS{"00001_a.sql": sql("ooo_a"), "00020_c.sql": sql("ooo_c")}
	if err := migrateFS(ctx, pool, first); err != nil {
		t.Fatalf("primeiro deploy: %v", err)
	}
	// Depois chega a 00009 (do outro PC).
	later := fstest.MapFS{"00001_a.sql": sql("ooo_a"), "00009_b.sql": sql("ooo_b"), "00020_c.sql": sql("ooo_c")}
	if err := migrateFS(ctx, pool, later); err != nil {
		t.Fatalf("migração fora de ordem recusada: %v", err)
	}
	var ok bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('ooo_b') IS NOT NULL").Scan(&ok); err != nil || !ok {
		t.Fatalf("a 00009 não foi aplicada (ok=%v, err=%v)", ok, err)
	}
}
