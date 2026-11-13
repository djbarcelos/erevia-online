package zone

import (
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Dias de teste no fuso dos eventos (Brasília): 14/11/2026 é um sábado.
var (
	saturday  = time.Date(2026, 11, 14, 12, 0, 0, 0, brasilia)
	monday0   = time.Date(2026, 11, 16, 0, 0, 0, 0, brasilia)
	wednesday = time.Date(2026, 11, 18, 12, 0, 0, 0, brasilia)
)

// newEventZone: zona com o relógio do teste e XP em dobro no sábado e no domingo.
func newEventZone(now *time.Time, saved ...EventData) *Zone {
	cfg := testConfig()
	cfg.Spawns = []game.Spawn{fixed("skeleton_minion", 3, 30, 0)}
	cfg.SafeRadius = 20
	cfg.XP2Days = []time.Weekday{time.Saturday, time.Sunday}
	cfg.Events = saved
	cfg.Clock = func() time.Time { return *now }
	return New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

// killXP: a XP que p ganha matando o lacaio da zona.
func killXP(z *Zone, p *entity) int64 {
	m := z.onlyMonster()
	if m.dead {
		z.reviveMonster(m)
	}
	before := p.pl.xp
	z.kill(m, p)
	return p.pl.xp - before
}

// savedEvents: o que a zona mandou gravar (os testes não rodam o worker).
func savedEvents(z *Zone) map[string]time.Time {
	out := map[string]time.Time{}
	for {
		select {
		case e := <-z.eventJobs:
			out[e.ID] = e.Until
		default:
			return out
		}
	}
}

func TestXP2BySchedulePersistsAcrossTheWeekendAndDoublesXP(t *testing.T) {
	now := wednesday
	z := newEventZone(&now)
	p, peer := joinAt(z, 1, "knight", 30, 3, 0)
	normal := killXP(z, p)
	if z.xp2.on || peer.lastEvents() != nil {
		t.Fatal("quarta-feira: sem XP em dobro (e sem estado no HUD)")
	}

	now = saturday
	z.tickFor(1)
	if !z.xp2.on {
		t.Fatal("sábado: XP em dobro pelo horário")
	}
	if a := peer.announces(); len(a) != 1 || a[0].Title != "XP em dobro!" || !strings.Contains(a[0].Text, "até o fim de domingo") {
		t.Fatalf("aviso ao ligar = %+v", a)
	}
	if e := peer.lastEvents(); e == nil || !e.Xp2 || e.Xp2UntilMs != monday0.UnixMilli() {
		t.Fatalf("HUD = %+v (esperava até %v)", e, monday0)
	}
	if got := killXP(z, p); got != 2*normal {
		t.Fatalf("XP em dobro: %d, normal %d", got, normal)
	}
	// Quem entra durante o evento já recebe o estado.
	_, late := joinAt(z, 2, "mage", 30, -3, 0)
	if e := late.lastEvents(); e == nil || !e.Xp2 {
		t.Fatal("quem entra no meio do evento vê o indicador")
	}

	now = monday0.Add(time.Second)
	z.tickFor(1)
	if z.xp2.on {
		t.Fatal("segunda-feira: desligada")
	}
	if a := peer.announces(); len(a) != 2 || a[1].Title != "Fim da XP em dobro" {
		t.Fatalf("aviso ao desligar = %+v", a)
	}
	if e := peer.lastEvents(); e.Xp2 {
		t.Fatal("HUD sem a XP em dobro")
	}
	if got := killXP(z, p); got != normal {
		t.Fatalf("XP normal de novo: %d, esperava %d", got, normal)
	}
}

func TestXP2ByGMTurnsOnForHoursAndOffAndSurvivesRestart(t *testing.T) {
	now := wednesday
	z := newEventZone(&now)
	mod := newFakeMod()
	z.SetModBackend(mod)
	gm := joinAs(z, 1, "Mestre", true)
	beto := joinAs(z, 2, "Beto", false)

	say(z, 2, "/evento xp2 3")
	if z.xp2.on || !strings.Contains(beto.lastSystem(), "Comando desconhecido") {
		t.Fatal("só GM liga a XP em dobro")
	}
	say(z, 1, "/evento xp2 3")
	drainMod(z)
	until := wednesday.Add(3 * time.Hour)
	if !z.xp2.on || !z.xp2.until.Equal(until) || !strings.Contains(gm.lastSystem(), "até as 15:00") {
		t.Fatalf("GM: ligada até %v (%q)", z.xp2.until, gm.lastSystem())
	}
	if a := beto.announces(); len(a) != 1 || !strings.Contains(a[0].Text, "até as 15:00") {
		t.Fatalf("todos são avisados: %+v", a)
	}
	if len(mod.logs) != 1 || mod.logs[0].Command != "evento" || mod.logs[0].Target != "xp2" || mod.logs[0].Detail != "3 h" {
		t.Fatalf("gm_log = %+v", mod.logs)
	}
	if s := savedEvents(z); !s[eventXP2].Equal(until) {
		t.Fatalf("salvo no banco: %v", s)
	}

	// Reinício no meio do evento: continua ligada (sem novo aviso).
	z2 := newEventZone(&now, EventData{ID: eventXP2, Until: until})
	if !z2.xp2.on {
		t.Fatal("a XP em dobro de GM sobrevive ao reinício")
	}

	now = until
	z.tickFor(1)
	if z.xp2.on {
		t.Fatal("depois do prazo, desliga sozinha")
	}

	// No sábado (horário ligado), /evento xp2 off desliga até o fim do fim de semana.
	now = saturday
	z.tickFor(1)
	if !z.xp2.on {
		t.Fatal("sábado: ligada pelo horário")
	}
	say(z, 1, "/evento xp2 off")
	if z.xp2.on {
		t.Fatal("o GM desligou")
	}
	saved := savedEvents(z)
	if !saved[eventXP2Off].Equal(monday0) {
		t.Fatalf("o horário fica desligado até segunda: %v", saved)
	}
	now = saturday.Add(20 * time.Hour) // domingo
	z.tickFor(1)
	if z.xp2.on {
		t.Fatal("domingo continua desligada")
	}
	z3 := newEventZone(&now, EventData{ID: eventXP2, Until: saved[eventXP2]}, EventData{ID: eventXP2Off, Until: saved[eventXP2Off]})
	if z3.xp2.on {
		t.Fatal("o desligamento do GM sobrevive ao reinício")
	}
	// Ligar de novo vence o desligamento.
	say(z, 1, "/evento xp2 1,5")
	if !z.xp2.on || !z.xp2.until.Equal(monday0) {
		t.Fatalf("religada: até %v", z.xp2.until)
	}
	say(z, 1, "/evento xp2 999")
	if !strings.HasPrefix(gm.lastSystem(), "Uso: /evento xp2") {
		t.Fatalf("prazo inválido: %q", gm.lastSystem())
	}
	say(z, 1, "/evento")
	if !strings.Contains(gm.lastSystem(), "XP em dobro: ligada") || !strings.Contains(gm.lastSystem(), "sábado, domingo") {
		t.Fatalf("estado: %q", gm.lastSystem())
	}
}

func TestParseWeekdays(t *testing.T) {
	for in, want := range map[string][]time.Weekday{
		"sab,dom":        {time.Saturday, time.Sunday},
		"Sábado domingo": {time.Saturday, time.Sunday},
		"fri;sat":        {time.Friday, time.Saturday},
		"0,6,6":          {time.Sunday, time.Saturday},
		"":               nil,
		"nenhum":         nil,
	} {
		got, err := ParseWeekdays(in)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("%q = %v, %v (esperava %v)", in, got, err, want)
		}
	}
	if _, err := ParseWeekdays("sab,feriado"); err == nil {
		t.Error("dia desconhecido deveria dar erro")
	}
}

func TestEventStatusTracksTheInvasionForTheHUD(t *testing.T) {
	z := newTownZone()
	_, peer := defender(z, 1, "Ana", 20, 20, 10, true)
	say(z, 1, "/evento invasao agora")
	z.tickFor(3.5) // o HUD recebe os vivos no máximo 1×/s
	e := peer.lastEvents()
	if e.Invasion != pb.InvasionPhase_INVASION_PHASE_ACTIVE || e.Wave != 1 || e.Waves != uint32(len(invasionWaves)) || e.Alive != uint32(len(z.inv.alive)) {
		t.Fatalf("HUD da invasão = %+v (vivos %d)", e, len(z.inv.alive))
	}
	// O relógio do teste está parado: o fim por tempo é o meio-dia + o limite − o que já passou.
	if want := z.wallAt(z.inv.at).UnixMilli(); e.InvasionAtMs < want || e.InvasionAtMs > want+int64(invStatusEvery*1000) {
		t.Fatalf("fim por tempo %d, esperava %d", e.InvasionAtMs, want)
	}
}
