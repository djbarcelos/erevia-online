package zone

import (
	"io"
	"log/slog"
	"math"
	"testing"

	"erevia/server/internal/game"
)

// BenchmarkTickAbilities: 50 golens brigando, cada um com 2 jogadores (100 no total): sem
// habilidades, com a pancada de verdade (recarga de 10 s) e no pior caso (recarga logo
// depois do golpe). A diferença é o custo por tick dos avisos.
// go test ./internal/zone -run x -bench TickAbilities -benchmem
func BenchmarkTickAbilities(b *testing.B) {
	golem := game.Monsters["stone_golem"]
	var worst []game.AbilityDef // recarga logo depois do golpe: ~50 avisos sempre no ar
	for _, a := range golem.Abilities {
		a.Cooldown, a.First = a.Delay+0.1, 0
		worst = append(worst, a)
	}
	for _, c := range []struct {
		name      string
		abilities []game.AbilityDef
	}{{"sem", nil}, {"com", golem.Abilities}, {"pior", worst}} {
		b.Run(c.name, func(b *testing.B) {
			saved := golem.Abilities
			golem.Abilities = c.abilities
			defer func() { golem.Abilities = saved }()
			cfg := testConfig()
			cfg.SafeRadius = 20
			cfg.Spawns = nil // só os golens
			for i := range 50 {
				a := float64(i) * 2 * math.Pi / 50
				cfg.Spawns = append(cfg.Spawns, fixed("stone_golem", 25, 80*math.Cos(a), 80*math.Sin(a)))
			}
			z := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
			id := uint32(1)
			for _, m := range z.monsters {
				for k := range 2 {
					z.handle(joinEvent{info: JoinInfo{ID: id, Name: "Bot", Class: "knight", X: m.pos.X + 2.5, Z: m.pos.Z + float64(k), Peer: discardPeer{}}})
					p := z.players[id]
					p.stats.MaxHP, p.hp = 1e9, 1e9
					m.mon.hate[id] = float64(k + 1)
					id++
				}
			}
			b.ResetTimer()
			for range b.N {
				for _, p := range z.players {
					p.hp = p.stats.MaxHP
				}
				z.update(0.05)
			}
			b.StopTimer()
			b.ReportMetric(float64(len(z.telegraphs)), "avisos")
			b.ReportMetric(float64(z.telegraphSeq)/float64(b.N), "avisos/tick")
		})
	}
}
