package store_test

import (
	"context"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

// Mede as consultas dos painéis do portal com uma semana de telemetria sintética
// (ADMIN_PERF_ROWS linhas em game_events, padrão 1 milhão, 2000 personagens). Cada uma tem
// de levar menos de 200 ms (a mediana de 5 rodadas).
//
//	ADMIN_PERF=1 go test ./internal/store -run TestAdminPanelsPerf -v
func TestAdminPanelsPerf(t *testing.T) {
	if os.Getenv("ADMIN_PERF") == "" {
		t.Skip("ADMIN_PERF vazio: pulando a medição com 1 milhão de eventos")
	}
	rows := 1_000_000
	if v, err := strconv.Atoi(os.Getenv("ADMIN_PERF_ROWS")); err == nil && v > 0 {
		rows = v
	}
	s, url := storetest.NewWithURL(t)
	db := rawDB(t, url)
	ctx := context.Background()

	start := time.Now()
	// 2000 contas e personagens, com 10 pilhas de itens cada e algum ouro.
	mustExec(t, db, `INSERT INTO accounts (stored_gold) SELECT (g * 37) % 5000 FROM generate_series(1, 2000) g`)
	mustExec(t, db, `INSERT INTO characters (account_id, name, class, level, gold, kit_given, last_seen_at)
		SELECT id, 'Bot' || id, (ARRAY['knight','barbarian','mage','rogue'])[1 + id % 4], 1 + id % 30, (id * 53) % 9000, true,
		       now() - (id % 20) * interval '1 day'
		FROM accounts`)
	mustExec(t, db, `INSERT INTO character_items (character_id, place, slot, item_id, count)
		SELECT c.id, 0, s, (ARRAY['potion_hp_s','ring_good','sword_common','bone_old','potion_mp_s'])[1 + s % 5], 1 + s
		FROM characters c, generate_series(0, 9) s`)
	// Uma semana de eventos: o mesmo perfil do jogo (abates, ouro, itens, FPS, sessões…).
	mustExec(t, db, `
		INSERT INTO game_events (at, kind, character_id, account_id, level, x, z, ref, amount, extra)
		SELECT at, kind, c, c, 1 + c % 30, x, z,
		  CASE kind
		    WHEN 'kill' THEN 'skeleton_minion'
		    WHEN 'gold' THEN (ARRAY['loot','sell','quest','buy','craft'])[1 + g % 5]
		    WHEN 'item_in' THEN (ARRAY['potion_hp_s','ring_good','sword_common','bone_old'])[1 + g % 4]
		    WHEN 'item_out' THEN (ARRAY['potion_hp_s','ring_good','sword_common','bone_old'])[1 + g % 4]
		    WHEN 'session' THEN CASE WHEN g % 2 = 0 THEN 'start' ELSE 'end' END
		    WHEN 'death' THEN 'monster:skeleton_minion'
		    WHEN 'fps' THEN ''
		    WHEN 'level' THEN ''
		    ELSE 'grupo' || (g % 25)
		  END,
		  CASE kind
		    WHEN 'gold' THEN CASE WHEN g % 5 >= 3 THEN -1 ELSE 1 END * (1 + (r2 * 120)::int)
		    WHEN 'item_in' THEN 1 + g % 3
		    WHEN 'item_out' THEN 1
		    WHEN 'fps' THEN 25 + (r2 * 40)::int
		    WHEN 'session' THEN CASE WHEN g % 2 = 1 THEN 300 + (r2 * 3000)::int ELSE 0 END
		    WHEN 'level' THEN 1 + c % 30
		    WHEN 'kill' THEN 1 + c % 30
		    ELSE 1 + g % 4
		  END,
		  CASE kind
		    WHEN 'item_in' THEN jsonb_build_object('how', (ARRAY['loot','buy','craft','quest'])[1 + g % 4])
		    WHEN 'item_out' THEN jsonb_build_object('how', (ARRAY['sell','destroy','craft'])[1 + g % 3])
		    WHEN 'fps' THEN jsonb_build_object('fps_low', 15 + (r2 * 20)::int, 'device', (ARRAY['desktop windows','desktop mac','mobile android','mobile ios'])[1 + g % 4], 'seconds', 60)
		    WHEN 'server_error' THEN jsonb_build_object('where', 'msg', 'panic', 'nil map', 'version', 'abc1234', 'stack', repeat('x', 400))
		    WHEN 'slow_tick' THEN jsonb_build_object('systems', 'ai 30ms', 'version', 'abc1234')
		    WHEN 'db_error' THEN jsonb_build_object('err', 'timeout', 'version', 'abc1234')
		    WHEN 'invariant' THEN jsonb_build_object('value', '-1', 'version', 'abc1234')
		  END
		FROM (
		  SELECT g, (1 + (g::bigint * 7919) % 2000)::int AS c, random() AS r2,
		         now() - random() * interval '7 days' AS at,
		         (random() * 300 - 150)::real AS x, (random() * 480 - 330)::real AS z,
		         CASE WHEN r < .35 THEN 'kill' WHEN r < .57 THEN 'gold' WHEN r < .67 THEN 'item_in' WHEN r < .72 THEN 'item_out'
		              WHEN r < .84 THEN 'fps' WHEN r < .92 THEN 'session' WHEN r < .95 THEN 'level' WHEN r < .98 THEN 'death'
		              WHEN r < .985 THEN 'server_error' WHEN r < .99 THEN 'slow_tick' WHEN r < .995 THEN 'db_error' ELSE 'invariant' END AS kind
		  FROM (SELECT g, random() AS r FROM generate_series(1, $1) g) gs
		) t`, rows)
	mustExec(t, db, `VACUUM ANALYZE game_events`)
	mustExec(t, db, `ANALYZE`)
	var n int64
	db.QueryRow(ctx, `SELECT count(*) FROM game_events`).Scan(&n)
	t.Logf("dados sintéticos: %d eventos, 2000 personagens, em %s", n, time.Since(start).Round(time.Millisecond))

	now := time.Now()
	today := store.DayStart(now)
	week := today.AddDate(0, 0, -6)
	day := today.AddDate(0, 0, -3) // um dia inteiro no meio da semana (~1/7 dos eventos)
	next := day.Add(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)
	char, err := s.FindCharacter(ctx, "Bot42")
	if err != nil {
		t.Fatal(err)
	}
	type perDayFn func(ctx context.Context, from, to time.Time) error
	wrap := func(f func(context.Context, time.Time, time.Time) error) perDayFn { return f }
	daily := []struct {
		name string
		fn   perDayFn
	}{
		{"gold (v_gold_daily)", wrap(func(c context.Context, a, b time.Time) error { _, err := s.GoldDaily(c, a, b); return err })},
		{"deaths (v_deaths_by_region)", wrap(func(c context.Context, a, b time.Time) error { _, err := s.DeathsDaily(c, a, b); return err })},
		{"fps (v_fps)", wrap(func(c context.Context, a, b time.Time) error { _, err := s.FPSDaily(c, a, b); return err })},
		{"items (v_items_daily)", wrap(func(c context.Context, a, b time.Time) error { _, err := s.ItemsDaily(c, a, b); return err })},
		{"itemsByItem (mais movimentados)", wrap(func(c context.Context, a, b time.Time) error { _, err := s.ItemsByItemDaily(c, a, b); return err })},
		{"server (erros do servidor)", wrap(func(c context.Context, a, b time.Time) error { _, err := s.ServerEventsDaily(c, a, b); return err })},
		{"players (sessões)", wrap(func(c context.Context, a, b time.Time) error { _, err := s.PlayersDaily(c, a, b); return err })},
		{"spikes (picos)", wrap(func(c context.Context, a, b time.Time) error { _, err := s.SpikeDays(c, a, b); return err })},
	}
	whole := []struct {
		name string
		fn   func() error
	}{
		{"levels (v_level_times, 7 dias)", func() error { _, err := s.LevelTimes(ctx, week); return err }},
		{"quit (v_quit_level)", func() error { _, err := s.QuitLevel(ctx); return err }},
		{"goldBefore (ouro até hoje, 1×/dia)", func() error { _, err := s.GoldTotalBefore(ctx, today); return err }},
		{"itemsBefore (itens até hoje, 1×/dia)", func() error { _, _, err := s.ItemTotalsBefore(ctx, today); return err }},
		{"supply (ouro de hoje + bolsas)", func() error {
			_, err := s.GoldSupply(ctx, store.TelemetryTotals{Cut: today})
			return err
		}},
		{"itemBalances (itens de hoje + bolsas)", func() error {
			_, err := s.ItemBalances(ctx, store.TelemetryTotals{Cut: today})
			return err
		}},
		{"character (histórico, 7 dias)", func() error { _, _, err := s.CharacterHistory(ctx, char.ID, week); return err }},
		{"bugs", func() error { _, err := s.ListBugs(ctx, store.BugFilter{}); return err }},
		{"client-errors", func() error { _, err := s.TopClientErrors(ctx, "", 7, 40); return err }},
		{"gm-log", func() error { _, err := s.GMLog(ctx, store.GMLogFilter{Query: "x"}); return err }},
	}
	const budget = 200 * time.Millisecond
	measure := func(name string, fn func() error) {
		t.Helper()
		var runs []time.Duration
		for range 5 {
			t0 := time.Now()
			if err := fn(); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			runs = append(runs, time.Since(t0))
		}
		slices.Sort(runs)
		t.Logf("%-46s melhor %7.1f ms  mediana %7.1f ms  pior %7.1f ms", name,
			ms(runs[0]), ms(runs[len(runs)/2]), ms(runs[len(runs)-1]))
		if runs[len(runs)/2] > budget {
			t.Errorf("%s: mediana %s > %s", name, runs[len(runs)/2], budget)
		}
	}
	for _, q := range daily {
		measure(q.name+": um dia", func() error { return q.fn(ctx, day, next) })
		measure(q.name+": hoje", func() error { return q.fn(ctx, today, tomorrow) })
	}
	for _, q := range whole {
		measure(q.name, q.fn)
	}
	// A rodada fria (sem nenhum dia guardado): os 7 dias, um de cada vez, como a API faz na
	// primeira abertura do painel. Só informativo: o portal faz isso uma vez por dia.
	for _, q := range daily {
		t0 := time.Now()
		for d := week; d.Before(tomorrow); d = d.Add(24 * time.Hour) {
			if err := q.fn(ctx, d, d.Add(24*time.Hour)); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("rodada fria de 7 dias, %-32s %7.1f ms (8 consultas)", q.name, ms(time.Since(t0)))
	}
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
