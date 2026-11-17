package zone

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

type nopSink struct{}

func (nopSink) InsertEvents(context.Context, []GameEvent) error { return nil }

// drainEvents tira tudo o que a zona anotou até agora.
func drainEvents(z *Zone) []GameEvent {
	var out []GameEvent
	for {
		select {
		case e := <-z.telemetry:
			out = append(out, e)
		default:
			return out
		}
	}
}

func findEvent(evs []GameEvent, kind, ref string) *GameEvent {
	for i := range evs {
		if evs[i].Kind == kind && (ref == "" || evs[i].Ref == ref) {
			return &evs[i]
		}
	}
	return nil
}

func TestTelemetryOffWithoutSink(t *testing.T) {
	z := newCombatZone()
	joinAt(z, 1, "knight", 0, 0, 0)
	if len(drainEvents(z)) != 0 {
		t.Fatal("no sink: nothing should be queued")
	}
}

func TestTelemetryTracksSessionsKillsLevelsAndDeaths(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 2, 25, 0))
	z.SetEventSink(nopSink{})
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "barbarian", 23.5, 0, game.XPLevelStart(2)-1)
	if e := findEvent(drainEvents(z), "session", "start"); e == nil || e.CharacterID != 1 {
		t.Fatal("join must record a session start")
	}
	m.hp = 1
	z.handle(attackEvent{id: p.id, target: m.id})
	for i := 0; i < 200 && !m.dead; i++ {
		z.update(0.05)
	}
	evs := drainEvents(z)
	if e := findEvent(evs, "kill", "skeleton_minion"); e == nil || e.Amount != 2 {
		t.Fatalf("kill event = %+v", e)
	}
	if findEvent(evs, "level", "") == nil {
		t.Fatal("level up must be recorded")
	}

	z.kill(p, m)
	if e := findEvent(drainEvents(z), "death", "monster:skeleton_minion"); e == nil {
		t.Fatal("death must record the killer")
	}
	z.removePlayer(p.id)
	if e := findEvent(drainEvents(z), "session", "end"); e == nil {
		t.Fatal("leaving must record a session end")
	}
}

func TestTelemetryTracksGoldAndItems(t *testing.T) {
	z, p, _, smith := atSmith(t)
	z.SetEventSink(nopSink{})
	drainEvents(z)
	p.pl.inv.gold = 5000
	p.pl.moveTokens = z.cfg.MaxMovesPerSec
	z.handle(itemEvent{id: p.id, msg: &pb.Buy{NpcId: smith, ItemId: game.SmithStock[0], Count: 1}})
	evs := drainEvents(z)
	g := findEvent(evs, "gold", "buy")
	if g == nil || g.Amount != -game.Items[game.SmithStock[0]].Price {
		t.Fatalf("gold out = %+v", g)
	}
	if it := findEvent(evs, "item_in", game.SmithStock[0]); it == nil || it.Extra["how"] != "buy" {
		t.Fatalf("item in = %+v", it)
	}
}

func TestClientStatsRecordsFPS(t *testing.T) {
	z := newCombatZone()
	z.SetEventSink(nopSink{})
	p, _ := joinAt(z, 1, "knight", 30, 0, 0)
	drainEvents(z)
	z.handle(clientStatsEvent{id: p.id, msg: &pb.ClientStats{FpsAvg: 57.6, FpsLow: 41, Device: "mobile android", Seconds: 60}})
	z.handle(clientStatsEvent{id: p.id, msg: &pb.ClientStats{FpsAvg: 9999}})
	evs := drainEvents(z)
	if len(evs) != 1 || evs[0].Kind != "fps" || evs[0].Amount != 58 || evs[0].Extra["device"] != "mobile android" || evs[0].X != 30 {
		t.Fatalf("fps events = %+v", evs)
	}
}

// Achado pelo teste de caos: o cliente manda o FPS, e um NaN, um infinito ou um NUL no
// aparelho iam para o extra do evento. O lote da telemetria é gravado de uma vez, e
// json.Marshal (NaN/Inf) ou o JSONB do Postgres (\u0000) recusavam o lote inteiro: perdiam-se
// junto o ouro e os itens de todo mundo daquele lote, e a conferência da economia não fechava.
func TestClientStatsNeverBreaksTheTelemetryBatch(t *testing.T) {
	z := newCombatZone()
	z.SetEventSink(nopSink{})
	p, _ := joinAt(z, 1, "knight", 30, 0, 0)
	drainEvents(z)
	nan, inf := float32(math.NaN()), float32(math.Inf(1))
	for _, m := range []*pb.ClientStats{
		{FpsAvg: nan, FpsLow: 10}, {FpsAvg: inf}, {FpsAvg: 60, FpsLow: nan}, {FpsAvg: 60, FpsLow: inf},
		{FpsAvg: 60, FpsLow: float32(math.Inf(-1))},
		{FpsAvg: 60, FpsLow: 30, Device: "mobile\x00\x00" + strings.Repeat("é", 30)},
	} {
		z.handle(clientStatsEvent{id: p.id, msg: m})
	}
	evs := drainEvents(z)
	if len(evs) != 1 {
		t.Fatalf("só a última amostra é válida; eventos = %+v", evs)
	}
	b, err := json.Marshal(evs[0].Extra)
	if err != nil || strings.Contains(string(b), `\u0000`) {
		t.Fatalf("extra do fps não pode quebrar o lote: %s, %v", b, err)
	}
}
