package store_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

func TestClientErrorHash(t *testing.T) {
	v8a := "Error: boom\n    at explode (https://x/assets/index-abc.js:1:200)\n    at tick (https://x/assets/index-abc.js:1:900)"
	v8b := "Error: boom\n    at explode (https://x/assets/index-abc.js:1:200)\n    at other (https://x/assets/index-abc.js:1:50)"
	if store.FirstStackFrame(v8a) != "at explode (https://x/assets/index-abc.js:1:200)" {
		t.Fatalf("first frame (v8) = %q", store.FirstStackFrame(v8a))
	}
	// Quem chamou não muda o grupo; a mensagem e o lugar do erro mudam.
	if store.ClientErrorHash("Error: boom", v8a) != store.ClientErrorHash("Error: boom", v8b) {
		t.Fatal("same message and first frame should group")
	}
	if store.ClientErrorHash("Error: boom", v8a) == store.ClientErrorHash("Error: bang", v8a) {
		t.Fatal("different message should not group")
	}
	ff := "explode@https://x/assets/index-abc.js:1:200\ntick@https://x/assets/index-abc.js:1:900"
	if store.FirstStackFrame(ff) != "explode@https://x/assets/index-abc.js:1:200" {
		t.Fatalf("first frame (firefox) = %q", store.FirstStackFrame(ff))
	}
	if store.FirstStackFrame("") != "" || store.FirstStackFrame("só uma linha") != "só uma linha" {
		t.Fatal("first frame without frames")
	}
}

type clientErrorRow struct {
	Count       int64
	Message     string
	Kind        string
	AccountID   *int64
	CharacterID *int64
	Region      string
	FPS         *float32
	Ping        *int32
	Extra       map[string]any
	Repeated    bool // last_seen > first_seen
}

func readClientError(t *testing.T, db *pgx.Conn, hash, version string) clientErrorRow {
	t.Helper()
	var r clientErrorRow
	err := db.QueryRow(context.Background(), `SELECT count, message, kind, account_id, character_id, region, fps, ping, extra,
		last_seen > first_seen FROM client_errors WHERE hash = $1 AND version = $2`, hash, version).
		Scan(&r.Count, &r.Message, &r.Kind, &r.AccountID, &r.CharacterID, &r.Region, &r.FPS, &r.Ping, &r.Extra, &r.Repeated)
	if err != nil {
		t.Fatalf("read client error: %v", err)
	}
	return r
}

