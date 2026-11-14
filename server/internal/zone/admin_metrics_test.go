package zone

import (
	"context"
	"testing"
	"time"
)

func TestLiveStatsGoesThroughZone(t *testing.T) {
	z := newTestZone(make(chan []SavedState, 16))
	join(z, 1, "Ana")
	join(z, 2, "Beto")
	z.players[2].pos = Vec2{X: 90, Z: 0} // Campos dos Ossos
	// Um período de tick fechado, como o loop faz a cada 30 s.
	t0 := time.Now().Add(-tickStatsEvery) // fecha agora: o loop só fecha o próximo daqui a 30 s
	z.flushTickStats(t0)
	for range 10 {
		z.stats.add(4 * time.Millisecond)
	}
	z.stats.add(12 * time.Millisecond)
	z.stats.slow = 1
	z.flushTickStats(t0.Add(tickStatsEvery))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { z.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	st, err := z.LiveStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Online != 2 || st.ByRegion["Campos dos Ossos"] != 1 || sumRegions(st.ByRegion) != 2 {
		t.Fatalf("live = %+v", st)
	}
	if st.LastTick == nil || st.LastTick.Ticks != 11 || st.LastTick.WorstMs != 12 || st.LastTick.SlowTicks != 1 ||
		st.LastTick.AvgMs < 4.7 || st.LastTick.AvgMs > 4.8 || st.LastTick.BudgetMs != 50 {
		t.Fatalf("last tick = %+v", st.LastTick)
	}
}

// sumRegions soma os jogadores de todas as regiões.
func sumRegions(m map[string]int) (n int) {
	for _, v := range m {
		n += v
	}
	return n
}

func TestRegionAtMatchesSQL(t *testing.T) {
	// Mesmos pontos de store/events_test.go e dos anéis de erevia_region.
	for _, tc := range []struct {
		x, z float64
		want string
	}{
		{0, 0, "Cidade"}, {39, 0, "Cidade"}, {60, 5, "Arredores"}, {5, 94, "Campos dos Ossos"},
		{-128, 5, "Ermos Partidos"}, {0, -270, "Vale das Pedras Vivas"}, {0, -199, "Ermos Partidos"},
	} {
		if got := RegionAt(tc.x, tc.z); got != tc.want {
			t.Errorf("RegionAt(%v, %v) = %q, want %q", tc.x, tc.z, got, tc.want)
		}
	}
}
