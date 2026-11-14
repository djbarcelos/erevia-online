package store_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/jackc/pgx/v5"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

// Curva de XP nova (tunnel#149, migração 00019): quem foi salvo na curva 1 é convertido ao
// carregar, fica no mesmo nível e na mesma fração dele, e a conversão já fica salva (roda
// uma vez). No 30 (o máximo de antes) começa o 30 → 31 do zero e mantém a Maestria.
func TestCharacterXPConvertsFromOldCurve(t *testing.T) {
	s, url := storetest.NewWithURL(t)
	ctx := context.Background()
	db, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)

	// XP de antes: o início e o meio de cada nível na curva 1 (80·l^2,2, ×1,1 a ×2 do 20 ao
	// 29). Os números são os da tabela do docs/progressao.md de antes do tunnel#149.
	v1Start := map[int]int64{1: 0, 19: 283_476, 20: 335_517, 29: 1_575_947, 30: 1_839_820}
	v1Next := map[int]int64{1: 80, 19: 52_041, 20: 64_084, 29: 263_873}
	cases := []struct {
		level int
		frac  float64
	}{{1, 0}, {1, 0.5}, {19, 0}, {19, 0.5}, {20, 0}, {20, 0.5}, {29, 0}, {29, 0.5}, {30, 0}, {30, 0.7}}

	acc, err := s.LoginIdentity(ctx, "dev", "veteranos", "veteranos")
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range cases {
		name := fmt.Sprintf("Vet%d", i)
		if i%3 == 0 { // 3 personagens por conta
			if acc, err = s.LoginIdentity(ctx, "dev", name, name); err != nil {
				t.Fatal(err)
			}
		}
		ch, err := s.CreateCharacter(ctx, acc, name, "knight", game.DefaultLook("knight"))
		if err != nil {
			t.Fatal(err)
		}
		old := v1Start[c.level] + int64(c.frac*float64(v1Next[c.level]))
		if c.level == 30 {
			old = v1Start[30] + int64(c.frac*100_000) // no 30 a XP passava do início (barra travada)
		}
		mastery := `{}`
		if c.level == 30 {
			mastery = `{"points": 5, "xp": 1000, "ranks": {"wayfarer": 3}}`
		}
		if _, err := db.Exec(ctx, `UPDATE characters SET level = $2, xp = $3, xp_curve = 1, mastery = $4::jsonb, mastery_level = 5 WHERE id = $1`,
			ch.ID, c.level, old, mastery); err != nil {
			t.Fatal(err)
		}

		got, err := s.CharacterForAccount(ctx, acc, ch.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Level != c.level || game.LevelForXP(got.XP) != c.level {
			t.Fatalf("nível %d (%.0f%%): carregou no %d com %d de XP", c.level, c.frac*100, game.LevelForXP(got.XP), got.XP)
		}
		frac := float64(got.XP-game.XPLevelStart(c.level)) / float64(game.XPToNext(c.level))
		want := c.frac
		if c.level == 30 {
			want = 0 // a barra do 30 estava cheia ou travada: começa o 30 → 31 do zero
			if got.Mastery.Points != 5 || got.Mastery.Ranks["wayfarer"] != 3 {
				t.Fatalf("a Maestria do 30 se perdeu: %+v", got.Mastery)
			}
		}
		if math.Abs(frac-want) > 1/float64(game.XPToNext(c.level))+1e-4 {
			t.Fatalf("nível %d: fração %.4f, quero %.4f", c.level, frac, want)
		}

		// Salvo na curva 2: carregar de novo não converte outra vez.
		var curve int
		var saved int64
		if err := db.QueryRow(ctx, `SELECT xp_curve, xp FROM characters WHERE id = $1`, ch.ID).Scan(&curve, &saved); err != nil {
			t.Fatal(err)
		}
		if curve != game.XPCurve || saved != got.XP {
			t.Fatalf("nível %d: salvo na curva %d com %d de XP", c.level, curve, saved)
		}
		again, err := s.CharacterForAccount(ctx, acc, ch.ID)
		if err != nil || again.XP != got.XP {
			t.Fatalf("segunda carga: %d (antes %d), %v", again.XP, got.XP, err)
		}
	}

	// Personagem novo já nasce na curva de hoje.
	acc, _ = s.LoginIdentity(ctx, "dev", "Novata", "Novata")
	c, err := s.CreateCharacter(ctx, acc, "Novata", "mage", game.DefaultLook("mage"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveStates(ctx, []store.CharacterState{{CharacterID: c.ID, Level: 33, XP: game.XPLevelStart(33) + 9, HP: -1, MP: -1, Seq: 1}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.CharacterForAccount(ctx, acc, c.ID)
	if err != nil || got.XP != game.XPLevelStart(33)+9 {
		t.Fatalf("personagem novo: %d, %v", got.XP, err)
	}
}

// Deploy: o servidor antigo, ao desligar, grava a XP da curva 1 (com o nível certo) numa
// linha que o novo já converteu e marcou na curva 2. A carga confere a XP contra o nível
// salvo: converte de novo a XP da curva 1, não mexe na XP legítima e prende no início do
// nível o que não cabe em nenhuma curva, com um evento de invariante.
func TestCharacterXPGuardAgainstOldServerSave(t *testing.T) {
	s, url := storetest.NewWithURL(t)
	ctx := context.Background()
	db, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	acc, err := s.LoginIdentity(ctx, "dev", "deploy", "deploy")
	if err != nil {
		t.Fatal(err)
	}
	acc2, err := s.LoginIdentity(ctx, "dev", "deploy2", "deploy2")
	if err != nil {
		t.Fatal(err)
	}
	// XP da curva 1 (docs/progressao.md de antes): início do 30 + 50 mil, meio do 15.
	const v1At30, v1Mid15 = 1_839_820 + 50_000, 129_930 + 15_469
	legit := game.XPLevelStart(33) + game.XPToNext(33)/3
	cases := []struct {
		acc         int64
		name        string
		level       int
		xp, wantXP  int64
		wantInvalid bool
	}{
		{acc, "Trinta", 30, v1At30, game.XPLevelStart(30), false},
		{acc, "Quinze", 15, v1Mid15, game.ConvertXP(v1Mid15, 1), false},
		{acc, "Legitima", 33, legit, legit, false},
		{acc2, "Torta", 36, 5_000_000, game.XPLevelStart(36), true},
	}
	for _, c := range cases {
		ch, err := s.CreateCharacter(ctx, c.acc, c.name, "rogue", game.DefaultLook("rogue"))
		if err != nil {
			t.Fatal(err)
		}
		// O save do servidor antigo: level e xp, sem tocar em xp_curve (que já está em 2).
		if err := s.SaveStates(ctx, []store.CharacterState{{CharacterID: ch.ID, Level: c.level, XP: c.xp, HP: -1, MP: -1, Seq: 1}}); err != nil {
			t.Fatal(err)
		}
		got, err := s.CharacterForAccount(ctx, c.acc, ch.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.XP != c.wantXP || got.Level != c.level || game.LevelForXP(got.XP) != c.level {
			t.Fatalf("%s: XP %d, nível %d (pela XP %d), quero XP %d no %d", c.name, got.XP, got.Level, game.LevelForXP(got.XP), c.wantXP, c.level)
		}
		if c.name == "Quinze" {
			frac := float64(got.XP-game.XPLevelStart(15)) / float64(game.XPToNext(15))
			if math.Abs(frac-0.5) > 0.01 {
				t.Fatalf("meio do 15 virou %.3f do nível", frac)
			}
		}
		var saved int64
		var curve int
		if err := db.QueryRow(ctx, `SELECT xp, xp_curve FROM characters WHERE id = $1`, ch.ID).Scan(&saved, &curve); err != nil {
			t.Fatal(err)
		}
		if saved != c.wantXP || curve != game.XPCurve {
			t.Fatalf("%s: salvo %d na curva %d", c.name, saved, curve)
		}
		var n int
		if err := db.QueryRow(ctx, `SELECT count(*) FROM game_events WHERE kind = 'invariant' AND ref = 'xp_curve' AND character_id = $1`, ch.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if (n == 1) != c.wantInvalid || n > 1 {
			t.Fatalf("%s: %d eventos de invariante", c.name, n)
		}
	}
}
