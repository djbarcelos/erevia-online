// Package storetest cria um Store isolado para testes, num schema próprio do Postgres.
//
// Os testes são pulados se TEST_DATABASE_URL estiver vazio.
package storetest

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"erevia/server/internal/store"
)

func New(t *testing.T) *store.Store {
	t.Helper()
	s, _ := NewWithURL(t)
	return s
}

// NewWithURL também devolve a URL do schema do teste (para conferir o banco direto).
func NewWithURL(t *testing.T) (*store.Store, string) {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL vazio: pulando teste com Postgres")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	admin.Close(ctx)

	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()

	s, err := store.Open(ctx, u.String())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() {
		s.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if c, err := pgx.Connect(ctx, base); err == nil {
			c.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
			c.Close(ctx)
		}
	})
	return s, u.String()
}
