package zone

import (
	"context"
	"time"

	"erevia/server/internal/pb"
)

// Telemetria: a zona anota acontecimentos (ouro, itens, abates, mortes, níveis,
// sessões e o FPS dos clientes) numa fila; um worker grava em lote no banco
// (game_events). Fila cheia descarta (nunca trava a simulação). A XP por fonte é somada
// na zona e sai a cada minuto (kind "xp", xptally.go).
//
// Ao desligar, a zona anota o fim da sessão de quem ainda está online (endSessions) e só
// então o worker esvazia a fila; o cmd/zone espera o worker (TelemetryDone) antes de sair.
// Sem isso, todo deploy perderia o fim das sessões abertas e o tempo de jogo por nível
// (v_level_playtime) sairia menor.

const (
	telemetryQueue    = 8192
	telemetryBatch    = 500
	telemetryEvery    = 2 * time.Second
	telemetryStopWait = 5 * time.Second // espera da zona parar antes de esvaziar a fila
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

// endSessions: a zona vai parar; fecha a sessão (e a XP somada) de quem está online.
func (z *Zone) endSessions() {
	for _, p := range z.players {
		z.endXPTally(p)
		z.track(p, "session", "end", int64(z.cfg.Clock().Sub(p.pl.joinedAt).Seconds()), map[string]any{"shutdown": true})
	}
}

// TelemetryDone fecha quando o worker da telemetria gravou o que faltava ao desligar (ou
// logo, se não há banco).
func (z *Zone) TelemetryDone() <-chan struct{} { return z.telemetryDone }

func (z *Zone) runTelemetryWorker(ctx context.Context) {
	if z.eventSink == nil {
		z.telemetryOnce.Do(func() { close(z.telemetryDone) })
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
			// Espera a zona anotar o fim das sessões (Run) e esvazia o que ainda está na fila
			// antes de sair (e os erros ainda não gravados).
			select {
			case <-z.stopped:
			case <-time.After(telemetryStopWait):
			}
			batch = append(batch, z.errs.rep.drain(z.cfg.Clock())...)
			for {
				select {
				case e := <-z.telemetry:
					batch = append(batch, e)
				default:
					flush()
					z.telemetryOnce.Do(func() { close(z.telemetryDone) })
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
	// O extra vira JSONB num lote só: NaN/infinito (json.Marshal recusa) ou um NUL no
	// aparelho (o Postgres recusa \u0000) derrubavam o lote inteiro, com o ouro e os itens
	// de todo mundo (achado pelo teste de caos).
	if !(m.FpsAvg > 0 && m.FpsAvg <= 500) || !finite(float64(m.FpsLow)) {
		return
	}
	dev := cleanText(m.Device, 40)
	z.track(p, "fps", "", int64(m.FpsAvg+0.5), map[string]any{"fps_low": m.FpsLow, "device": dev, "seconds": m.Seconds})
}

// trackMastery: kind "mastery", ponto de Maestria ganho no nível 30 (ref "point", amount =
// a Maestria nova, extra.gained = pontos de uma vez).
func (z *Zone) trackMastery(p *entity, level, gained int) {
	z.track(p, "mastery", "point", int64(level), map[string]any{"gained": gained})
}
