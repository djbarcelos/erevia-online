// Package world carrega o layout do mapa (world.json, gerado por
// client/scripts/build-world.mjs) e responde perguntas de colisão e caminho:
// um ponto está livre? a linha reta está livre? qual caminho contorna os obstáculos?
package world

import (
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"sync"
)

//go:embed world.json
var embedded []byte

// AgentRadius é o raio de um personagem na colisão (igual ao cliente).
const AgentRadius = 0.4

const (
	cellSize     = 1.0    // metros por célula da grade de caminhos
	maxExpanded  = 120000 // o mapa vai até o Vale das Pedras Vivas: caminhos longos
	sampleStep   = 0.5    // passo da checagem de linha reta (m)
	bucketSize   = 4.0    // balde espacial para a checagem exata de colisão
	cellClearPad = 0.3    // folga extra ao marcar células bloqueadas (caminhos não raspam)
)

type Point struct{ X, Z float64 }

type Collider struct {
	X, Z, R float64
}

// Bounds é o retângulo jogável do mapa (o mapa cresce só para o sul, então não é
// um quadrado centrado na cidade).
type Bounds struct {
	MinX float64 `json:"minX"`
	MaxX float64 `json:"maxX"`
	MinZ float64 `json:"minZ"`
	MaxZ float64 `json:"maxZ"`
}

// Square é o quadrado de lado size centrado na origem.
func Square(size float64) Bounds {
	h := size / 2
	return Bounds{MinX: -h, MaxX: h, MinZ: -h, MaxZ: h}
}

// Clamp prende o ponto dentro do retângulo.
func (b Bounds) Clamp(p Point) Point {
	return Point{X: min(max(p.X, b.MinX), b.MaxX), Z: min(max(p.Z, b.MinZ), b.MaxZ)}
}

// Contains: o ponto está dentro (bordas incluídas)?
func (b Bounds) Contains(p Point) bool {
	return p.X >= b.MinX && p.X <= b.MaxX && p.Z >= b.MinZ && p.Z <= b.MaxZ
}

type World struct {
	// Size é o lado do quadrado original (compatibilidade); os limites reais estão em Bounds.
	Size       float64
	Bounds     Bounds
	SafeRadius float64
	colliders  []Collider
	buckets    map[[2]int][]int
	nx, nz     int       // células em X e em Z
	blocked    []bool    // grade nx×nz para o A*
	pool       sync.Pool // *astarScratch: memória do A* reaproveitada (seguro entre goroutines)
}

