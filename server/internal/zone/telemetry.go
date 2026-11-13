package zone

import (
	"context"
	"time"

	"erevia/server/internal/pb"
)

// Telemetria: a zona anota acontecimentos (ouro, itens, abates, mortes, níveis,
// sessões e o FPS dos clientes) numa fila; um worker grava em lote no banco
// (game_events). Fila cheia descarta (nunca trava a simulação).

const (
	telemetryQueue = 8192
	telemetryBatch = 500
	telemetryEvery = 2 * time.Second
)

// GameEvent é um acontecimento para a tabela game_events.
type GameEvent struct {
	At          time.Time
	Kind        string
	CharacterID int64
	AccountID   int64
	Level       int
	X, Z        float64
	Ref         string
	Amount      int64
	Extra       map[string]any
}

// EventSink grava eventos (implementado com o store).
type EventSink interface {
	InsertEvents(ctx context.Context, events []GameEvent) error
}

// SetEventSink liga a telemetria ao banco (antes de Run). Sem ele, nada é anotado.
func (z *Zone) SetEventSink(s EventSink) { z.eventSink = s }

// track anota um acontecimento de um jogador (p pode ser nil).
func (z *Zone) track(p *entity, kind, ref string, amount int64, extra map[string]any) {
	if z.eventSink == nil {
		return
	}
	e := GameEvent{At: z.cfg.Clock(), Kind: kind, Ref: ref, Amount: amount, Extra: extra}
	if p != nil && p.pl != nil {
		e.CharacterID, e.AccountID, e.Level, e.X, e.Z = int64(p.id), p.pl.accountID, p.level, p.pos.X, p.pos.Z
	}
	select {
	case z.telemetry <- e:
	default:
		z.droppedEvents++
	}
}

// trackGold: moedas que entraram (+) ou saíram (−) do jogo, e de onde.
func (z *Zone) trackGold(p *entity, source string, amount int64) {
	if amount != 0 {
		z.track(p, "gold", source, amount, nil)
	}
}

// trackItem: itens que entraram (in) ou saíram (out) do jogo.
func (z *Zone) trackItem(p *entity, in bool, itemID string, count int, how string) {
	kind := "item_out"
	if in {
		kind = "item_in"
	}
	z.track(p, kind, itemID, int64(count), map[string]any{"how": how})
}

func (z *Zone) runTelemetryWorker(ctx context.Context) {
	if z.eventSink == nil {
		return
	}
	tick := time.NewTicker(telemetryEvery)
	defer tick.Stop()
	batch := make([]GameEvent, 0, telemetryBatch)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		cctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := z.eventSink.InsertEvents(cctx, batch); err != nil {
			z.log.Warn("telemetry insert", "err", err, "count", len(batch))
			z.ReportDBError("telemetry", err)
		}
		cancel()
		batch = batch[:0]
	}
	for {
		select {
		case <-ctx.Done():
			// Esvazia o que ainda está na fila antes de sair (e os erros ainda não gravados).
			batch = append(batch, z.errs.rep.drain(z.cfg.Clock())...)
			for {
				select {
				case e := <-z.telemetry:
					batch = append(batch, e)
				default:
					flush()
					return
				}
			}
		case e := <-z.telemetry:
			batch = append(batch, e)
			if len(batch) >= telemetryBatch {
				flush()
			}
		case <-tick.C:
			batch = append(batch, z.dueErrors()...) // contagens de erro acumuladas (errors.go)
			flush()
		}
	}
}

// onClientStats: amostra de FPS que o cliente manda a cada ~60 s.
func (z *Zone) onClientStats(p *entity, m *pb.ClientStats) {
	if m.FpsAvg <= 0 || m.FpsAvg > 500 {
		return
	}
	dev := m.Device
	if len(dev) > 40 {
		dev = dev[:40]
	}
	z.track(p, "fps", "", int64(m.FpsAvg+0.5), map[string]any{"fps_low": m.FpsLow, "device": dev, "seconds": m.Seconds})
}
