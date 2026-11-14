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

	"erevia/server/internal/game"
)

const classesMigration = "00052_classes.sql"

// Migração 00052 (subclasse, tunnel#156): os personagens de antes ficam com a classe deles
// como principal e ativa, e o progresso de hoje (nível, XP, atributos, skills, barra e
// especialização) vira o da principal. O Down devolve a principal às colunas de quem
// estava com a subclasse ativa.
func TestClassesMigrationKeepsCurrentProgress(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL vazio: pulando teste com Postgres")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	schema := fmt.Sprintf("test_classes_%d", time.Now().UnixNano())
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

	// O banco de hoje: todas as migrações menos a 00052.
	all := mustSub(migrations, "migrations")
	before := fstest.MapFS{}
	entries, err := fs.ReadDir(all, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == classesMigration || !strings.HasSuffix(e.Name(), ".sql") {
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
	var acc int64
	if err := pool.QueryRow(ctx, `INSERT INTO accounts DEFAULT VALUES RETURNING id`).Scan(&acc); err != nil {
		t.Fatal(err)
	}
	xp := game.XPLevelStart(30) + 1234
	var vet, novato int64
	if err := pool.QueryRow(ctx, `INSERT INTO characters (account_id, name, class, level, xp, attr_str, attr_con, skill_ranks, hotbar, spec, kit_given)
		VALUES ($1, 'Veterano', 'knight', 30, $2, 7, 3, '{"1": 4, "2": 1}', '["skill:1", "", "item:potion_hp_s"]', 'bastion', true) RETURNING id`, acc, xp).Scan(&vet); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO characters (account_id, name, class, kit_given) VALUES ($1, 'Novato', 'mage', true) RETURNING id`, acc).Scan(&novato); err != nil {
		t.Fatal(err)
	}

	// Agora a 00052.
	if err := migrateFS(ctx, pool, all); err != nil {
		t.Fatalf("00052: %v", err)
	}
	st := &Store{pool: pool}
	c, err := st.CharacterForAccount(ctx, acc, vet)
	if err != nil {
		t.Fatal(err)
	}
	set := c.Classes
	m := set.Find("knight")
	if set.Main != "knight" || set.Active != "knight" || len(set.List) != 1 || m == nil {
		t.Fatalf("classes do veterano: %+v", set)
	}
	if m.Level != 30 || m.XP != xp || m.Attrs != (game.Attrs{Str: 7, Con: 3}) || m.Ranks[1] != 4 || m.Ranks[2] != 1 ||
		len(m.Hotbar) != 3 || m.Hotbar[2] != "item:potion_hp_s" || m.Spec != "bastion" {
		t.Fatalf("progresso da principal: %+v", m)
	}
	if c.ActiveClass != "knight" || c.Level != 30 || c.XP != xp || c.ClassLevels != nil {
		t.Fatalf("veterano carregado: %+v", c)
	}
	n, err := st.CharacterForAccount(ctx, acc, novato)
	if err != nil || n.Classes.Main != "mage" || n.Classes.Active != "mage" || n.Classes.Find("mage").Level != 1 {
		t.Fatalf("novato: %+v / %v", n.Classes, err)
	}

	// O veterano pega a subclasse mago e fica com ela ativa (as colunas viram as do mago).
	sub := game.ClassSet{Main: "knight", Active: "mage", List: []game.ClassProgress{*m, game.NewSubclass("mage")}}
	if err := st.SaveStates(ctx, []CharacterState{{CharacterID: vet, Level: 20, XP: game.XPLevelStart(20), HP: -1, MP: -1,
		Attrs: game.Attrs{Int: 2}, Classes: sub, Seq: 1}}); err != nil {
		t.Fatal(err)
	}

	// Down: a coluna sai, e o cavaleiro volta às colunas.
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, all, goose.WithAllowOutofOrder(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.ApplyVersion(ctx, 52, false); err != nil {
		t.Fatalf("Down da 00052: %v", err)
	}
	var level, str, con, intel int
	var gotXP int64
	var spec, ranks string
	if err := pool.QueryRow(ctx, `SELECT level, xp, attr_str, attr_con, attr_int, spec, skill_ranks::text FROM characters WHERE id = $1`, vet).
		Scan(&level, &gotXP, &str, &con, &intel, &spec, &ranks); err != nil {
		t.Fatal(err)
	}
	if level != 30 || gotXP != xp || str != 7 || con != 3 || intel != 0 || spec != "bastion" || !strings.Contains(ranks, `"1": 4`) {
		t.Fatalf("depois do Down: nível %d XP %d for %d con %d int %d spec %q ranks %s", level, gotXP, str, con, intel, spec, ranks)
	}
	var hasCol bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = 'characters' AND column_name = 'classes')`, schema).Scan(&hasCol); err != nil || hasCol {
		t.Fatalf("a coluna classes continua lá (%v)", err)
	}
}
