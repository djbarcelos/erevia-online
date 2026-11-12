package zone

import (
	"math"
	"testing"
)

func TestStepMovesTowardTarget(t *testing.T) {
	pos, moved := step(Vec2{0, 0}, Vec2{10, 0}, 2)
	if !moved || pos != (Vec2{2, 0}) {
		t.Fatalf("got %v moved=%v, want {2 0} moved=true", pos, moved)
	}
}

func TestStepDiagonalKeepsSpeed(t *testing.T) {
	pos, _ := step(Vec2{0, 0}, Vec2{10, 10}, 1)
	if d := math.Hypot(pos.X, pos.Z); math.Abs(d-1) > 1e-9 {
		t.Fatalf("moved %v, want 1", d)
	}
}

func TestStepStopsAtTarget(t *testing.T) {
	pos, moved := step(Vec2{0, 0}, Vec2{1, 0}, 5)
	if !moved || pos != (Vec2{1, 0}) {
		t.Fatalf("got %v, want exactly the target", pos)
	}
	pos, moved = step(pos, Vec2{1, 0}, 5)
	if moved {
		t.Fatalf("should not move when already at target, got %v", pos)
	}
}

func TestClampToZone(t *testing.T) {
	got := clampToZone(Vec2{500, -500}, 100)
	if got != (Vec2{100, -100}) {
		t.Fatalf("got %v, want {100 -100}", got)
	}
	in := Vec2{3, -4}
	if clampToZone(in, 100) != in {
		t.Fatal("point inside zone should not change")
	}
}

func TestValidName(t *testing.T) {
	for _, ok := range []string{"Ana", "Guerreiro01", "abcdefghijklmnop"} {
		if !ValidName(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "ab", "abcdefghijklmnopq", "nome com espaco", "<script>", "ção"} {
		if ValidName(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}
