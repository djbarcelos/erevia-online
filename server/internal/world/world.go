// Package world carrega o layout do mapa (world.json, gerado por
// client/scripts/build-world.mjs) e responde perguntas de colisão e caminho:
// um ponto está livre? a linha reta está livre? qual caminho contorna os obstáculos?
package world

import (
	"container/heap"
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
)

//go:embed world.json
var embedded []byte

// AgentRadius é o raio de um personagem na colisão (igual ao cliente).
const AgentRadius = 0.4

const (
	cellSize     = 1.0 // metros por célula da grade de caminhos
	maxExpanded  = 20000
	sampleStep   = 0.5 // passo da checagem de linha reta (m)
	bucketSize   = 4.0 // balde espacial para a checagem exata de colisão
	cellClearPad = 0.3 // folga extra ao marcar células bloqueadas (caminhos não raspam)
)

type Point struct{ X, Z float64 }

type Collider struct {
	X, Z, R float64
}

type World struct {
	Size       float64
	SafeRadius float64
	colliders  []Collider
	buckets    map[[2]int][]int
	n          int    // células por lado
	blocked    []bool // grade n×n para o A*
}

type fileFormat struct {
	Size       float64    `json:"size"`
	SafeRadius float64    `json:"safeRadius"`
	Colliders  []Collider `json:"colliders"`
}

func (c *Collider) UnmarshalJSON(b []byte) error {
	var v struct{ X, Z, R float64 }
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*c = Collider{X: v.X, Z: v.Z, R: v.R}
	return nil
}

// Load lê o mapa embutido no binário.
func Load() (*World, error) {
	var f fileFormat
	if err := json.Unmarshal(embedded, &f); err != nil {
		return nil, fmt.Errorf("world.json: %w", err)
	}
	if f.Size <= 0 {
		return nil, fmt.Errorf("world.json: size inválido")
	}
	w := New(f.Size, f.Colliders)
	w.SafeRadius = f.SafeRadius
	return w, nil
}

// New monta um mundo a partir de colisores (usado também nos testes).
func New(size float64, colliders []Collider) *World {
	w := &World{Size: size, colliders: colliders, buckets: map[[2]int][]int{}}
	for i, c := range colliders {
		r := c.R + AgentRadius
		for bx := bucket(c.X - r); bx <= bucket(c.X+r); bx++ {
			for bz := bucket(c.Z - r); bz <= bucket(c.Z+r); bz++ {
				k := [2]int{bx, bz}
				w.buckets[k] = append(w.buckets[k], i)
			}
		}
	}
	w.n = int(math.Ceil(size / cellSize))
	w.blocked = make([]bool, w.n*w.n)
	for gz := 0; gz < w.n; gz++ {
		for gx := 0; gx < w.n; gx++ {
			p := w.cellCenter(gx, gz)
			w.blocked[gz*w.n+gx] = w.blockedPad(p.X, p.Z, cellClearPad)
		}
	}
	return w
}

func bucket(v float64) int { return int(math.Floor(v / bucketSize)) }

// Blocked: o personagem (raio AgentRadius) encostaria num obstáculo em (x, z)?
func (w *World) Blocked(x, z float64) bool { return w.blockedPad(x, z, 0) }

func (w *World) blockedPad(x, z, pad float64) bool {
	for _, i := range w.buckets[[2]int{bucket(x), bucket(z)}] {
		c := w.colliders[i]
		if math.Hypot(c.X-x, c.Z-z) < c.R+AgentRadius+pad {
			return true
		}
	}
	return false
}

// SegmentClear diz se dá para andar em linha reta de a até b.
func (w *World) SegmentClear(a, b Point) bool {
	d := math.Hypot(b.X-a.X, b.Z-a.Z)
	n := int(math.Ceil(d / sampleStep))
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		if w.Blocked(a.X+(b.X-a.X)*t, a.Z+(b.Z-a.Z)*t) {
			return false
		}
	}
	return true
}

// NearestFree devolve o ponto livre mais próximo (procura em anéis de células).
func (w *World) NearestFree(p Point) Point {
	if !w.Blocked(p.X, p.Z) {
		return p
	}
	gx, gz := w.toCell(p)
	for r := 1; r < w.n; r++ {
		best, bestD := Point{}, math.Inf(1)
		for dz := -r; dz <= r; dz++ {
			for dx := -r; dx <= r; dx++ {
				if max(abs(dx), abs(dz)) != r {
					continue
				}
				x, z := gx+dx, gz+dz
				if !w.inGrid(x, z) || w.blocked[z*w.n+x] {
					continue
				}
				c := w.cellCenter(x, z)
				if d := math.Hypot(c.X-p.X, c.Z-p.Z); d < bestD {
					best, bestD = c, d
				}
			}
		}
		if !math.IsInf(bestD, 1) {
			return best
		}
	}
	return p
}

