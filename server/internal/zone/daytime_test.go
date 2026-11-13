package zone

import (
	"io"
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/game"
)

// Instantes de dia e de noite no mesmo ciclo.
var (
	cycleStart = time.UnixMilli(game.DayCycle.Milliseconds() * 1000)
	noon       = cycleStart.Add(15 * time.Minute) // fase 0.375
	midnight   = cycleStart.Add(35 * time.Minute) // fase 0.875
)

// testConfig é a configuração padrão com o relógio parado ao meio-dia: os testes não
// podem depender da hora real (à noite os monstros batem mais e enxergam mais longe).
func testConfig() Config {
	cfg := DefaultConfig()
	cfg.Clock = func() time.Time { return noon }
	// Sem eventos pelo horário: a XP em dobro de fim de semana e a invasão sorteada à noite
	// mudariam as contas dos outros testes (events_test.go e invasion_test.go ligam).
	cfg.InvasionChance, cfg.XP2Days = 0, nil
	return cfg
}

// newClockZone cria uma zona de combate com um relógio controlado pelo teste.
func newClockZone(now *time.Time, spawns ...game.Spawn) *Zone {
	cfg := testConfig()
	cfg.Spawns = spawns
	cfg.SafeRadius = 20
	cfg.Clock = func() time.Time { return *now }
	return New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

func TestNightMakesMonstersStrongerAndRewardsMore(t *testing.T) {
	xpFor := func(at time.Time) (atk float64, xp int64, aggro float64) {
		now := at
		z := newClockZone(&now, fixed("skeleton_minion", 3, 30, 0))
		p, _ := joinAt(z, 1, "knight", 30, 3, 0)
		m := z.onlyMonster()
		before := p.pl.xp
		z.kill(m, p)
		return z.attackPower(m, false), p.pl.xp - before, z.aggroRange()
	}
	dayAtk, dayXP, dayAggro := xpFor(noon)
	nightAtk, nightXP, nightAggro := xpFor(midnight)

	if math.Abs(nightAtk/dayAtk-game.NightMonsterAtk) > 1e-9 {
		t.Errorf("ataque à noite %.2f, de dia %.2f: esperava ×%.2f", nightAtk, dayAtk, game.NightMonsterAtk)
	}
	if want := int64(math.Round(float64(dayXP) * game.NightXP)); nightXP != want {
		t.Errorf("XP à noite %d, esperava %d (de dia %d)", nightXP, want, dayXP)
	}
	if dayAggro != aggroRange || nightAggro != game.NightAggroRange {
		t.Errorf("alcance de agressão dia/noite = %v/%v", dayAggro, nightAggro)
	}
}

func TestNightDoesNotBuffPlayers(t *testing.T) {
	now := midnight
	z := newClockZone(&now)
	p, _ := joinAt(z, 1, "barbarian", 30, 0, 0)
	if got, want := z.attackPower(p, false), p.atk(false, z.now); got != want {
		t.Errorf("jogador à noite: ataque %.2f, esperava %.2f", got, want)
	}
}

func TestDaytimeChangeIsAnnouncedOnce(t *testing.T) {
	now := noon
	z := newClockZone(&now)
	_, peer := joinAt(z, 1, "mage", 30, 0, 0)
	z.tickFor(0.2)
	if peer.systemContains("A noite caiu") {
		t.Fatal("anunciou a noite em pleno dia")
	}

	now = midnight
	z.tickFor(0.5)
	n := 0
	for _, c := range peer.chats() {
		if strings.HasPrefix(c.Text, "A noite caiu") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("esperava 1 aviso de noite, veio %d", n)
	}

	now = cycleStart.Add(game.DayCycle) // amanhece
	z.tickFor(0.1)
	if !peer.systemContains("O sol nasceu") {
		t.Fatal("não avisou o nascer do sol")
	}
}

func TestWelcomeCarriesWorldClock(t *testing.T) {
	now := midnight
	z := newClockZone(&now)
	_, peer := joinAt(z, 1, "rogue", 30, 0, 0)
	var ok bool
	for _, m := range peer.msgs {
		if w := m.GetWelcome(); w != nil {
			ok = w.ServerTimeMs == midnight.UnixMilli() && w.DayCycleMs == uint32(game.DayCycle.Milliseconds())
		}
	}
	if !ok {
		t.Fatal("Welcome sem o relógio do mundo")
	}
}
