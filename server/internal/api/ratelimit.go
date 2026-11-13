package api

import (
	"math"
	"sync"
	"time"
)

// limiter é um balde de fichas por conta: cabe "burst" pedidos de uma vez e o balde
// enche "rate" fichas por segundo. Fica em memória (um servidor só).
type limiter struct {
	mu    sync.Mutex
	rate  float64
	burst float64
	now   func() time.Time
	m     map[int64]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

// pruneAt: acima disso, a próxima chamada joga fora os baldes que já encheram de novo.
const pruneAt = 1024

func newLimiter(rate, burst float64) *limiter {
	return &limiter{rate: rate, burst: burst, now: time.Now, m: map[int64]*bucket{}}
}

// allow gasta uma ficha da conta. Sem ficha, diz quanto falta para a próxima.
func (l *limiter) allow(key int64) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.m) >= pruneAt {
		for k, b := range l.m {
			if b.tokens+now.Sub(b.last).Seconds()*l.rate >= l.burst {
				delete(l.m, k)
			}
		}
	}
	b := l.m[key]
	if b == nil {
		b = &bucket{tokens: l.burst, last: now}
		l.m[key] = b
	}
	b.tokens = math.Min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
	b.last = now
	if b.tokens < 1 {
		return false, time.Duration((1 - b.tokens) / l.rate * float64(time.Second))
	}
	b.tokens--
	return true, 0
}
