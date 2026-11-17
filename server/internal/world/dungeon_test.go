package world

import (
	"math"
	"testing"
)

// As Catacumbas: o mapa que o gerador (client/scripts/dungeons/) gravou é jogável. A entrada,
// as escadas e todo monstro ficam em chão livre, e dá para andar até cada um (cada andar a
// partir do ponto em que se chega a ele).
func TestCatacombsMapIsPlayable(t *testing.T) {
	w, err := LoadMap("catacumbas")
	if err != nil {
		t.Fatal(err)
	}
	d := w.Dungeon
	if d == nil {
		t.Fatal("o mapa não tem o campo dungeon")
	}
	if w.SafeRadius <= 0 || math.Hypot(d.Entry.X, d.Entry.Z) > 0.01 {
		t.Fatalf("a entrada precisa ser a origem (zona segura e renascimento): %+v, raio seguro %v", d.Entry, w.SafeRadius)
	}
	if len(d.Floors) != 2 || len(d.Stairs) != 2 {
		t.Fatalf("%d andares e %d escadas, esperava 2 e 2", len(d.Floors), len(d.Stairs))
	}
	for i, f := range d.Floors {
		if f.MaxX-f.MinX != 60 || f.MaxZ-f.MinZ != 60 {
			t.Errorf("andar %d com %.0f × %.0f m, esperava 60 × 60", i+1, f.MaxX-f.MinX, f.MaxZ-f.MinZ)
		}
	}
	// Alcançável: preenche a grade de 0,5 m a partir de um ponto (o A* devolve o ponto mais
	// próximo quando não há caminho, então ele não prova nada).
	const step = 0.5
	b := w.Bounds
	nx, nz := int((b.MaxX-b.MinX)/step), int((b.MaxZ-b.MinZ)/step)
	cellOf := func(p Point) (int, int) { return int((p.X - b.MinX) / step), int((p.Z - b.MinZ) / step) }
	centre := func(i, j int) Point {
		return Point{X: b.MinX + (float64(i)+0.5)*step, Z: b.MinZ + (float64(j)+0.5)*step}
	}
	flood := func(from Point) []bool {
		seen := make([]bool, nx*nz)
		ei, ej := cellOf(from)
		queue := [][2]int{{ei, ej}}
		seen[ej*nx+ei] = true
		for len(queue) > 0 {
			c := queue[0]
			queue = queue[1:]
			for _, dd := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				i, j := c[0]+dd[0], c[1]+dd[1]
				if i < 0 || j < 0 || i >= nx || j >= nz || seen[j*nx+i] {
					continue
				}
				if p := centre(i, j); !w.Blocked(p.X, p.Z) {
					seen[j*nx+i] = true
					queue = append(queue, [2]int{i, j})
				}
			}
		}
		return seen
	}
	up, down := d.Stairs[0], d.Stairs[1] // sobe (andar 1) e desce (andar 2)
	floorSeen := [2][]bool{flood(d.Entry), flood(Point{X: up.ToX, Z: up.ToZ})}
	reach := func(what string, floor int, p Point) {
		t.Helper()
		if w.Blocked(p.X, p.Z) {
			t.Errorf("%s em (%.1f, %.1f) está dentro de uma parede", what, p.X, p.Z)
			return
		}
		if i, j := cellOf(p); !floorSeen[floor][j*nx+i] {
			t.Errorf("%s em (%.1f, %.1f) não tem caminho dentro do andar %d", what, p.X, p.Z, floor+1)
		}
	}
	reach("entrada", 0, d.Entry)
	reach(up.Name, 0, Point{X: up.X, Z: up.Z})
	reach(down.Name+" (chegada)", 0, Point{X: down.ToX, Z: down.ToZ})
	reach(down.Name, 1, Point{X: down.X, Z: down.Z})
	reach(up.Name+" (chegada)", 1, Point{X: up.ToX, Z: up.ToZ})
	mobs, boss := 0, 0
	for _, m := range d.Spawns {
		reach(m.Monster, m.Floor, Point{X: m.X, Z: m.Z})
		mobs += max(1, m.Count)
		if !d.Floors[m.Floor].Contains(Point{X: m.X, Z: m.Z}) {
			t.Errorf("%s fora do andar %d", m.Monster, m.Floor+1)
		}
		if m.Monster == "abade_ulric" {
			boss++
		}
	}
	if boss != 1 {
		t.Errorf("%d chefes, esperava 1", boss)
	}
	if mobs < 25 || mobs+4 > 40 {
		t.Errorf("%d monstros (+4 invocados): o roteiro pede ~30 e no máximo 40", mobs)
	}
}
