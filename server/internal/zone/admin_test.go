package zone

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAdminStatusGoesThroughZone(t *testing.T) {
	z := newTestZone(make(chan []SavedState, 16)) // ao parar, a zona salva quem está online
	join(z, 1, "Ana")
	join(z, 2, "Beto")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { z.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	st, err := z.Status(context.Background())
	if err != nil || st.Online != 2 {
		t.Fatalf("Status = %+v, %v; want 2 online", st, err)
	}
}

func TestAdminStatusTimesOutWithoutZone(t *testing.T) {
	z := newTestZone(nil) // sem Run: ninguém responde
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := z.Status(ctx); !errors.Is(err, ErrZoneBusy) {
		t.Fatalf("err = %v, want ErrZoneBusy", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("did not respect the caller's deadline")
	}
	// A fila cheia também não trava quem pede.
	for len(z.events) < cap(z.events) {
		z.events <- socialDone{fn: func() {}}
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()
	if _, err := z.Status(ctx2); !errors.Is(err, ErrZoneBusy) {
		t.Fatalf("full queue: err = %v, want ErrZoneBusy", err)
	}
}
