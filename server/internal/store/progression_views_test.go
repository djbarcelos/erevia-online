package store_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

// Views da progressão (00018): eventos sintéticos de três personagens e as contas feitas
// à mão.
//
//	knight: sessão 0–20 min (sobe ao 2 aos 10), sessão 60–80 min (sobe ao 3 aos 70)
//	        → nível 1 em 10 min de jogo; nível 2 em 20 min de jogo (1 h de relógio)
//	mage:   sessão 0–? (sobe ao 2 aos 30, servidor caiu: último evento aos 40), sessão
//	        120–125 min → nível 1 em 30 min; nível 2 ainda aberto (15 min até agora)
//	rogue:  já existia: primeira sessão no nível 5, sobe ao 6 aos 5 min, sai aos 10
//	        → o nível 5 não começou na telemetria (fica fora do tempo por nível)
func TestProgressionViews(t *testing.T) {
	s, url := storetest.NewWithURL(t)
	ctx := context.Background()
	db, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(ctx) })

	char := func(sub, name, class string) (int64, int64) {
		acc, err := s.LoginIdentity(ctx, "google", sub, name)
		if err != nil {
			t.Fatal(err)
		}
		c, err := s.CreateCharacter(ctx, acc, name, class, game.DefaultLook(class))
		if err != nil {
			t.Fatal(err)
		}
		return c.ID, acc
	}
	kn, knAcc := char("sub-kn", "Cavaleira", "knight")
	mg, mgAcc := char("sub-mg", "Maga", "mage")
	rg, rgAcc := char("sub-rg", "Ladino", "rogue")

	t0 := time.Now().Add(-72 * time.Hour).Truncate(time.Second)
	at := func(m int) time.Time { return t0.Add(time.Duration(m) * time.Minute) }
	var evs []store.GameEvent
	ev := func(id, acc int64, at time.Time, kind string, level int, ref string, amount int64) {
		evs = append(evs, store.GameEvent{At: at, Kind: kind, CharacterID: id, AccountID: acc, Level: level, Ref: ref, Amount: amount})
	}
	// Cavaleira.
	ev(kn, knAcc, at(0), "session", 1, "start", 0)
	ev(kn, knAcc, at(2), "kill", 1, "skeleton_minion", 1)
	ev(kn, knAcc, at(3), "kill", 1, "skeleton_minion", 1)
	ev(kn, knAcc, at(4), "kill", 1, "skeleton_minion", 1)
	ev(kn, knAcc, at(5), "death", 1, "monster:skeleton_minion", 0)
	ev(kn, knAcc, at(6), "gold", 1, "loot", 950)
	ev(kn, knAcc, at(7), "gold", 1, "buy", -190)
	ev(kn, knAcc, at(8), "gold", 1, "gm", 100000)
	ev(kn, knAcc, at(9), "xp", 1, "monster", 300)
	ev(kn, knAcc, at(9), "xp", 1, "event_bonus", 100)
	ev(kn, knAcc, at(9), "xp", 1, "quest", 100)
	ev(kn, knAcc, at(9), "xp", 1, "death_loss", -50)
	ev(kn, knAcc, at(10), "level", 2, "", 2)
	ev(kn, knAcc, at(20), "session", 2, "end", 1200)
	ev(kn, knAcc, at(60), "session", 2, "start", 0)
	ev(kn, knAcc, at(70), "level", 3, "", 3)
	ev(kn, knAcc, at(80), "session", 3, "end", 1200)
	ev(kn, knAcc, at(80), "xp", 25, "party", 1000) // só para a faixa 20–29
	// Maga.
	ev(mg, mgAcc, at(0), "session", 1, "start", 0)
	ev(mg, mgAcc, at(1), "kill", 1, "skeleton_minion", 1)
	ev(mg, mgAcc, at(30), "level", 2, "", 2)
	ev(mg, mgAcc, at(40), "kill", 2, "skeleton_minion", 2) // último sinal antes da queda
	ev(mg, mgAcc, at(120), "session", 2, "start", 0)
	ev(mg, mgAcc, at(125), "session", 2, "end", 300)
	// Ladino.
	ev(rg, rgAcc, at(0), "session", 5, "start", 0)
	ev(rg, rgAcc, at(5), "level", 6, "", 6)
	ev(rg, rgAcc, at(10), "session", 6, "end", 600)
	if err := s.InsertEvents(ctx, evs); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE characters SET level = CASE id WHEN $1 THEN 3 WHEN $2 THEN 2 ELSE 6 END,
		last_seen_at = CASE id WHEN $3 THEN now() - interval '10 days' END`, kn, mg, rg); err != nil {
		t.Fatal(err)
	}

	// Cada consulta devolve uma linha de texto (colunas separadas por "|").
	checks := []struct{ name, sql, want string }{
		// Tempo de jogo por nível: mediana e p75 das sessões, não do relógio.
		{"nível 1 total", `SELECT concat_ws('|', personagens, minutos_media, minutos_mediana, minutos_p75) FROM v_level_playtime WHERE nivel = 1 AND classe = 'todas'`, "2|20.0|20.0|25.0"},
		{"nível 1 cavaleira", `SELECT concat_ws('|', personagens, minutos_mediana) FROM v_level_playtime WHERE nivel = 1 AND classe = 'knight'`, "1|10.0"},
		{"nível 2 cavaleira", `SELECT concat_ws('|', minutos_mediana, horas_relogio_mediana) FROM v_level_playtime WHERE nivel = 2 AND classe = 'knight'`, "20.0|1.0"},
		{"sem nível incompleto", `SELECT count(*)::text FROM v_level_playtime WHERE nivel IN (3, 5, 6)`, "0"},
		// Sessão sem fim (servidor caiu): vai até o último evento antes da próxima.
		{"sessão caída", fmt.Sprintf(`SELECT (fim - inicio)::text FROM v_sessions WHERE character_id = %d AND NOT fechada`, mg), "00:40:00"},
		// XP por fonte e faixa (pct da XP ganha na faixa).
		{"xp monstro", `SELECT concat_ws('|', xp, pct) FROM v_xp_sources WHERE faixa = '01-09' AND fonte = 'monster'`, "300|60.0"},
		{"xp morte", `SELECT concat_ws('|', xp, pct) FROM v_xp_sources WHERE faixa = '01-09' AND fonte = 'death_loss'`, "-50|-10.0"},
		{"xp grupo", `SELECT pct::text FROM v_xp_sources WHERE faixa = '20-29' AND fonte = 'party'`, "100.0"},
		// Abates e mortes por nível.
		{"abates nível 1", `SELECT concat_ws('|', abates, mortes, personagens, abates_por_personagem, abates_por_morte, xp_perdida_em_mortes) FROM v_level_kills_deaths WHERE nivel = 1`,
			"4|1|2|2.0|4.0|50"},
		// Ouro por hora: 5700 s jogados no 1–9 (40 + 45 + 10 min); o ouro de GM fica fora.
		{"ouro por hora", `SELECT concat_ws('|', horas_jogadas, ouro_ganho, ouro_gasto, ganho_por_hora, gasto_por_hora, saldo_por_hora) FROM v_gold_per_hour WHERE faixa = '01-09'`,
			"1.6|950|190|600|120|480"},
		// Funil (de characters).
		{"funil nível 1", `SELECT concat_ws('|', chegaram, estao_nele) FROM v_level_reached WHERE nivel = 1`, "3|0"},
		{"funil nível 3", `SELECT concat_ws('|', chegaram, pct_do_anterior) FROM v_level_reached WHERE nivel = 3`, "2|66.7"},
		{"funil nível 6", `SELECT concat_ws('|', chegaram, estao_nele, pararam_nele) FROM v_level_reached WHERE nivel = 6`, "1|1|1"},
		{"funil até o 30", `SELECT count(*)::text FROM v_level_reached`, "30"},
	}
	for _, c := range checks {
		var got string
		if err := db.QueryRow(ctx, c.sql).Scan(&got); err != nil {
			t.Errorf("%s: %v", c.name, err)
		} else if got != c.want {
			t.Errorf("%s = %s, quero %s", c.name, got, c.want)
		}
	}

	// As consultas prontas (queries/progressao.sql) rodam sobre estes dados sem erro.
	raw, err := os.ReadFile("queries/progressao.sql")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, q := range strings.Split(string(raw), ";\n") {
		var code []string
		for _, line := range strings.Split(q, "\n") {
			if l := strings.TrimSpace(line); l != "" && !strings.HasPrefix(l, "--") {
				code = append(code, line)
			}
		}
		if len(code) == 0 {
			continue
		}
		n++
		rows, err := db.Query(ctx, strings.Join(code, "\n"))
		if err == nil {
			for rows.Next() {
			}
			err = rows.Err()
		}
		if err != nil {
			t.Errorf("queries/progressao.sql, consulta %d: %v", n, err)
		}
	}
	if n < 9 {
		t.Fatalf("queries/progressao.sql: %d consultas, quero ao menos 9", n)
	}
}
