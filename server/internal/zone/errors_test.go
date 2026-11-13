package zone

import (
	"context"
	"errors"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/pb"
)

// errorZone: zona de teste com telemetria ligada, fila de salvamento e relógio controlado.
func errorZone(t *testing.T) (*Zone, chan []SavedState, *time.Time) {
	t.Helper()
	saves := make(chan []SavedState, 64)
	z := newTestZone(saves)
	z.SetEventSink(nopSink{})
	now := noon
	z.cfg.Clock = func() time.Time { return now }
	z.cfg.Version = "abc123"
	return z, saves, &now
}

func drainSaves(ch chan []SavedState) []SavedState {
	var out []SavedState
	for {
		select {
		case b := <-ch:
			out = append(out, b...)
		default:
			return out
		}
	}
}

func countSnapshots(p *fakePeer) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, m := range p.msgs {
		if m.GetSnapshot() != nil {
			n++
		}
	}
	return n
}

func kickedReason(p *fakePeer) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, m := range p.msgs {
		if k := m.GetKicked(); k != nil {
			return k.Reason
		}
	}
	return ""
}

// Uma mensagem que dá panic no handler (aqui um PickUp nulo, que o handler não espera)
// derruba só quem mandou; a zona segue e o outro jogador continua recebendo snapshots.
func TestPanicInHandlerKicksOnlyThatPlayer(t *testing.T) {
	z, saves, now := errorZone(t)
	ana := join(z, 1, "Ana")
	bia := join(z, 2, "Bia")

	// O último estado salvo da Ana tem 50 moedas; depois disso ela "ganha" 999.
	z.players[1].pl.inv.gold = 50
	z.saveAll(false)
	drainSaves(saves)
	z.players[1].pl.inv.gold += 999
	drainEvents(z)

	z.handleSafe(itemEvent{id: 1, msg: (*pb.PickUp)(nil)})

	if ana.closed != "server error" {
		t.Fatalf("culprit connection: closed = %q", ana.closed)
	}
	if got := kickedReason(ana); got != kickedServerError {
		t.Fatalf("culprit must get Kicked, got %q", got)
	}
	if z.players[1] != nil || z.players[2] == nil {
		t.Fatal("only the culprit must leave the zone")
	}
	if bia.closed != "" {
		t.Fatal("the other player must stay connected")
	}
	// Não salva o estado de depois do panic; o recent é o último estado salvo.
	for _, s := range drainSaves(saves) {
		if s.ID == 1 {
			t.Fatalf("state after a panic must not be saved: %+v", s)
		}
	}
	if r, ok := z.recent[1]; !ok || r.state.Gold != 50 {
		t.Fatalf("recent must be the last saved state (50 gold), got %+v %v", r.state.Gold, ok)
	}

	evs := drainEvents(z)
	e := findEvent(evs, "server_error", "")
	if e == nil {
		t.Fatalf("server_error must be recorded, got %+v", evs)
	}
	if e.CharacterID != 1 || e.Amount != 1 || e.Extra["msg"] != "PickUp" || e.Extra["where"] != "msg" ||
		e.Extra["version"] != "abc123" {
		t.Fatalf("server_error = %+v", e)
	}
	if st, _ := e.Extra["stack"].(string); !strings.Contains(st, "onItem") {
		t.Fatalf("stack must point to the handler:\n%s", st)
	}
	if frames, _ := e.Extra["frames"].([]string); len(frames) == 0 || !strings.Contains(frames[0], "onItem") {
		t.Fatalf("frames = %v", e.Extra["frames"])
	}

	// A zona continua: a Bia anda e recebe snapshots.
	before := countSnapshots(bia)
	z.handleSafe(moveEvent{id: 2, x: 5, z: 5})
	for range 5 {
		z.update(0.05)
	}
	if countSnapshots(bia) <= before {
		t.Fatal("the other player must keep receiving snapshots")
	}

	// Ao voltar, a Ana tem o último estado salvo.
	ana2 := join(z, 1, "Ana")
	if g := z.players[1].pl.inv.gold; g != 50 {
		t.Fatalf("rejoin after panic: gold = %d, want 50", g)
	}

	// O mesmo panic de novo dentro do minuto: não grava, só conta.
	z.handleSafe(itemEvent{id: 1, msg: (*pb.PickUp)(nil)})
	if ana2.closed == "" {
		t.Fatal("second panic must kick again")
	}
	if findEvent(drainEvents(z), "server_error", "") != nil {
		t.Fatal("same stack within a minute must not be recorded again")
	}
	*now = now.Add(61 * time.Second)
	due := z.dueErrors()
	if len(due) != 1 || due[0].Ref != e.Ref || due[0].Amount != 1 || due[0].Extra["total"] != int64(2) {
		t.Fatalf("accumulated count after a minute = %+v", due)
	}
}

