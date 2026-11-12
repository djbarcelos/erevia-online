package zone

import (
	"io"
	"log/slog"
	"math"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/world"
)

// Uma "parede" de colisores em x=10, de z=-6 a z=6.
func wallWorld() *world.World {
	var cs []world.Collider
	for z := -6.0; z <= 6; z += 1 {
		cs = append(cs, world.Collider{X: 10, Z: z, R: 0.8})
	}
	return world.New(200, cs)
}

func newNavZone(w *world.World, spawns ...game.Spawn) *Zone {
	cfg := DefaultConfig()
	cfg.Spawns = spawns
	cfg.World = w
	cfg.SafeRadius = 0 // os testes de navegação lutam perto da origem
	return New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

func TestMoveAroundWall(t *testing.T) {
	z := newNavZone(wallWorld())
	p, _ := joinAt(z, 1, "knight", 5, 0, 0)
	z.handle(moveEvent{id: p.id, x: 15, z: 0})
	if len(p.path) == 0 && p.dest == (Vec2{15, 0}) {
		t.Fatal("blocked straight line should produce a path")
	}
	for i := 0; i < 200; i++ {
		z.update(0.05)
		if !z.walkable(p.pos) {
			t.Fatalf("player inside obstacle at %v", p.pos)
		}
	}
	if d := dist(p.pos, Vec2{15, 0}); d > 0.2 {
		t.Fatalf("player should reach the other side, still %.1f m away at %v", d, p.pos)
	}
}

func TestClickInsideObstacleStopsNextToIt(t *testing.T) {
	z := newNavZone(wallWorld())
	p, _ := joinAt(z, 1, "knight", 5, 0, 0)
	z.handle(moveEvent{id: p.id, x: 10, z: 0}) // no meio da parede
	z.tickFor(5)
	if !z.walkable(p.pos) {
		t.Fatalf("player inside obstacle at %v", p.pos)
	}
	if d := dist(p.pos, Vec2{10, 0}); d > 2.5 {
		t.Fatalf("should stop next to the wall, got %.1f m away", d)
	}
}

func TestMonsterChasesAroundWallAndNeverClips(t *testing.T) {
	z := newNavZone(wallWorld(), fixed("skeleton_warrior", 6, 14, 0))
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "knight", 7, 0, 0) // 7 m, do outro lado da parede
	p.hp, p.stats.MaxHP = 1e9, 1e9          // não morre no teste
	z.handle(attackEvent{id: p.id, target: m.id})
	reached := false
	for i := 0; i < 300; i++ {
		z.update(0.05)
		if !z.walkable(m.pos) || !z.walkable(p.pos) {
			t.Fatalf("entity inside obstacle: monster %v player %v", m.pos, p.pos)
		}
		if dist(p.pos, m.pos) < p.stats.Range+0.5 {
			reached = true
		}
	}
	if !reached {
		t.Fatalf("player should reach the monster around the wall (player %v monster %v)", p.pos, m.pos)
	}
}

func TestSpawnsNeverInsideObstacles(t *testing.T) {
	// Colisor exatamente em cima do ponto do monstro e do jogador.
	w := world.New(200, []world.Collider{{X: 30, Z: 0, R: 2}, {X: 0, Z: 0, R: 1}})
	z := newNavZone(w, fixed("skeleton_minion", 1, 30, 0))
	m := z.onlyMonster()
	if !z.walkable(m.pos) || dist(m.pos, Vec2{30, 0}) > 4 {
		t.Fatalf("monster should spawn next to the obstacle, got %v", m.pos)
	}
	p, _ := joinAt(z, 1, "mage", 0, 0, 0)
	if !z.walkable(p.pos) {
		t.Fatalf("player joined inside obstacle at %v", p.pos)
	}
}

func TestRealWorldIntegration(t *testing.T) {
	w, err := world.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.World, cfg.Size, cfg.SafeRadius = w, w.Size, w.SafeRadius
	z := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	for _, m := range z.monsters {
		if !z.walkable(m.pos) {
			t.Fatalf("monster %s spawned inside obstacle at %v", m.name, m.pos)
		}
		if math.Hypot(m.pos.X, m.pos.Z) < cfg.SafeRadius {
			t.Fatalf("monster inside town at %v", m.pos)
		}
	}
	// Do centro até a estrada norte, lá fora.
	p, _ := joinAt(z, 1, "rogue", 2, 2, 0)
	z.handle(moveEvent{id: p.id, x: 0, z: 55})
	for i := 0; i < 300; i++ {
		z.update(0.05)
		if !z.walkable(p.pos) {
			t.Fatalf("player inside obstacle at %v", p.pos)
		}
	}
	if d := dist(p.pos, Vec2{0, 55}); d > 0.5 {
		t.Fatalf("player should leave town through the gate, still %.1f m away at %v", d, p.pos)
	}
}
