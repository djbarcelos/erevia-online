package store_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

// count roda um SELECT count(*) direto no schema do teste.
func count(t *testing.T, db *pgx.Conn, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func TestDeleteAccountRemovesEverything(t *testing.T) {
	s, url := storetest.NewWithURL(t)
	ctx := context.Background()
	db, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(ctx) })

	acc, _ := s.LoginIdentity(ctx, "google", "sub-apaga", "Apaga")
	other, _ := s.LoginIdentity(ctx, "discord", "sub-fica", "Fica")
	token, _ := s.CreateSession(ctx, acc)
	otherToken, _ := s.CreateSession(ctx, other)
	newChar := func(account int64, name, class string) store.Character {
		c, err := s.CreateCharacter(ctx, account, name, class, game.DefaultLook(class))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	a1, a2, a3 := newChar(acc, "Alfa", "knight"), newChar(acc, "Beta", "mage"), newChar(acc, "Gama", "rogue")
	b1, b2, b3 := newChar(other, "Delta", "knight"), newChar(other, "Epsilon", "mage"), newChar(other, "Zeta", "rogue")

	// Lobos: líder da conta apagada, com um membro e um oficial de outra conta (o oficial assume).
	lobos, _ := s.CreateClan(ctx, "Lobos", a1.ID)
	// Sos: líder da conta apagada, sozinho (é desfeito).
	sos, _ := s.CreateClan(ctx, "Sos", a2.ID)
	// Corvos: líder de outra conta, com um membro da conta apagada (só o membro sai).
	corvos, _ := s.CreateClan(ctx, "Corvos", b3.ID)
	for _, op := range []store.ClanOp{
		{Kind: store.ClanOpJoin, ClanID: lobos, CharacterID: b1.ID, Rank: store.ClanRankMember},
		{Kind: store.ClanOpJoin, ClanID: lobos, CharacterID: b2.ID, Rank: store.ClanRankOfficer},
		{Kind: store.ClanOpJoin, ClanID: corvos, CharacterID: a3.ID, Rank: store.ClanRankMember},
	} {
		if err := s.ApplyClanOp(ctx, op); err != nil {
			t.Fatal(err)
		}
	}

	// Itens, moedas e armazém da conta; o nome de um personagem no registro do raid boss.
	seq := time.Now().UnixNano()
	if err := s.SaveStates(ctx, []store.CharacterState{{
		CharacterID: a1.ID, Level: 5, HP: -1, MP: -1, Gold: 99, Seq: seq,
		Items:   []game.ItemRecord{{Slot: 0, ItemID: "bone_old", Count: 3}},
		Storage: &store.StorageState{AccountID: acc, Gold: 500, Seq: seq, Items: []game.ItemRecord{{Slot: 0, ItemID: "bone_old", Count: 9}}},
	}}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	s.SaveBoss(ctx, store.Boss{ID: "ozrhal", RespawnAt: now, KilledAt: now, TopName: "Alfa"})
	s.SaveBoss(ctx, store.Boss{ID: "outro", RespawnAt: now, KilledAt: now, TopName: "Delta"})

	d, err := s.DeleteAccount(ctx, acc)
	if err != nil {
		t.Fatal(err)
	}
	if d.AccountID != acc || !slices.Equal(d.Characters, []int64{a1.ID, a2.ID, a3.ID}) {
		t.Fatalf("deleted = %+v", d)
	}
	wantClans := []store.ClanSuccession{{ClanID: lobos, LeaderID: b2.ID}, {ClanID: sos, LeaderID: 0}}
	if !slices.Equal(d.Clans, wantClans) {
		t.Fatalf("clans = %+v, want %+v", d.Clans, wantClans)
	}

	// Nada da conta sobra em nenhuma tabela.
	ids := d.Characters
	for _, q := range []struct {
		query string
		arg   any
	}{
		{`SELECT count(*) FROM accounts WHERE id = $1`, acc},
		{`SELECT count(*) FROM identities WHERE account_id = $1 OR subject = 'sub-apaga'`, acc},
		{`SELECT count(*) FROM sessions WHERE account_id = $1`, acc},
		{`SELECT count(*) FROM account_items WHERE account_id = $1`, acc},
		{`SELECT count(*) FROM characters WHERE account_id = $1`, acc},
		{`SELECT count(*) FROM characters WHERE id = ANY($1)`, ids},
		{`SELECT count(*) FROM character_items WHERE character_id = ANY($1)`, ids},
		{`SELECT count(*) FROM clan_members WHERE character_id = ANY($1)`, ids},
		{`SELECT count(*) FROM clans WHERE leader_id = ANY($1)`, ids},
		{`SELECT count(*) FROM world_bosses WHERE top_name = 'Alfa'`, nil},
	} {
		var n int
		if q.arg == nil {
			n = count(t, db, q.query)
		} else {
			n = count(t, db, q.query, q.arg)
		}
		if n != 0 {
			t.Errorf("%s → %d linhas", q.query, n)
		}
	}
	// E nada fica órfão: todo clã tem membros e o líder é membro com cargo de líder.
	if n := count(t, db, `SELECT count(*) FROM clans c WHERE NOT EXISTS (SELECT 1 FROM clan_members m WHERE m.clan_id = c.id)`); n != 0 {
		t.Errorf("%d clãs sem membros", n)
	}
	if n := count(t, db, `SELECT count(*) FROM clans c WHERE NOT EXISTS
		(SELECT 1 FROM clan_members m WHERE m.clan_id = c.id AND m.character_id = c.leader_id AND m.rank = 2)`); n != 0 {
		t.Errorf("%d clãs com líder que não é membro-líder", n)
	}
	if n := count(t, db, `SELECT count(*) FROM world_bosses WHERE top_name = 'Delta'`); n != 1 {
		t.Errorf("o nome de outra conta sumiu do raid boss")
	}

	// A sessão morreu; a outra conta continua inteira.
	if _, err := s.AccountBySession(ctx, token); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("session after delete: %v", err)
	}
	if id, err := s.AccountBySession(ctx, otherToken); err != nil || id != other {
		t.Fatalf("other session = %d, %v", id, err)
	}
	if chars, _ := s.ListCharacters(ctx, other); len(chars) != 3 {
		t.Fatalf("other characters = %d", len(chars))
	}
	clans, err := s.LoadClans(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]store.Clan{}
	for _, c := range clans {
		byName[c.Name] = c
	}
	if c := byName["Lobos"]; c.LeaderID != b2.ID || len(c.Members) != 2 {
		t.Fatalf("Lobos = %+v", c)
	}
	if c := byName["Corvos"]; c.LeaderID != b3.ID || len(c.Members) != 1 {
		t.Fatalf("Corvos = %+v", c)
	}
	if _, ok := byName["Sos"]; ok || len(clans) != 2 {
		t.Fatalf("clans = %+v", clans)
	}

	// Um salvamento atrasado da zona (personagem e armazém) não recria nada nem dá erro.
	if err := s.SaveStates(ctx, []store.CharacterState{{
		CharacterID: a1.ID, Level: 6, HP: -1, MP: -1, Gold: 1, Seq: time.Now().UnixNano(),
		Items:   []game.ItemRecord{{Slot: 0, ItemID: "bone_old", Count: 1}},
		Storage: &store.StorageState{AccountID: acc, Gold: 1, Seq: time.Now().UnixNano(), Items: []game.ItemRecord{{Slot: 0, ItemID: "bone_old", Count: 1}}},
	}}); err != nil {
		t.Fatalf("late save: %v", err)
	}
	if n := count(t, db, `SELECT count(*) FROM character_items WHERE character_id = $1`, a1.ID) +
		count(t, db, `SELECT count(*) FROM account_items WHERE account_id = $1`, acc) +
		count(t, db, `SELECT count(*) FROM accounts WHERE id = $1`, acc); n != 0 {
		t.Fatalf("late save recreated %d rows", n)
	}
	// Operações de clã atrasadas também não recriam o personagem nem o clã desfeito.
	_ = s.ApplyClanOp(ctx, store.ClanOp{Kind: store.ClanOpLeave, CharacterID: a3.ID, LeftAt: now})
	if err := s.ApplyClanOp(ctx, store.ClanOp{Kind: store.ClanOpJoin, ClanID: sos, CharacterID: b1.ID}); err == nil {
		t.Fatal("join into a deleted clan should fail")
	}

	// Apagar de novo: não existe mais. Entrar de novo com o mesmo Google cria uma conta nova e vazia.
	if _, err := s.DeleteAccount(ctx, acc); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	again, err := s.LoginIdentity(ctx, "google", "sub-apaga", "Apaga")
	if err != nil || again == acc {
		t.Fatalf("relogin = %d, %v", again, err)
	}
	if chars, _ := s.ListCharacters(ctx, again); len(chars) != 0 {
		t.Fatalf("new account has %d characters", len(chars))
	}
	// O nome dos personagens apagados fica livre.
	if _, err := s.CreateCharacter(ctx, again, "Alfa", "knight", game.DefaultLook("knight")); err != nil {
		t.Fatalf("reuse name: %v", err)
	}
}

