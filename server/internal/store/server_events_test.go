package store_test

import (
	"context"
	"testing"
	"time"

	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

func TestServerEventsRoundTrip(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	if got, err := s.LoadServerEvents(ctx); err != nil || len(got) != 0 {
		t.Fatalf("no events saved yet: %v %v", got, err)
	}
	at := time.Date(2026, 11, 14, 18, 0, 0, 0, time.UTC)
	for _, e := range []store.ServerEvent{
		{ID: "xp2", Until: at},
		{ID: "xp2_off", Until: time.Time{}},
		{ID: "xp2", Until: at.Add(2 * time.Hour)}, // o GM mudou o prazo: a linha é trocada
	} {
		if err := s.SaveServerEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.LoadServerEvents(ctx)
	if err != nil || len(got) != 2 {
		t.Fatalf("one row per event: %v %v", got, err)
	}
	if got[0].ID != "xp2" || !got[0].Until.Equal(at.Add(2*time.Hour)) || got[1].ID != "xp2_off" || !got[1].Until.IsZero() {
		t.Fatalf("round trip: %+v", got)
	}
}
