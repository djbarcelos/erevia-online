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
)

const discoveredMigration = "00036_discovered.sql"

// Migração 00036 (descoberta de pedras, tunnel#141): quem já tinha personagem ganha as 5
// pedras do mapa principal de antes do Brejo; personagem criado depois começa vazio.
func TestDiscoveredMigrationKeepsOldPlayersStones(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL vazio: pulando teste com Postgres")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	schema := fmt.Sprintf("test_discovered_%d", time.Now().UnixNano())
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer admin.Close(context.Background())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
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

	// O banco de hoje: todas as migrações menos a 00036.
	all := mustSub(migrations, "migrations")
	before := fstest.MapFS{}
	entries, err := fs.ReadDir(all, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == discoveredMigration || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		data, err := fs.ReadFile(all, e.Name())
		if err != nil {
			t.Fatal(err)
		}
		before[e.Name()] = &fstest.MapFile{Data: data}
	}
	if err := migrateFS(ctx, pool, before); err != nil {
		t.Fatalf("migrações de antes: %v", err)
	}
	var acc, vet int64
	if err := pool.QueryRow(ctx, `INSERT INTO accounts DEFAULT VALUES RETURNING id`).Scan(&acc); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO characters (account_id, name, class, kit_given) VALUES ($1, 'Veterano', 'knight', true) RETURNING id`, acc).Scan(&vet); err != nil {
		t.Fatal(err)
	}

	if err := migrateFS(ctx, pool, all); err != nil {
		t.Fatalf("00036: %v", err)
	}
	var nw int64
	if err := pool.QueryRow(ctx, `INSERT INTO characters (account_id, name, class, kit_given) VALUES ($1, 'Novato', 'mage', true) RETURNING id`, acc).Scan(&nw); err != nil {
		t.Fatal(err)
	}
	st := &Store{pool: pool}
	c, err := st.CharacterForAccount(ctx, acc, vet)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"cidade", "arredores", "ossos", "ermos", "vale"}
	if fmt.Sprint(c.Discovered.Stones) != fmt.Sprint(want) {
		t.Fatalf("veterano: %v, esperava %v (as do Brejo precisam ser descobertas a pé)", c.Discovered.Stones, want)
	}
	n, err := st.CharacterForAccount(ctx, acc, nw)
	if err != nil || len(n.Discovered.Stones) != 0 {
		t.Fatalf("novato: %+v / %v", n.Discovered, err)
	}
}
