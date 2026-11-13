package store_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

// Apagar a conta deixa a telemetria dela anônima: os eventos continuam (estatísticas), mas
// sem a conta e os personagens. Eventos que chegam depois (lote atrasado ou concorrente)
// também entram sem eles.
func TestDeleteAccountAnonymizesTelemetry(t *testing.T) {
	s, url := storetest.NewWithURL(t)
	ctx := context.Background()
	db, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(ctx) })

	acc, _ := s.LoginIdentity(ctx, "google", "sub-tele", "Tele")
	other, _ := s.LoginIdentity(ctx, "discord", "sub-outra", "Outra")
	a, err := s.CreateCharacter(ctx, acc, "Medido", "knight", game.DefaultLook("knight"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.CreateCharacter(ctx, other, "Intacto", "mage", game.DefaultLook("mage"))
	if err != nil {
		t.Fatal(err)
	}
	ev := func(char, account int64, kind string) store.GameEvent {
		return store.GameEvent{At: time.Now(), Kind: kind, CharacterID: char, AccountID: account, Ref: "x", Amount: 1}
	}
	if err := s.InsertEvents(ctx, []store.GameEvent{
		ev(a.ID, acc, "kill"), ev(a.ID, acc, "gold"), ev(a.ID, 0, "fps"), ev(b.ID, other, "kill"),
	}); err != nil {
		t.Fatal(err)
	}

	// Uma carta do personagem apagado para o da outra conta.
	if _, err := db.Exec(ctx, `INSERT INTO mail (to_id, from_id, from_name, body) VALUES ($1, $2, 'Medido', 'oi')`, b.ID, a.ID); err != nil {
		t.Fatal(err)
	}

	// Lotes gravados sem parar enquanto a conta é apagada.
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if err := s.InsertEvents(ctx, []store.GameEvent{ev(a.ID, acc, "session"), ev(b.ID, other, "session")}); err != nil {
				t.Error(err)
				return
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()
	time.Sleep(50 * time.Millisecond)
	if _, err := s.DeleteAccount(ctx, acc); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()

	// Lote atrasado, depois da exclusão (ex.: o fim de sessão de quem foi derrubado).
	if err := s.InsertEvents(ctx, []store.GameEvent{ev(a.ID, acc, "session")}); err != nil {
		t.Fatal(err)
	}

	if n := count(t, db, `SELECT count(*) FROM game_events WHERE account_id = $1 OR character_id = $2`, acc, a.ID); n != 0 {
		t.Fatalf("sobraram %d eventos ligados à conta apagada", n)
	}
	if n := count(t, db, `SELECT count(*) FROM game_events WHERE account_id IS NULL AND character_id IS NULL AND kind IN ('kill', 'gold', 'fps')`); n != 3 {
		t.Fatalf("eventos anonimizados = %d, quero 3 (as estatísticas continuam)", n)
	}
	if n := count(t, db, `SELECT count(*) FROM game_events WHERE account_id = $1 AND character_id = $2 AND kind = 'kill'`, other, b.ID); n != 1 {
		t.Fatalf("eventos da outra conta = %d, quero 1 intacto", n)
	}
	// A carta fica com quem recebeu, sem o remetente.
	if n := count(t, db, `SELECT count(*) FROM mail WHERE to_id = $1 AND from_id IS NULL AND from_name = 'Personagem apagado'`, b.ID); n != 1 {
		t.Fatalf("carta anonimizada = %d, quero 1", n)
	}
}
