package zone

import (
	"math"
	"testing"

	"erevia/server/internal/world"
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
	got := clampToZone(Vec2{500, -500}, world.Square(200))
	if got != (Vec2{100, -100}) {
		t.Fatalf("got %v, want {100 -100}", got)
	}
	in := Vec2{3, -4}
	if clampToZone(in, world.Square(200)) != in {
		t.Fatal("point inside zone should not change")
	}
}

func TestClampToZoneRectangle(t *testing.T) {
	b := world.Bounds{MinX: -150, MaxX: 150, MinZ: -330, MaxZ: 150}
	if got := clampToZone(Vec2{10, -300}, b); got != (Vec2{10, -300}) {
		t.Fatalf("o sul estendido deveria ser válido, veio %v", got)
	}
	if got := clampToZone(Vec2{0, -400}, b); got != (Vec2{0, -330}) {
		t.Fatalf("got %v, want {0 -330}", got)
	}
	if got := clampToZone(Vec2{0, 200}, b); got != (Vec2{0, 150}) {
		t.Fatalf("o norte não cresceu: got %v", got)
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

// Soltar o WASD: o pedido chega atrasado (latência + 1 tick). Antes o cliente mandava
// um destino com a posição que ele via, e o servidor voltava até ela e virava para trás.
// Com o stop o servidor para onde está.
func TestStopAfterWalkingNeverGoesBack(t *testing.T) {
	z := newTestZone(nil)
	join(z, 1, "Ana")
	p := z.players[1]
	start := p.pos
	z.handle(moveEvent{id: 1, x: start.X + 3, z: start.Z})
	for i := 0; i < 8; i++ {
		z.update(0.05) // seis ticks até soltar + dois de latência
	}
	ahead := p.pos.X
	z.handle(stopEvent{id: 1})
	if p.dest != p.pos {
		t.Fatalf("deveria parar onde está: pos=%v dest=%v", p.pos, p.dest)
	}
	for i := 0; i < 10; i++ {
		z.update(0.05)
		if p.pos.X < ahead-1e-9 {
			t.Fatalf("o servidor voltou: x=%.3f depois de ter chegado a %.3f", p.pos.X, ahead)
		}
		if p.face.X <= 0 {
			t.Fatalf("o personagem virou para trás: face=%v", p.face)
		}
	}
}

// Achado pelo teste de caos (loadbot -chaos): MoveTo com NaN passava pelo clampToZone (min e
// max de NaN dão NaN), virava o destino e, no tick seguinte, a posição do jogador. Com
// DEBUG_INVARIANTS=1 a invariante "position" derrubava o servidor inteiro.
// NaN é ignorado (não move); ±Inf o clamp leva para a borda. Nos dois casos a posição e o
// destino continuam finitos (o caos achou o NaN derrubando o servidor com DEBUG_INVARIANTS).
func TestMoveToNaNOrInfIsIgnored(t *testing.T) {
	z := newTestZone(nil)
	join(z, 1, "Ana")
	p := z.players[1]
	start := p.pos
	send := func(x, zz float64) {
		p.pl.moveTokens = z.cfg.MaxMovesPerSec
		z.handle(moveEvent{id: 1, x: x, z: zz})
		for range 20 { // 1 s: a checagem das invariantes passa por todo mundo
			z.update(0.05)
		}
		if !finite(p.pos.X) || !finite(p.pos.Z) || !finite(p.dest.X) || !finite(p.dest.Z) {
			t.Fatalf("MoveTo(%v, %v): posição %v, destino %v", x, zz, p.pos, p.dest)
		}
	}
	for _, v := range [][2]float64{{math.NaN(), 0}, {0, math.NaN()}, {math.NaN(), math.NaN()}} {
		send(v[0], v[1])
	}
	if p.pos != start {
		t.Fatalf("MoveTo com NaN não deveria mover: de %v para %v", start, p.pos)
	}
	for _, v := range [][2]float64{{math.Inf(1), 3}, {2, math.Inf(-1)}} {
		send(v[0], v[1])
	}
}
