package world

import (
	"math"
	"testing"
)

func pathOK(t *testing.T, w *World, from Point, path []Point) {
	t.Helper()
	cur := from
	for i, p := range path {
		if !w.SegmentClear(cur, p) {
			t.Fatalf("segment %d (%v → %v) crosses an obstacle", i, cur, p)
		}
		cur = p
	}
}

func length(from Point, path []Point) float64 {
	total, cur := 0.0, from
	for _, p := range path {
		total += math.Hypot(p.X-cur.X, p.Z-cur.Z)
		cur = p
	}
	return total
}

func TestPathGoesAroundObstacle(t *testing.T) {
	w := New(40, []Collider{{X: 5, Z: 0, R: 2}})
	from, to := Point{0, 0}, Point{10, 0}
	if w.SegmentClear(from, to) {
		t.Fatal("straight line should be blocked")
	}
	path := w.FindPath(from, to)
	if len(path) < 2 {
		t.Fatalf("expected a detour, got %v", path)
	}
	pathOK(t, w, from, path)
	if last := path[len(path)-1]; last != to {
		t.Fatalf("path should end at destination, got %v", last)
	}
	if l := length(from, path); l < 10.2 || l > 14 {
		t.Fatalf("detour length %.1f looks wrong", l)
	}
}

func TestStraightLineWhenClear(t *testing.T) {
	w := New(40, []Collider{{X: 5, Z: 8, R: 1}})
	path := w.FindPath(Point{0, 0}, Point{10, 0})
	if len(path) != 1 || path[0] != (Point{10, 0}) {
		t.Fatalf("clear line should be a single waypoint: %v", path)
	}
}

func TestBlockedDestinationGoesToNearestFree(t *testing.T) {
	w := New(40, []Collider{{X: 5, Z: 0, R: 2}})
	path := w.FindPath(Point{-5, 0}, Point{5, 0}) // clicou no meio do obstáculo
	if len(path) == 0 {
		t.Fatal("should still walk next to the obstacle")
	}
	last := path[len(path)-1]
	if w.Blocked(last.X, last.Z) {
		t.Fatalf("path ends inside obstacle: %v", last)
	}
	if d := math.Hypot(last.X-5, last.Z); d > 2+AgentRadius+1.5 {
		t.Fatalf("should stop close to the obstacle, got %.1f m away", d)
	}
}

func TestUnreachableReturnsClosest(t *testing.T) {
	// Cercado por um anel de colisores: não há como entrar.
	var ring []Collider
	for a := 0.0; a < 2*math.Pi; a += 0.2 {
		ring = append(ring, Collider{X: 10 * math.Cos(a), Z: 10 * math.Sin(a), R: 1.5})
	}
	w := New(60, ring)
	path := w.FindPath(Point{20, 0}, Point{0, 0})
	if len(path) == 0 {
		return // aceitável: não anda
	}
	last := path[len(path)-1]
	if math.Hypot(last.X, last.Z) < 9 {
		t.Fatalf("must not enter the sealed ring: %v", last)
	}
	pathOK(t, w, Point{20, 0}, path)
}

func TestEmbeddedWorld(t *testing.T) {
	w, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if w.Size != 300 || w.SafeRadius < 35 || len(w.colliders) < 100 {
		t.Fatalf("unexpected world: size=%v safe=%v colliders=%d", w.Size, w.SafeRadius, len(w.colliders))
	}
	// Muralha (raio 38) a 30° bloqueia; portões (0°, 90°…) ficam abertos.
	a := math.Pi / 6
	if !w.Blocked(38*math.Cos(a), 38*math.Sin(a)) {
		t.Error("wall at 30° should block")
	}
	for _, g := range []Point{{38, 0}, {0, 38}, {-38, 0}, {0, -38}} {
		if w.Blocked(g.X, g.Z) {
			t.Errorf("gate at %v should be open", g)
		}
	}
	// Sair do centro da cidade para a floresta diagonal: o caminho passa por um portão.
	from := w.NearestFree(Point{3, 3})
	to := w.NearestFree(Point{60, 60})
	path := w.FindPath(from, to)
	if len(path) < 2 {
		t.Fatalf("expected path through a gate, got %v", path)
	}
	pathOK(t, w, from, path)
	throughGate := false
	for _, p := range path {
		r := math.Hypot(p.X, p.Z)
		if r > 34 && r < 42 && (math.Abs(p.X) < 3.5 || math.Abs(p.Z) < 3.5) {
			throughGate = true
		}
	}
	if !throughGate {
		t.Logf("path: %v", path)
		t.Error("path out of town should go through a gate")
	}
}

func TestEmbeddedWorldGrowsSouthOnly(t *testing.T) {
	w, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	b := w.Bounds
	if b.MinX != -150 || b.MaxX != 150 || b.MaxZ != 150 || b.MinZ != -330 {
		t.Fatalf("o mapa deveria crescer só para o sul (até z=-330): %+v", b)
	}
	// O vale é alcançável a pé pela cidade (portão sul → estrada → entrada do vale) e
	// dá para andar até o fundo da área dos golens.
	from := w.NearestFree(Point{0, -42})
	for _, goal := range []Point{{6, -198}, {0, -270}, {-30, -300}, {35, -290}} {
		to := w.NearestFree(goal)
		path := w.FindPath(from, to)
		if len(path) == 0 {
			t.Fatalf("sem caminho até %v", goal)
		}
		pathOK(t, w, from, path)
		if end := path[len(path)-1]; math.Hypot(end.X-to.X, end.Z-to.Z) > 0.5 {
			t.Errorf("caminho até %v parou em %v", goal, end)
		}
	}
	// A célula mais ao sul existe na grade (o A* não corta o mapa estendido).
	if p := w.NearestFree(Point{0, -328}); p.Z > -320 {
		t.Errorf("o fundo do vale deveria ser andável: %v", p)
	}
}