func TestDeleteAccountWithoutCharacters(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "vazio", "Vazio")
	d, err := s.DeleteAccount(ctx, acc)
	if err != nil || len(d.Characters) != 0 || len(d.Clans) != 0 {
		t.Fatalf("delete = %+v, %v", d, err)
	}
	if _, err := s.DeleteAccount(ctx, 999999); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown account: %v", err)
	}
}

// O banco não deixa apagar o líder de um clã sem passar a liderança (migração 00020): um
// clã nunca some junto com o líder.
func TestClanLeaderCannotBeDeletedDirectly(t *testing.T) {
	s, url := storetest.NewWithURL(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "lider", "Lider")
	c, _ := s.CreateCharacter(ctx, acc, "Chefe", "knight", game.DefaultLook("knight"))
	if _, err := s.CreateClan(ctx, "Trono", c.ID); err != nil {
		t.Fatal(err)
	}
	db, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	_, err = db.Exec(ctx, `DELETE FROM characters WHERE id = $1`, c.ID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("delete leader = %v, want foreign key violation", err)
	}
	// Pela conta, o clã vazio é desfeito antes e tudo sai.
	if d, err := s.DeleteAccount(ctx, acc); err != nil || len(d.Clans) != 1 || d.Clans[0].LeaderID != 0 {
		t.Fatalf("delete account = %+v, %v", d, err)
	}
	if clans, _ := s.LoadClans(ctx); len(clans) != 0 {
		t.Fatalf("clans = %+v", clans)
	}
}