// panicPeer dá panic no primeiro envio (o Welcome) e depois funciona.
type panicPeer struct {
	fakePeer
	sends int
}

func (p *panicPeer) Send(data []byte) bool {
	p.sends++
	if p.sends == 1 {
		panic("send boom")
	}
	return p.fakePeer.Send(data)
}

// Panic no meio do Join (o jogador já entrou em z.players): a conexão cai e ele sai.
func TestPanicInJoinClosesConnection(t *testing.T) {
	z, _, _ := errorZone(t)
	peer := &panicPeer{}
	z.handleSafe(joinEvent{info: JoinInfo{ID: 9, Name: "Caio", Class: "knight", Peer: peer}})
	if peer.closed != "server error" {
		t.Fatalf("join panic must close the connection (closed=%q)", peer.closed)
	}
	if z.players[9] != nil {
		t.Fatal("player must not stay in the zone")
	}
	if e := findEvent(drainEvents(z), "server_error", ""); e == nil || e.Extra["msg"] != "Join" {
		t.Fatalf("server_error = %+v", e)
	}
}

// Um sistema do tick que dá panic sempre é desligado depois de panicDisableAfter vezes
// seguidas; o resto do tick continua.
func TestRepeatedSystemPanicDisablesSystem(t *testing.T) {
	z, _, _ := errorZone(t)
	bia := join(z, 2, "Bia")
	z.ground[999] = nil // item no chão corrompido: expireGround (1×/s) dá panic
	drainEvents(z)

	ticks := (panicDisableAfter + 2) * z.cfg.TickRate
	for range ticks {
		z.update(0.05)
	}
	if !z.errs.sys[sysGround].off {
		t.Fatal("ground system must be disabled after repeated panics")
	}
	for s := range numSystems {
		if s != sysGround && z.errs.sys[s].off {
			t.Fatalf("only the ground system must be disabled, %s is off", s)
		}
	}
	evs := drainEvents(z)
	var first, off *GameEvent
	for i := range evs {
		if evs[i].Kind == "server_error" {
			if evs[i].Extra["disabled"] == true {
				off = &evs[i]
			} else if first == nil {
				first = &evs[i]
			}
		}
	}
	if first == nil || first.Extra["where"] != "tick:ground" {
		t.Fatalf("first panic must be recorded: %+v", evs)
	}
	if off == nil {
		t.Fatal("disabling the system must be recorded")
	}

	// Desligado, não roda mais (nada novo), e a zona segue mandando snapshots.
	before := countSnapshots(bia)
	z.handleSafe(moveEvent{id: 2, x: 8, z: 8})
	for range 3 * z.cfg.TickRate {
		z.update(0.05)
	}
	if countSnapshots(bia) <= before {
		t.Fatal("zone must keep running")
	}
	if z.errs.sys[sysGround].streak != panicDisableAfter {
		t.Fatalf("disabled system must not run again (streak %d)", z.errs.sys[sysGround].streak)
	}
}

// Um monstro com estado corrompido dá panic a cada tick: só ele é parado.
func TestRepeatedMonsterPanicParksOnlyThatMonster(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 2, 25, 0), fixed("skeleton_minion", 2, -25, 0))
	z.SetEventSink(nopSink{})
	var bad, good *entity
	for _, m := range z.monsters {
		if bad == nil {
			bad = m
		} else {
			good = m
		}
	}
	bad.mon.def = nil
	for range panicDisableAfter + 5 {
		z.update(0.05)
	}
	if !z.errs.parked[bad.id] || z.errs.parked[good.id] {
		t.Fatalf("parked = %v (bad %d, good %d)", z.errs.parked, bad.id, good.id)
	}
	if z.errs.sys[sysMonsters].off {
		t.Fatal("the monsters system must stay on")
	}
	if findEvent(drainEvents(z), "server_error", "") == nil {
		t.Fatal("monster panic must be recorded")
	}
}

