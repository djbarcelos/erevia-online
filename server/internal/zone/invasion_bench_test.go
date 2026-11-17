package zone

import (
	"io"
	"log/slog"
	"math"
	"testing"

	"erevia/server/internal/game"
)

// BenchmarkTickCrowdInvasion: o cenário do BenchmarkTickCrowd (400 jogadores, monstros
// padrão), com 100 deles parados na praça e nas estradas, sem e com uma invasão no limite
// de vivos. A diferença entre os dois é o custo da invasão no tick.
// go test ./internal/zone -run x -bench 'TickCrowd'
func BenchmarkTickCrowdInvasion(b *testing.B) {
	for _, tc := range []struct {
		name string
		on   bool
	}{{"sem", false}, {"com", true}} {
		b.Run(tc.name, func(b *testing.B) {
			cfg := testConfig()
			cfg.Spawns = game.DefaultSpawns()
			z := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
			for i := 0; i < 400; i++ {
				a := float64(i) * 2 * math.Pi / 400
				r := 45 + float64(i%30)
				if i%4 == 0 {
					r = 3 + float64(i%12) // na praça e nas estradas
				}
				z.handle(joinEvent{info: JoinInfo{ID: uint32(i + 1), Name: "Bot", Class: "knight", X: r * math.Cos(a), Z: r * math.Sin(a), XP: game.XPLevelStart(12), Peer: discardPeer{}}})
			}
			tick := func() {
				if tc.on {
					if z.inv == nil {
						z.startInvasion(0, "bench")
					}
					z.inv.at = z.now + 60 // nunca acaba por tempo
				}
				for _, p := range z.players {
					if p.dead {
						z.respawnPlayer(p)
					}
					if p.dest == p.pos && math.Hypot(p.pos.X, p.pos.Z) > 40 { // o anel anda sem parar
						a := math.Atan2(p.pos.Z, p.pos.X) + 0.3
						p.dest = Vec2{X: 60 * math.Cos(a), Z: 60 * math.Sin(a)}
						p.moved = true
					}
				}
				z.update(0.05)
			}
			for range 20 * 90 { // 90 s: a 2ª onda já encheu o limite de vivos
				tick()
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tick()
			}
			b.ReportMetric(float64(len(z.invaders())), "invasores")
		})
	}
}