// FindPath devolve os pontos para ir de a até b contornando obstáculos (sem incluir a).
// Se b estiver bloqueado, vai até o ponto livre mais próximo dele. Caminho vazio = não dá.
func (w *World) FindPath(a, b Point) []Point {
	b = w.NearestFree(b)
	if w.SegmentClear(a, b) {
		return []Point{b}
	}
	start, goal := w.nearestFreeCell(a), w.nearestFreeCell(b)
	if start < 0 || goal < 0 {
		return nil
	}
	cells := w.astar(start, goal)
	if len(cells) == 0 {
		return nil
	}
	pts := make([]Point, 0, len(cells)+1)
	for _, c := range cells {
		pts = append(pts, w.cellCenter(c%w.n, c/w.n))
	}
	if cells[len(cells)-1] == goal {
		pts[len(pts)-1] = b // termina no ponto exato pedido
	}
	return w.smooth(a, pts)
}

// smooth remove pontos intermediários quando há linha reta livre até um ponto mais à frente.
func (w *World) smooth(from Point, pts []Point) []Point {
	var out []Point
	cur := from
	i := 0
	for i < len(pts) {
		j := len(pts) - 1
		for j > i && !w.SegmentClear(cur, pts[j]) {
			j--
		}
		out = append(out, pts[j])
		cur = pts[j]
		i = j + 1
	}
	return out
}

func (w *World) nearestFreeCell(p Point) int {
	q := w.NearestFree(p)
	gx, gz := w.toCell(q)
	if !w.inGrid(gx, gz) || w.blocked[gz*w.n+gx] {
		// q pode estar livre sem que o centro da célula esteja: procura a célula livre mais perto.
		for r := 0; r < 4; r++ {
			for dz := -r; dz <= r; dz++ {
				for dx := -r; dx <= r; dx++ {
					x, z := gx+dx, gz+dz
					if w.inGrid(x, z) && !w.blocked[z*w.n+x] {
						return z*w.n + x
					}
				}
			}
		}
		return -1
	}
	return gz*w.n + gx
}

func (w *World) toCell(p Point) (int, int) {
	half := w.Size / 2
	gx := int(math.Floor((p.X + half) / cellSize))
	gz := int(math.Floor((p.Z + half) / cellSize))
	return min(max(gx, 0), w.n-1), min(max(gz, 0), w.n-1)
}

func (w *World) cellCenter(gx, gz int) Point {
	half := w.Size / 2
	return Point{X: -half + (float64(gx)+0.5)*cellSize, Z: -half + (float64(gz)+0.5)*cellSize}
}

func (w *World) inGrid(x, z int) bool { return x >= 0 && z >= 0 && x < w.n && z < w.n }

// ---------- A* (8 direções, sem cortar quinas) ----------

type node struct {
	cell int
	f    float64
	idx  int
}
type openSet []*node

func (o openSet) Len() int           { return len(o) }
func (o openSet) Less(i, j int) bool { return o[i].f < o[j].f }
func (o openSet) Swap(i, j int)      { o[i], o[j] = o[j], o[i]; o[i].idx = i; o[j].idx = j }
func (o *openSet) Push(x any)        { n := x.(*node); n.idx = len(*o); *o = append(*o, n) }
func (o *openSet) Pop() any          { old := *o; n := old[len(old)-1]; *o = old[:len(old)-1]; return n }

var dirs = [8][3]float64{
	{1, 0, 1}, {-1, 0, 1}, {0, 1, 1}, {0, -1, 1},
	{1, 1, math.Sqrt2}, {1, -1, math.Sqrt2}, {-1, 1, math.Sqrt2}, {-1, -1, math.Sqrt2},
}

func (w *World) astar(start, goal int) []int {
	gx, gz := goal%w.n, goal/w.n
	h := func(c int) float64 {
		dx, dz := float64(abs(c%w.n-gx)), float64(abs(c/w.n-gz))
		return math.Max(dx, dz) + (math.Sqrt2-1)*math.Min(dx, dz)
	}
	g := map[int]float64{start: 0}
	parent := map[int]int{}
	closed := map[int]bool{}
	open := &openSet{}
	heap.Push(open, &node{cell: start, f: h(start)})
	best, bestH := start, h(start)

	for expanded := 0; open.Len() > 0 && expanded < maxExpanded; expanded++ {
		cur := heap.Pop(open).(*node).cell
		if cur == goal {
			best = goal
			break
		}
		if closed[cur] {
			continue
		}
		closed[cur] = true
		if hc := h(cur); hc < bestH {
			best, bestH = cur, hc
		}
		cx, cz := cur%w.n, cur/w.n
		for _, d := range dirs {
			nx, nz := cx+int(d[0]), cz+int(d[1])
			if !w.inGrid(nx, nz) || w.blocked[nz*w.n+nx] {
				continue
			}
			if d[0] != 0 && d[1] != 0 && (w.blocked[cz*w.n+nx] || w.blocked[nz*w.n+cx]) {
				continue // não corta quinas
			}
			next := nz*w.n + nx
			ng := g[cur] + d[2]
			if old, ok := g[next]; ok && ng >= old {
				continue
			}
			g[next] = ng
			parent[next] = cur
			heap.Push(open, &node{cell: next, f: ng + h(next)})
		}
	}

	// Reconstrói até o melhor nó alcançado (o objetivo, ou o mais perto dele).
	var path []int
	for c := best; c != start; {
		path = append(path, c)
		p, ok := parent[c]
		if !ok {
			break
		}
		c = p
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
