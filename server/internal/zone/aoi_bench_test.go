package zone

import (
	"io"
	"log/slog"
	"math"
	"testing"

	"erevia/server/internal/game"
)

// discardPeer aceita tudo sem guardar (para medir o custo do tick, não do teste).
type discardPeer struct{}

func (discardPeer) Send([]byte) bool { return true }
func (discardPeer) Close(string)     {}

// BenchmarkTickCrowd: 400 jogadores andando no anel de caça com os monstros padrão.
// go test ./internal/zone -run x -bench TickCrowd -cpuprofile /tmp/cpu.out
func BenchmarkTickCrowd(b *testing.B) {
	cfg := testConfig()
	cfg.Spawns = game.DefaultSpawns()
	z := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	for i := 0; i < 400; i++ {
		a := float64(i) * 2 * math.Pi / 400
		r := 45 + float64(i%30)
		z.handle(joinEvent{info: JoinInfo{ID: uint32(i + 1), Name: "Bot", Class: "knight", X: r * math.Cos(a), Z: r * math.Sin(a), Peer: discardPeer{}}})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range z.players {
			if p.dest == p.pos { // anda sem parar pelo anel
				a := math.Atan2(p.pos.Z, p.pos.X) + 0.3
				p.dest = Vec2{X: 60 * math.Cos(a), Z: 60 * math.Sin(a)}
				p.moved = true
			}
		}
		z.update(0.05)
	}
}
