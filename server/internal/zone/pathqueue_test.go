package zone

import (
	"io"
	"log/slog"
	"math"
	"slices"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/world"
)

// newRealZone: a zona com o mapa de verdade (muralha, casas, árvores e o vale ao sul).
func newRealZone(t testing.TB) *Zone {
	t.Helper()
	w, err := world.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := testConfig()
	cfg.World, cfg.Size, cfg.SafeRadius = w, w.Size, w.SafeRadius
	return New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

// O "Ir" de uma missão no vale, de dentro da cidade: o caminho é longo e a muralha está
// no meio, então vai para a fila; no máximo longPathsPerTick por tick são calculados.
func TestLongPathsWaitInLine(t *testing.T) {
	z := newRealZone(t)
	var ps []*entity
	for i := 0; i < 7; i++ {
		a := float64(i) * 2 * math.Pi / 7
		p, _ := joinAt(z, uint32(i+1), "knight", 15*math.Cos(a), 15*math.Sin(a), 0)
		ps = append(ps, p)
	}
	for _, p := range ps {
		z.handle(moveEvent{id: p.id, x: 0, z: -280})
	}
	if len(z.paths) != len(ps) {
		t.Fatalf("queue = %d, want %d", len(z.paths), len(ps))
	}
	routed := func() int {
		n := 0
		for _, p := range ps {
			if len(p.path) > 0 {
				n++
			}
		}
		return n
	}
	for tick := 1; tick <= 4; tick++ {
		z.update(0.05)
		if want := min(len(ps), tick*longPathsPerTick); routed() != want {
			t.Fatalf("tick %d: %d players have a path, want %d", tick, routed(), want)
		}
	}
	if len(z.paths) != 0 {
		t.Fatalf("queue should be empty, has %d", len(z.paths))
	}
	// Todos saem pela muralha rumo ao vale.
	for i := 0; i < 400; i++ {
		z.update(0.05)
	}
	for _, p := range ps {
		if p.pos.Z > -30 {
			t.Fatalf("player %d did not head south: %v", p.id, p.pos)
		}
	}
}

// Perto (ou sem obstáculo) segue na hora, como sempre; e um pedido novo (perto, parar ou
// atacar) cancela o longo que esperava. Um jogador só ocupa um lugar na fila.
func TestLongPathQueueIsCancelledAndDeduplicated(t *testing.T) {
	z := newRealZone(t)
	p, _ := joinAt(z, 1, "rogue", 2, 2, 0)
	z.handle(moveEvent{id: p.id, x: 0, z: 12})
	if p.pl.pathGoal != nil || len(z.paths) != 0 || p.dest == p.pos {
		t.Fatalf("short move should go right away (dest %v, queue %d)", p.dest, len(z.paths))
	}

	for i := 0; i < 5; i++ {
		p.pl.moveTokens = z.cfg.MaxMovesPerSec
		z.handle(moveEvent{id: p.id, x: float64(i), z: -280})
	}
	if len(z.paths) != 1 || p.pl.pathGoal == nil || p.pl.pathGoal.X != 4 {
		t.Fatalf("five long requests should take one place with the last goal: queue %d, goal %v", len(z.paths), p.pl.pathGoal)
	}
	z.handle(stopEvent{id: p.id})
	z.update(0.05)
	if len(p.path) > 0 || p.dest != p.pos {
		t.Fatalf("stop should cancel the queued path, dest %v path %d", p.dest, len(p.path))
	}

	// Atacar enquanto espera: o caminho longo não sobrescreve a perseguição.
	p.pl.moveTokens = z.cfg.MaxMovesPerSec
	z.handle(moveEvent{id: p.id, x: 0, z: -280})
	p.act = action{kind: actAttack}
	z.updatePaths()
	if p.pl.pathGoal != nil || len(p.path) > 0 {
		t.Fatalf("attacking should drop the queued path, path %d", len(p.path))
	}
}

// Quem sai da zona com um pedido na fila não trava a fila nem recebe caminho.
func TestLongPathQueueSkipsWhoLeft(t *testing.T) {
	z := newRealZone(t)
	a, _ := joinAt(z, 1, "mage", 3, 3, 0)
	b, _ := joinAt(z, 2, "mage", -3, 3, 0)
	z.handle(moveEvent{id: a.id, x: 0, z: -280})
	z.handle(moveEvent{id: b.id, x: 0, z: -280})
	delete(z.players, a.id) // saiu
	z.updatePaths()
	if len(b.path) == 0 || len(z.paths) != 0 {
		t.Fatalf("b should get its path in the same tick (path %d, queue %d)", len(b.path), len(z.paths))
	}
}

// O passo de matar diz quais monstros contam (o "Caçar aqui" só mira neles).
func TestKillStepNamesItsMonsters(t *testing.T) {
	z := newQuestZone()
	p, _ := joinAt(z, 1, "knight", 0, 0, 0)
	got := z.stepInfo(p, game.Quests["tut_bones"].Steps[0], 0).Monsters
	if !slices.Equal(got, []string{"Esqueleto Guerreiro", "Esqueleto Ladino"}) {
		t.Fatalf("monsters = %v", got)
	}
	if m := z.stepInfo(p, game.Quests["tut_hunt"].Steps[0], 0).Monsters; !slices.Equal(m, []string{"Esqueleto Lacaio"}) {
		t.Fatalf("minion step monsters = %v", m)
	}
	if m := z.stepInfo(p, game.Quests["tut_level"].Steps[0], 0).Monsters; len(m) != 0 {
		t.Fatalf("level step should not filter monsters, got %v", m)
	}
}