func TestRecordClientErrorsUpsert(t *testing.T) {
	s, url := storetest.NewWithURL(t)
	ctx := context.Background()
	db, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(ctx) })

	acc, _ := s.LoginIdentity(ctx, "dev", "ana", "Ana")
	other, _ := s.LoginIdentity(ctx, "dev", "bob", "Bob")
	mine, err := s.CreateCharacter(ctx, acc, "Aria", "mage", game.DefaultLook("mage"))
	if err != nil {
		t.Fatal(err)
	}
	theirs, _ := s.CreateCharacter(ctx, other, "Bobo", "knight", game.DefaultLook("knight"))

	stack := "Error: boom\n    at f (https://x/assets/index-abc.js:1:2)"
	hash := store.ClientErrorHash("Error: boom", stack)
	fps, ping := float32(58.5), int32(40)
	base := store.ClientError{Hash: hash, Version: "v1", Kind: "error", Message: "Error: boom", Stack: stack, Count: 3, UserAgent: "UA"}

	// Anônimo (tela de login): sem conta nem personagem, mesmo que o cliente mande um.
	anon := base
	anon.CharacterID = mine.ID
	if err := s.RecordClientErrors(ctx, []store.ClientError{anon}); err != nil {
		t.Fatal(err)
	}
	r := readClientError(t, db, hash, "v1")
	if r.Count != 3 || r.AccountID != nil || r.CharacterID != nil || r.Repeated {
		t.Fatalf("first = %+v", r)
	}

	// Repetido: soma a contagem e guarda o contexto da última vez. Mensagem fica a primeira.
	again := base
	again.Count, again.Message, again.Kind = 4, "outra mensagem", "rejection"
	again.AccountID, again.CharacterID, again.Region, again.FPS, again.Ping = acc, mine.ID, "Arredores", &fps, &ping
	again.Extra = []byte(`{"bytes":12}`)
	if err := s.RecordClientErrors(ctx, []store.ClientError{again}); err != nil {
		t.Fatal(err)
	}
	r = readClientError(t, db, hash, "v1")
	if r.Count != 7 || r.Message != "Error: boom" || r.Kind != "rejection" || r.AccountID == nil || *r.AccountID != acc ||
		r.CharacterID == nil || *r.CharacterID != mine.ID || r.Region != "Arredores" || r.FPS == nil || *r.FPS != 58.5 ||
		r.Ping == nil || *r.Ping != 40 || r.Extra["bytes"] != float64(12) || !r.Repeated {
		t.Fatalf("upsert = %+v", r)
	}

	// Personagem de outra conta não é gravado; outra versão é outra linha.
	foreign := base
	foreign.Version, foreign.AccountID, foreign.CharacterID = "v2", acc, theirs.ID
	if err := s.RecordClientErrors(ctx, []store.ClientError{foreign, foreign}); err != nil { // repetido no mesmo lote também soma
		t.Fatal(err)
	}
	r = readClientError(t, db, hash, "v2")
	if r.Count != 6 || r.AccountID == nil || *r.AccountID != acc || r.CharacterID != nil {
		t.Fatalf("foreign character = %+v", r)
	}
	if n := count(t, db, `SELECT count(*) FROM client_errors`); n != 2 {
		t.Fatalf("rows = %d, want 2", n)
	}

	// Conta que não existe (apagada no meio do caminho) fica anônima.
	ghost := base
	ghost.Version, ghost.AccountID = "v3", 999999
	if err := s.RecordClientErrors(ctx, []store.ClientError{ghost}); err != nil {
		t.Fatal(err)
	}
	if r := readClientError(t, db, hash, "v3"); r.AccountID != nil {
		t.Fatalf("ghost account = %+v", r)
	}
}

// Apagar a conta tira o dono dos erros dela (e só dela); o erro continua.
func TestDeleteAccountAnonymizesClientErrors(t *testing.T) {
	s, url := storetest.NewWithURL(t)
	ctx := context.Background()
	db, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(ctx) })

	acc, _ := s.LoginIdentity(ctx, "dev", "ana", "Ana")
	other, _ := s.LoginIdentity(ctx, "dev", "bob", "Bob")
	mine, _ := s.CreateCharacter(ctx, acc, "Aria", "mage", game.DefaultLook("mage"))
	theirs, _ := s.CreateCharacter(ctx, other, "Bobo", "knight", game.DefaultLook("knight"))
	rec := func(msg string, account, char int64) {
		e := store.ClientError{Hash: store.ClientErrorHash(msg, ""), Version: "v1", Kind: "error", Message: msg, Count: 1,
			AccountID: account, CharacterID: char, Region: "Cidade"}
		if err := s.RecordClientErrors(ctx, []store.ClientError{e}); err != nil {
			t.Fatal(err)
		}
	}
	rec("da ana", acc, mine.ID)
	rec("do bob", other, theirs.ID)

	if _, err := s.DeleteAccount(ctx, acc); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, `SELECT count(*) FROM client_errors`); n != 2 {
		t.Fatalf("rows = %d, want 2 (erro não some)", n)
	}
	if n := count(t, db, `SELECT count(*) FROM client_errors WHERE account_id = $1 OR character_id = $2`, acc, mine.ID); n != 0 {
		t.Fatalf("rows still linked to the deleted account = %d", n)
	}
	if n := count(t, db, `SELECT count(*) FROM client_errors WHERE account_id = $1 AND character_id = $2`, other, theirs.ID); n != 1 {
		t.Fatalf("other account rows = %d, want 1", n)
	}
}