// Tick acima de 50 ms: slow_tick com os sistemas mais lentos, no máximo 1 por minuto.
func TestSlowTickRecordedOncePerMinute(t *testing.T) {
	z, _, now := errorZone(t)
	drainEvents(z)

	z.runSystem(sysTrades, func() { time.Sleep(60 * time.Millisecond) })
	z.checkSlowTick(70 * time.Millisecond)
	e := findEvent(drainEvents(z), "slow_tick", "")
	if e == nil || e.Ref != "trades" || e.Amount != 70 {
		t.Fatalf("slow_tick = %+v", e)
	}
	sys, _ := e.Extra["systems"].(map[string]any)
	if ms, _ := sys["trades"].(float64); ms < 60 {
		t.Fatalf("systems = %v", sys)
	}

	z.checkSlowTick(80 * time.Millisecond) // lento de novo, dentro do minuto
	z.checkSlowTick(10 * time.Millisecond) // dentro do orçamento
	if findEvent(drainEvents(z), "slow_tick", "") != nil {
		t.Fatal("at most one slow_tick per minute")
	}
	if z.stats.slow != 2 {
		t.Fatalf("tick stats must count slow ticks: %d", z.stats.slow)
	}

	*now = now.Add(61 * time.Second)
	z.checkSlowTick(90 * time.Millisecond)
	e = findEvent(drainEvents(z), "slow_tick", "")
	if e == nil || e.Extra["slow_ticks"] != 2 {
		t.Fatalf("second slow_tick must carry the count since the last one: %+v", e)
	}
}

func panicA() { panic("a") }

func panicB() {
	var m map[string]int
	m["x"] = 1
}

func stackOf(f func()) (s []byte) {
	defer func() {
		recover()
		s = debug.Stack()
	}()
	f()
	return nil
}

func TestStackKeyGroupsSameSite(t *testing.T) {
	// Mesmo lugar (mesma linha de chamada) duas vezes: mesmo grupo.
	var keys []string
	var f1 []string
	for range 2 {
		k, f := stackKey(stackOf(panicA))
		keys, f1 = append(keys, k), f
	}
	k1, k2 := keys[0], keys[1]
	k3, _ := stackKey(stackOf(panicB))
	if k1 != k2 || k1 == k3 {
		t.Fatalf("keys: %s %s %s", k1, k2, k3)
	}
	if len(f1) != 3 || !strings.Contains(f1[0], "panicA") || strings.Contains(f1[0], "0x") {
		t.Fatalf("frames = %v", f1)
	}
}

func TestDBErrorCountedOncePerMinute(t *testing.T) {
	z, _, now := errorZone(t)
	drainEvents(z)
	z.ReportDBError("save_characters", errors.New("conn refused"))
	z.ReportDBError("save_characters", errors.New("conn refused"))
	z.ReportDBError("boss_save", errors.New("timeout\x00"))
	z.reportSocialDBError("send mail", ErrMailRate) // recusa de regra, não é erro do banco
	evs := drainEvents(z)
	if e := findEvent(evs, "db_error", "save_characters"); e == nil || e.Amount != 1 || e.Extra["err"] != "conn refused" {
		t.Fatalf("db_error = %+v", e)
	}
	if e := findEvent(evs, "db_error", "boss_save"); e == nil || e.Extra["err"] != "timeout" {
		t.Fatalf("db_error must strip NUL: %+v", e)
	}
	if len(evs) != 2 {
		t.Fatalf("events = %+v", evs)
	}
	*now = now.Add(time.Minute)
	due := z.dueErrors()
	if len(due) != 1 || due[0].Ref != "save_characters" || due[0].Amount != 1 || due[0].Extra["total"] != int64(2) {
		t.Fatalf("due = %+v", due)
	}
}

// Um worker que dá panic é registrado e volta a rodar; o processo não cai.
func TestWorkerPanicIsRecoveredAndRestarted(t *testing.T) {
	z, _, _ := errorZone(t)
	drainEvents(z)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	runs := make(chan int, 4)
	n := 0
	z.goWorker(ctx, "test", func(ctx context.Context) {
		n++
		runs <- n
		if n == 1 {
			panic("worker boom")
		}
		<-ctx.Done()
	})
	for want := 1; want <= 2; want++ {
		select {
		case got := <-runs:
			if got != want {
				t.Fatalf("run %d, want %d", got, want)
			}
		case <-ctx.Done():
			t.Fatal("worker must restart after a panic")
		}
	}
	if e := findEvent(drainEvents(z), "server_error", ""); e == nil || e.Extra["where"] != "worker:test" {
		t.Fatalf("server_error = %+v", e)
	}
}
