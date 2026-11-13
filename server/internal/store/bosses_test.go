package store_test

import (
	"context"
	"testing"
	"time"

	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

func TestBossRespawnRoundTrip(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	if got, err := s.LoadBosses(ctx); err != nil || len(got) != 0 {
		t.Fatalf("no bosses saved yet: %v %v", got, err)
	}
	killed := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	b := store.Boss{ID: "ozrhal", RespawnAt: killed.Add(3*time.Hour + 20*time.Minute), KilledAt: killed, TopName: "grupo de Ana"}
	if err := s.SaveBoss(ctx, b); err != nil {
		t.Fatal(err)
	}
	// Morre de novo: a linha é trocada, não duplicada.
	b.KilledAt = killed.Add(5 * time.Hour)
	b.RespawnAt = b.KilledAt.Add(3 * time.Hour)
	b.TopName = "clã Lobos"
	if err := s.SaveBoss(ctx, b); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadBosses(ctx)
	if err != nil || len(got) != 1 {
		t.Fatalf("one row per boss: %v %v", got, err)
	}
	g := got[0]
	if g.ID != "ozrhal" || !g.RespawnAt.Equal(b.RespawnAt) || !g.KilledAt.Equal(b.KilledAt) || g.TopName != "clã Lobos" {
		t.Fatalf("round trip: %+v", g)
	}
}
