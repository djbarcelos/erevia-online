package zone

import (
	"context"
	"math"
	"time"
)

// Painel de métricas do portal (/admin#metricas): o retrato da zona ao vivo. Tudo aqui
// roda na goroutine da zona (pelo askZone) e só conta o que já está em memória: nada de
// banco, alocação proporcional ao mapa ou ordenação. Os gráficos (telemetria) vêm do banco,
// direto pela API, sem passar pela zona.

// TickSnapshot é o custo do tick num período (o mesmo que o log "tick stats" registra).
type TickSnapshot struct {
	Since        time.Time `json:"since"`
	Until        time.Time `json:"until"`
	Ticks        int       `json:"ticks"`
	AvgMs        float64   `json:"avgMs"`
	WorstMs      float64   `json:"worstMs"`
	BudgetMs     float64   `json:"budgetMs"`
	SlowTicks    int       `json:"slowTicks"`
	EventsPerSec int       `json:"eventsPerSec"`
}

// snapshot resume o período corrente (sem zerar).
func (s *tickStats) snapshot(now time.Time, tickRate int) TickSnapshot {
	out := TickSnapshot{Since: s.since, Until: now, Ticks: s.count, SlowTicks: s.slow}
	if tickRate > 0 {
		out.BudgetMs = 1000 / float64(tickRate)
	}
	if s.count > 0 {
		out.AvgMs = round2(float64(s.total.Microseconds()) / float64(s.count) / 1000)
	}
	out.WorstMs = round2(float64(s.worst.Microseconds()) / 1000)
	if secs := now.Sub(s.since).Seconds(); secs > 0 && !s.since.IsZero() {
		out.EventsPerSec = int(float64(s.events) / secs)
	}
	return out
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// LiveStats é o que o painel "ao vivo" mostra.
type LiveStats struct {
	Online   int            `json:"online"`
	ByRegion map[string]int `json:"byRegion"` // jogadores por região (RegionAt)
	Monsters int            `json:"monsters"` // monstros vivos
	Dead     int            `json:"monstersDead"`
	Ground   int            `json:"ground"` // itens no chão
	// LastTick: o último período fechado de tickStatsEvery (30 s); nil antes do primeiro.
	LastTick *TickSnapshot `json:"lastTick"`
	// CurrentTick: o período ainda aberto (parcial, desde LastTick.Until).
	CurrentTick TickSnapshot `json:"currentTick"`
}

// LiveStats pergunta à zona o retrato de agora (pelo canal, com prazo).
func (z *Zone) LiveStats(ctx context.Context) (LiveStats, error) {
	return askZone(ctx, z, func(z *Zone) LiveStats {
		out := LiveStats{Online: len(z.players), ByRegion: map[string]int{}, Ground: len(z.ground)}
		for _, p := range z.players {
			out.ByRegion[RegionAt(p.pos.X, p.pos.Z)]++
		}
		for _, m := range z.monsters {
			if m.dead {
				out.Dead++
			} else {
				out.Monsters++
			}
		}
		if last := z.stats.last; last.Ticks > 0 {
			out.LastTick = &last
		}
		out.CurrentTick = z.stats.snapshot(time.Now(), z.cfg.TickRate) // o tick mede com o relógio de verdade
		return out
	})
}

// Regiões, na mesma ordem e com os mesmos anéis da função SQL erevia_region
// (migração 00011): o Vale fica ao sul, o resto são anéis em volta da cidade.
var Regions = []string{"Cidade", "Arredores", "Campos dos Ossos", "Ermos Partidos", "Vale das Pedras Vivas"}

// RegionAt é a região aproximada pela posição (igual a erevia_region no banco).
func RegionAt(x, z float64) string {
	r := math.Sqrt(x*x + z*z)
	switch {
	case z < -200:
		return "Vale das Pedras Vivas"
	case r < 40:
		return "Cidade"
	case r < 78:
		return "Arredores"
	case r < 112:
		return "Campos dos Ossos"
	default:
		return "Ermos Partidos"
	}
}
