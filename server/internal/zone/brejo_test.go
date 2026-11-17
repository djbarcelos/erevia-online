package zone

import (
	"math"
	"math/rand/v2"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/world"
)

// O Brejo de Lamagris (tunnel#139) cabe no orçamento do roadmap: 36 monstros ao mesmo tempo (de dia e de
// noite), do 14 ao 20, o pico de monstros num raio de 70 m (a área de interesse do cliente) fica em 16 ou
// menos, e todos nascem em chão livre dentro do bioma.
func TestBrejoBudget(t *testing.T) {
	w, err := world.Load()
	if err != nil {
		t.Fatal(err)
	}
	if w.Bounds.MaxX != game.BrejoMaxX || w.Bounds.MinZ > -300 {
		t.Fatalf("os limites do mapa não têm o Brejo: %+v", w.Bounds)
	}
	for seed := uint64(1); seed <= 12; seed++ {
		points := game.ExpandSpawns(game.DefaultSpawns(), rand.New(rand.NewPCG(seed, seed*7+3)))
		for _, night := range []bool{false, true} {
			var brejo []game.SpawnPoint
			for _, p := range points {
				if p.Region == game.BrejoRegion && p.Monster.ID != "mae_lodo" && p.When.Active(night) {
					brejo = append(brejo, p)
				}
			}
			if len(brejo) != 36 {
				t.Fatalf("seed %d, noite %v: %d monstros no Brejo, quero 36", seed, night, len(brejo))
			}
			for _, p := range brejo {
				if p.X < game.BrejoMinX || p.X > game.BrejoMaxX || p.Z < game.BrejoMinZ || p.Z > game.BrejoMaxZ {
					t.Errorf("%s em (%.0f, %.0f) fora do Brejo", p.Monster.ID, p.X, p.Z)
				}
				if p.Level < 14 || p.Level > 20 {
					t.Errorf("%s no nível %d", p.Monster.ID, p.Level)
				}
				if f := w.NearestFree(world.Point{X: p.X, Z: p.Z}); math.Hypot(f.X-p.X, f.Z-p.Z) > 3 {
					t.Errorf("seed %d: %s em (%.0f, %.0f) preso em obstáculos (livre a %.1f m)", seed, p.Monster.ID, p.X, p.Z, math.Hypot(f.X-p.X, f.Z-p.Z))
				}
			}
			peak := 0
			for x := game.BrejoMinX - 40; x <= game.BrejoMaxX; x += 5 {
				for z := game.BrejoMinZ - 40; z <= game.BrejoMaxZ+40; z += 5 {
					n := 0
					for _, p := range brejo {
						if math.Hypot(p.X-x, p.Z-z) <= 70 {
							n++
						}
					}
					peak = max(peak, n)
				}
			}
			if peak > 16 {
				t.Errorf("seed %d, noite %v: pico de %d monstros num raio de 70 m (o teto é 16)", seed, night, peak)
			}
		}
	}
}

// A lagoa de Vau Velho: a Mãe-Lodo nasce no meio dela, em chão livre; o cais e a ilha também, e a parte funda
// (o fosso da ilha e a aldeia afogada) não deixa ninguém passar a pé.
func TestBrejoWaterAndPlaces(t *testing.T) {
	w, err := world.Load()
	if err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]world.Point{
		"Mãe-Lodo": {X: game.MotherX, Z: game.MotherZ}, "cais": {X: game.DockX, Z: game.DockZ},
		"ilha": {X: game.IslandX, Z: game.IslandZ}, "posto": {X: game.PostX, Z: game.PostZ},
	} {
		if w.Blocked(p.X, p.Z) {
			t.Errorf("%s em (%.0f, %.0f) está bloqueado", name, p.X, p.Z)
		}
	}
	// Do cais à ilha não há caminho a pé: o fosso é de colisor (só o barco atravessa).
	path := w.FindPath(world.Point{X: game.DockX, Z: game.DockZ}, world.Point{X: game.IslandX, Z: game.IslandZ})
	if n := len(path); n > 0 && math.Hypot(path[n-1].X-game.IslandX, path[n-1].Z-game.IslandZ) < 3 {
		t.Errorf("dá para andar do cais até a ilha: %v", path[n-1])
	}
	// A estrada leste vai do portão até o posto sem obstáculo, e as montanhas sem acesso fecham o resto.
	if !w.SegmentClear(world.Point{X: 60, Z: 0}, world.Point{X: game.PostX, Z: 0}) {
		t.Error("a estrada leste até o posto está bloqueada")
	}
	for _, p := range []world.Point{{X: 200, Z: 0}, {X: 300, Z: -60}, {X: 160, Z: 100}} {
		if far := w.FindPath(world.Point{X: 0, Z: 0}, p); len(far) == 0 {
			t.Errorf("sem caminho da cidade até (%.0f, %.0f)", p.X, p.Z)
		}
	}
	for _, sealed := range []world.Point{{X: 300, Z: -250}, {X: 320, Z: 120}} {
		path := w.FindPath(world.Point{X: 0, Z: 0}, sealed)
		if n := len(path); n > 0 && math.Hypot(path[n-1].X-sealed.X, path[n-1].Z-sealed.Z) < 5 {
			t.Errorf("dá para chegar às montanhas sem acesso, em (%.0f, %.0f)", sealed.X, sealed.Z)
		}
	}
}
