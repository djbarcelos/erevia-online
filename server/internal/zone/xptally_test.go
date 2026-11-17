package zone

import (
	"testing"

	"erevia/server/internal/game"
)

// xpBySource soma os eventos "xp" por fonte.
func xpBySource(evs []GameEvent) map[string]int64 {
	out := map[string]int64{}
	for _, e := range evs {
		if e.Kind == "xp" {
			out[e.Ref] += e.Amount
		}
	}
	return out
}

// Um abate de noite com XP em dobro: a soma vai para o banco só ao sair, dividida em
// abate + extra da noite + extra do evento, e fecha com a XP que o jogador ganhou.
func TestXPTallySplitsMonsterXPBySource(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 5, 25, 0))
	z.SetEventSink(nopSink{})
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "knight", 23.5, 0, game.XPLevelStart(5))
	drainEvents(z)
	z.night, z.xp2.on = true, true

	before := p.pl.xp
	z.kill(m, p)
	gained := p.pl.xp - before
	if gained <= 0 {
		t.Fatal("o abate tem que dar XP")
	}
	if evs := drainEvents(z); xpBySource(evs)["monster"] != 0 {
		t.Fatalf("a XP não pode ir para o banco a cada abate: %+v", evs)
	}

	z.removePlayer(p.id)
	got := xpBySource(drainEvents(z))
	if got["monster"]+got["night_bonus"]+got["event_bonus"] != gained {
		t.Fatalf("fontes %v não fecham a XP ganha %d", got, gained)
	}
	if got["event_bonus"] != gained/2 || got["night_bonus"] <= 0 {
		t.Fatalf("extras errados: %v (ganhou %d)", got, gained)
	}
	if len(z.xpTally.by) != 0 {
		t.Fatal("quem saiu não pode ficar no mapa")
	}
}

// Missão, morte e a gravação de minuto em minuto, com o nível em que a XP veio.
func TestXPTallyQuestDeathAndPeriodicFlush(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 3, 25, 0))
	z.SetEventSink(nopSink{})
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "knight", 23.5, 0, game.XPLevelStart(3)+10)
	drainEvents(z)

	z.grantXPFrom(p, xpQuest, 5)
	before := p.pl.xp
	z.kill(p, m)
	lost := before - p.pl.xp
	if lost <= 0 {
		t.Fatal("morrer para um monstro custa XP")
	}
	z.tickFor(1)
	got := xpBySource(drainEvents(z))
	if got["quest"] != 5 || got["death_loss"] != -lost {
		t.Fatalf("fontes = %v, quero quest 5 e death_loss %d", got, -lost)
	}

	// Até o próximo minuto, só soma.
	z.grantXPFrom(p, xpQuest, 7)
	z.tickFor(5)
	if evs := xpBySource(drainEvents(z)); evs["quest"] != 0 {
		t.Fatalf("gravou antes do minuto: %v", evs)
	}
	z.tickFor(xpTallyEvery)
	if evs := xpBySource(drainEvents(z)); evs["quest"] != 7 {
		t.Fatalf("depois do minuto = %v, quero quest 7", evs)
	}

	// Subir de nível fecha a soma do nível anterior.
	z.grantXPFrom(p, xpQuest, game.XPLevelStart(4)-p.pl.xp)
	z.grantXPFrom(p, xpQuest, 3)
	z.endXPTally(p)
	levels := map[int]int64{}
	for _, e := range drainEvents(z) {
		if e.Kind == "xp" {
			levels[e.Level] += e.Amount
		}
	}
	if p.level != 4 || levels[4] != 3 || levels[3] <= 0 {
		t.Fatalf("nível %d, XP por nível = %v", p.level, levels)
	}
}

// Desligar a zona fecha a sessão (e a XP) de quem ainda está online.
func TestEndSessionsOnShutdown(t *testing.T) {
	z := newCombatZone()
	z.SetEventSink(nopSink{})
	p, _ := joinAt(z, 1, "mage", 30, 0, 0)
	z.grantXPFrom(p, xpQuest, 4)
	drainEvents(z)
	z.endSessions()
	evs := drainEvents(z)
	if e := findEvent(evs, "session", "end"); e == nil || e.Extra["shutdown"] != true {
		t.Fatalf("fim de sessão no desligamento = %+v", e)
	}
	if xpBySource(evs)["quest"] != 4 {
		t.Fatalf("XP somada não gravada no desligamento: %+v", evs)
	}
}
