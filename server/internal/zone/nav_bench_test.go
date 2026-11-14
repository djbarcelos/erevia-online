package zone

import (
	"io"
	"log/slog"
	"math"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/world"
)

// Custo dos caminhos longos (o "Ir" das missões e o clique no mapa do mundo).
// go test ./internal/zone -run x -bench 'FindPathLong|TickLongPaths' -benchmem

// BenchmarkFindPathLong: um caminho só, do A* de 1 m, em rotas que atravessam o mapa.
func BenchmarkFindPathLong(b *testing.B) {
	w, err := world.Load()
	if err != nil {
		b.Fatal(err)
	}
	routes := []struct {
		name     string
		from, to world.Point
	}{
		{"praça-vale", world.Point{X: 2, Z: 2}, world.Point{X: 0, Z: -270}},
		{"nordeste-fundo-do-vale", world.Point{X: 120, Z: 120}, world.Point{X: -10, Z: -300}},
		{"ermos-campos", world.Point{X: -128, Z: 5}, world.Point{X: 130, Z: 100}},
		{"praça-campos", world.Point{X: 0, Z: 0}, world.Point{X: 5, Z: 94}},
	}
	for _, r := range routes {
		b.Run(r.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				w.FindPath(r.from, r.to)
			}
		})
	}
}

// BenchmarkTickLongPaths: o tick com 50 jogadores na cidade e os monstros padrão.
// "parados" é a base; "1x-por-segundo" é o "Ir" de todos ao mesmo tempo, repetido a cada
// segundo (o cliente reenvia no máximo a cada 3 s); "todo-tick" é o pior caso, 50 pedidos
// longos por tick (o limite de taxa deixa até 10/s por jogador). ns/op = um tick em média;
// ms-pior-tick = o tick mais lento (o que importa: o pico quando todos pedem juntos).
func BenchmarkTickLongPaths(b *testing.B) {
	for _, c := range []struct {
		name  string
		every uint32 // ticks entre pedidos (0 = nunca)
	}{{"parados", 0}, {"1x-por-segundo", 20}, {"todo-tick", 1}} {
		b.Run(c.name, func(b *testing.B) {
			z := newRealZone(b)
			z.cfg.Spawns = game.DefaultSpawns()
			z = New(z.cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
			for i := 0; i < 50; i++ {
				a := float64(i) * 2 * math.Pi / 50
				z.handle(joinEvent{info: JoinInfo{ID: uint32(i + 1), Name: "Bot", Class: "knight", X: 20 * math.Cos(a), Z: 20 * math.Sin(a), Peer: discardPeer{}}})
			}
			var worst time.Duration
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				t0 := time.Now()
				if c.every > 0 && uint32(i)%c.every == 0 {
					for _, p := range z.players {
						p.pl.moveTokens = z.cfg.MaxMovesPerSec
						// Para o vale, do lado oposto ao que está: a muralha sempre no meio.
						z.handle(moveEvent{id: p.id, x: -p.pos.X * 0.2, z: -280})
					}
				}
				z.update(0.05)
				worst = max(worst, time.Since(t0))
			}
			b.ReportMetric(float64(worst.Microseconds())/1000, "ms-pior-tick")
		})
	}
}
