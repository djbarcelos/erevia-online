package zone

import "time"

// tickStatsEvery: de quanto em quanto tempo a zona registra no log o custo do tick.
const tickStatsEvery = 30 * time.Second

// tickStats mede quanto tempo cada tick (update) leva. Um tick precisa caber com folga
// em 1/TickRate (50 ms a 20 ticks/s); perto disso, a zona começa a atrasar.
type tickStats struct {
	since  time.Time
	count  int
	total  time.Duration
	worst  time.Duration
	events int
	slow   int // ticks acima do orçamento (ver checkSlowTick)
}

func (s *tickStats) add(d time.Duration) {
	s.count++
	s.total += d
	s.worst = max(s.worst, d)
}

// flush registra a média e o pior tick do período e zera os contadores.
func (z *Zone) flushTickStats(now time.Time) {
	s := &z.stats
	if s.since.IsZero() {
		s.since = now
		return
	}
	if now.Sub(s.since) < tickStatsEvery || s.count == 0 {
		return
	}
	secs := now.Sub(s.since).Seconds()
	z.log.Info("tick stats",
		"avg_ms", float64(s.total.Microseconds())/float64(s.count)/1000,
		"max_ms", float64(s.worst.Microseconds())/1000,
		"budget_ms", 1000/float64(z.cfg.TickRate),
		"events_per_s", int(float64(s.events)/secs),
		"slow_ticks", s.slow,
		"players", len(z.players), "monsters", len(z.monsters), "ground", len(z.ground))
	*s = tickStats{since: now}
}
