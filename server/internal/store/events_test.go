package store_test

import (
	"context"
	"testing"
	"time"

	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

func TestInsertEventsAndDashboardViews(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	now := time.Now()
	evs := []store.GameEvent{
		{At: now, Kind: "gold", CharacterID: 1, Ref: "loot", Amount: 30},
		{At: now, Kind: "gold", CharacterID: 1, Ref: "buy", Amount: -20},
		{At: now.Add(-10 * time.Minute), Kind: "level", CharacterID: 1, Amount: 2},
		{At: now, Kind: "level", CharacterID: 1, Amount: 3},
		{At: now, Kind: "death", CharacterID: 1, X: 60, Z: 0, Ref: "monster:skeleton_minion", Level: 3},
		{At: now, Kind: "fps", CharacterID: 1, X: 0, Z: 0, Amount: 58, Extra: map[string]any{"fps_low": 41, "device": "desktop mac"}},
		{At: now, Kind: "item_in", CharacterID: 1, Ref: "ring_rare", Amount: 1, Extra: map[string]any{"how": "craft"}},
	}
	if err := s.InsertEvents(ctx, evs); err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{
		"v_gold_daily":       "SELECT count(*) FROM v_gold_daily WHERE origem IN ('loot', 'buy')",
		"v_level_times":      "SELECT count(*) FROM v_level_times WHERE nivel = 3 AND minutos_medios = 10",
		"v_deaths_by_region": "SELECT count(*) FROM v_deaths_by_region WHERE regiao = 'Arredores'",
		"v_fps":              "SELECT count(*) FROM v_fps WHERE regiao = 'Cidade' AND fps_medio = 58",
		"v_items_daily":      "SELECT count(*) FROM v_items_daily WHERE item = 'ring_rare' AND como = 'craft'",
		"v_quit_level":       "SELECT count(*) + 1 FROM v_quit_level",
	}
	for name, q := range checks {
		var n int
		if err := s.QueryRowForTest(ctx, q, &n); err != nil || n < 1 {
			t.Errorf("%s: n=%d err=%v", name, n, err)
		}
	}
}
