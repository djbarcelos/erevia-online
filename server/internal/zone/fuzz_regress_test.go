package zone

// Regressões dos bugs achados pelo fuzz da entrada (tunnel#118). Cada teste reproduz a
// mensagem que o fuzz encontrou.

import (
	"math"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/world"
)

// MoveTo{x: NaN} passava pelo clamp (min/max com NaN dão NaN): o destino virava NaN, a
// posição também no tick seguinte, e o personagem ficava fora do mapa para sempre (e era
// salvo assim). Achado pelo FuzzZoneSemantic: move_to:{z:nan}.
func TestMoveToNaNIsIgnored(t *testing.T) {
	mapa, err := world.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []*world.World{nil, mapa} {
		z := newTestZone(nil)
		z.cfg.World = w
		join(z, 1, "Ana")
		p := z.players[1]
		start := p.pos
		for _, m := range []moveEvent{
			{id: 1, x: math.NaN(), z: 0},
			{id: 1, x: 3, z: math.NaN()},
			{id: 1, x: math.NaN(), z: math.NaN()},
		} {
			z.handle(m)
			z.tickFor(0.5)
			if p.pos != start || p.dest != start {
				t.Fatalf("world=%v, move %+v: pos=%v dest=%v (devia ficar em %v)", w != nil, m, p.pos, p.dest, start)
			}
		}
		// ±Inf continua valendo: o clamp leva para a borda.
		z.handle(moveEvent{id: 1, x: math.Inf(1), z: math.Inf(-1)})
		if b := z.bounds(); w == nil && (p.dest.X != b.MaxX || p.dest.Z != b.MinZ) {
			t.Fatalf("move para +Inf/-Inf: dest=%v, quer a borda %+v", p.dest, b)
		}
	}
}

// O saque espalhado em volta do corpo (dropItem → scatter) só conferia obstáculos, não os
// limites: quem morria na borda (PK com karma, monstro, recompensa de missão com a bolsa
// cheia) deixava itens até 1,5 m fora do mapa, e ir pegar levava o personagem para fora
// também (o movimento não limita à borda). Achado pelo FuzzZoneSemantic: Ana mata Beto
// (com karma) depois de ele andar até a borda com move_to:{x:inf}; o item caiu em
// x=150,38 num mapa de ±150.
func TestDropAtEdgeStaysInsideMap(t *testing.T) {
	mapa, err := world.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []*world.World{nil, mapa} {
		z := newTestZone(nil)
		z.cfg.World = w
		b := z.bounds()
		corners := []Vec2{{X: b.MaxX, Z: b.MaxZ}, {X: b.MinX, Z: b.MinZ}, {X: b.MaxX, Z: 0}, {X: 0, Z: b.MinZ}}
		def := game.Items["boots_good"]
		for i := range 200 {
			at := corners[i%len(corners)]
			g := z.dropItem(at, def, 1, 0, 0, 0)
			if g == nil {
				t.Fatal("dropItem não pôs o item")
			}
			if !b.Contains(world.Point{X: g.pos.X, Z: g.pos.Z}) {
				t.Fatalf("world=%v: item caiu fora do mapa em %v (limites %+v)", w != nil, g.pos, b)
			}
			delete(z.ground, g.id)
		}
	}
}