// fileFormat é o que o servidor lê do world.json. Os biomas (`biomes[]`, ou o `biome`
// único do formato antigo) só servem ao cliente, que pinta o chão com eles: a colisão
// e os limites vêm dos campos de cima, iguais nos dois formatos.
type fileFormat struct {
	Size       float64    `json:"size"`
	Bounds     *Bounds    `json:"bounds"`
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

// Os outros mapas (maps/<id>.json, gerados por client/scripts/build-maps.mjs).
//
//go:embed maps/*.json
var embeddedMaps embed.FS

// StarterMap é o id do mapa principal (world.json): o Vale de Vel'Aran.
const StarterMap = "starter"

// Load lê o mapa embutido no binário.
func Load() (*World, error) { return parse(embedded) }

// LoadMap lê um mapa pelo id ("starter" ou "" = o principal; senão maps/<id>.json).
func LoadMap(id string) (*World, error) {
	if id == "" || id == StarterMap {
		return Load()
	}
	data, err := embeddedMaps.ReadFile("maps/" + id + ".json")
	if err != nil {
		return nil, fmt.Errorf("mapa %q: %w", id, err)
	}
	return parseNamed(id+".json", data)
}

func parse(data []byte) (*World, error) { return parseNamed("world.json", data) }

func parseNamed(name string, data []byte) (*World, error) {
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if f.Size <= 0 {
		return nil, fmt.Errorf("%s: size inválido", name)
	}
	b := Square(f.Size)
	if f.Bounds != nil {
		b = *f.Bounds
		if b.MaxX <= b.MinX || b.MaxZ <= b.MinZ {
			return nil, fmt.Errorf("%s: bounds inválido", name)
		}
	}
	w := NewBounds(b, f.Colliders)
	w.Size = f.Size
	w.SafeRadius = f.SafeRadius
	return w, nil
}

// New monta um mundo quadrado centrado na origem (usado nos testes).
func New(size float64, colliders []Collider) *World {
	w := NewBounds(Square(size), colliders)
	w.Size = size
	return w
}

// NewBounds monta um mundo retangular a partir de colisores.
func NewBounds(b Bounds, colliders []Collider) *World {
	w := &World{Size: max(b.MaxX-b.MinX, b.MaxZ-b.MinZ), Bounds: b, colliders: colliders, buckets: map[[2]int][]int{}}
	for i, c := range colliders {
		r := c.R + AgentRadius
		for bx := bucket(c.X - r); bx <= bucket(c.X+r); bx++ {
			for bz := bucket(c.Z - r); bz <= bucket(c.Z+r); bz++ {
				k := [2]int{bx, bz}
				w.buckets[k] = append(w.buckets[k], i)
			}
		}
	}
	w.nx = int(math.Ceil((b.MaxX - b.MinX) / cellSize))
	w.nz = int(math.Ceil((b.MaxZ - b.MinZ) / cellSize))
	w.blocked = make([]bool, w.nx*w.nz)
	for gz := 0; gz < w.nz; gz++ {
		for gx := 0; gx < w.nx; gx++ {
			p := w.cellCenter(gx, gz)
			w.blocked[gz*w.nx+gx] = w.blockedPad(p.X, p.Z, cellClearPad)
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
	for r := 1; r < max(w.nx, w.nz); r++ {
		best, bestD := Point{}, math.Inf(1)
		for dz := -r; dz <= r; dz++ {
			for dx := -r; dx <= r; dx++ {
				if max(abs(dx), abs(dz)) != r {
					continue
				}
				x, z := gx+dx, gz+dz
				if !w.inGrid(x, z) || w.blocked[z*w.nx+x] {
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
		pts = append(pts, w.cellCenter(c%w.nx, c/w.nx))
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
	if !w.inGrid(gx, gz) || w.blocked[gz*w.nx+gx] {
		// q pode estar livre sem que o centro da célula esteja: procura a célula livre mais perto.
		for r := 0; r < 4; r++ {
			for dz := -r; dz <= r; dz++ {
				for dx := -r; dx <= r; dx++ {
					x, z := gx+dx, gz+dz
					if w.inGrid(x, z) && !w.blocked[z*w.nx+x] {
						return z*w.nx + x
					}
				}
			}
		}
		return -1
	}
	return gz*w.nx + gx
}

func (w *World) toCell(p Point) (int, int) {
	gx := int(math.Floor((p.X - w.Bounds.MinX) / cellSize))
	gz := int(math.Floor((p.Z - w.Bounds.MinZ) / cellSize))
	return min(max(gx, 0), w.nx-1), min(max(gz, 0), w.nz-1)
}

func (w *World) cellCenter(gx, gz int) Point {
	return Point{X: w.Bounds.MinX + (float64(gx)+0.5)*cellSize, Z: w.Bounds.MinZ + (float64(gz)+0.5)*cellSize}
}

func (w *World) inGrid(x, z int) bool { return x >= 0 && z >= 0 && x < w.nx && z < w.nz }

// ---------- A* (8 direções, sem cortar quinas) ----------

// openNode é uma célula na fila do A* (por valor: nada de ponteiro por nó).
type openNode struct {
	cell int32
	f    float64
}

// astarScratch é a memória de uma busca, reaproveitada entre buscas: uma grade inteira
// de g e de pai, e um carimbo por busca que diz se a célula já foi vista (ou fechada)
// nesta busca, sem precisar limpar nada. Antes eram mapas criados a cada busca, e um
// caminho longo (dezenas de milhares de células) gastava mais com alocação e GC do que
// com o A* em si.
type astarScratch struct {
	stamp  uint32
	seen   []uint32 // seen[c] == stamp: g[c] e parent[c] valem nesta busca
	closed []uint32 // closed[c] == stamp: já expandida
	g      []float64
	parent []int32
	open   []openNode
}

func (w *World) scratch() *astarScratch {
	if s, ok := w.pool.Get().(*astarScratch); ok {
		return s
	}
	n := w.nx * w.nz
	return &astarScratch{seen: make([]uint32, n), closed: make([]uint32, n), g: make([]float64, n), parent: make([]int32, n)}
}

func (s *astarScratch) begin() {
	s.stamp++
	if s.stamp == 0 { // deu a volta: limpa os carimbos uma vez
		clear(s.seen)
		clear(s.closed)
		s.stamp = 1
	}
	s.open = s.open[:0]
}

// push e pop: heap binário de mínimo por f.
func (s *astarScratch) push(n openNode) {
	s.open = append(s.open, n)
	i := len(s.open) - 1
	for i > 0 {
		p := (i - 1) / 2
		if s.open[p].f <= s.open[i].f {
			break
		}
		s.open[p], s.open[i] = s.open[i], s.open[p]
		i = p
	}
}

func (s *astarScratch) pop() openNode {
	o := s.open
	top := o[0]
	last := len(o) - 1
	o[0] = o[last]
	o = o[:last]
	for i := 0; ; {
		l, r, m := 2*i+1, 2*i+2, i
		if l < len(o) && o[l].f < o[m].f {
			m = l
		}
		if r < len(o) && o[r].f < o[m].f {
			m = r
		}
		if m == i {
			break
		}
		o[i], o[m] = o[m], o[i]
		i = m
	}
	s.open = o
	return top
}

var dirs = [8][3]float64{
	{1, 0, 1}, {-1, 0, 1}, {0, 1, 1}, {0, -1, 1},
	{1, 1, math.Sqrt2}, {1, -1, math.Sqrt2}, {-1, 1, math.Sqrt2}, {-1, -1, math.Sqrt2},
}

func (w *World) astar(start, goal int) []int {
	gx, gz := goal%w.nx, goal/w.nx
	h := func(c int) float64 {
		dx, dz := float64(abs(c%w.nx-gx)), float64(abs(c/w.nx-gz))
		return math.Max(dx, dz) + (math.Sqrt2-1)*math.Min(dx, dz)
	}
	s := w.scratch()
	defer w.pool.Put(s)
	s.begin()
	st := s.stamp
	s.seen[start], s.g[start], s.parent[start] = st, 0, -1
	s.push(openNode{cell: int32(start), f: h(start)})
	best, bestH := start, h(start)

	for expanded := 0; len(s.open) > 0 && expanded < maxExpanded; expanded++ {
		cur := int(s.pop().cell)
		if cur == goal {
			best = goal
			break
		}
		if s.closed[cur] == st {
			continue
		}
		s.closed[cur] = st
		if hc := h(cur); hc < bestH {
			best, bestH = cur, hc
		}
		cx, cz := cur%w.nx, cur/w.nx
		gc := s.g[cur]
		for _, d := range dirs {
			nx, nz := cx+int(d[0]), cz+int(d[1])
			if !w.inGrid(nx, nz) || w.blocked[nz*w.nx+nx] {
				continue
			}
			if d[0] != 0 && d[1] != 0 && (w.blocked[cz*w.nx+nx] || w.blocked[nz*w.nx+cx]) {
				continue // não corta quinas
			}
			next := nz*w.nx + nx
			ng := gc + d[2]
			if s.seen[next] == st && ng >= s.g[next] {
				continue
			}
			s.seen[next], s.g[next], s.parent[next] = st, ng, int32(cur)
			s.push(openNode{cell: int32(next), f: ng + h(next)})
		}
	}

	// Reconstrói até o melhor nó alcançado (o objetivo, ou o mais perto dele).
	var path []int
	for c := best; c != start; {
		path = append(path, c)
		p := s.parent[c]
		if p < 0 {
			break
		}
		c = int(p)
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
