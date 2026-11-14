package game

import (
	"math"
	"math/rand/v2"
	"testing"
)

// Setor de exemplo no formato do Bosque Gris (roadmap 5.1): noroeste dos anéis 2 e 3.
func testSector(monster string, count int, when When) Spawn {
	return Spawn{Monster: monster, Count: count, RMin: 80, RMax: 150, LevelMin: 7, LevelMax: 15, Respawn: 30,
		AngleMin: 100, AngleMax: 170, Region: "Bosque de Teste", When: when}
}

func TestSectorSpanAndAngleConvention(t *testing.T) {
	cases := []struct{ min, max, start, span float64 }{
		{100, 170, 100, 70},
		{340, 20, 340, 40}, // cruza o 0°
		{-30, 30, 330, 60},
		{0, 360, 0, 360},
		{370, 380, 10, 10},
	}
	for _, c := range cases {
		s := Spawn{AngleMin: c.min, AngleMax: c.max}
		if !s.Sector() {
			t.Fatalf("%v–%v devia ser setor", c.min, c.max)
		}
		start, span := s.SectorSpan()
		if math.Abs(start-c.start) > 1e-9 || math.Abs(span-c.span) > 1e-9 {
			t.Errorf("%v–%v: começo %v e largura %v, esperava %v e %v", c.min, c.max, start, span, c.start, c.span)
		}
	}
	if (Spawn{}).Sector() {
		t.Error("ângulos zerados são o anel inteiro, não um setor")
	}
	// 0° = leste (+X), 90° = norte (+Z), 180° = oeste, 270° = sul.
	for _, c := range []struct{ x, z, deg float64 }{{1, 0, 0}, {0, 1, 90}, {-1, 0, 180}, {0, -1, 270}, {-1, 1, 135}} {
		if got := AngleDeg(c.x, c.z); math.Abs(got-c.deg) > 1e-9 {
			t.Errorf("AngleDeg(%v, %v) = %v, esperava %v", c.x, c.z, got, c.deg)
		}
	}
}

func TestSectorSpawnsStayInsideTheSector(t *testing.T) {
	east := Spawn{Monster: "skeleton_minion", Count: 300, RMin: 46, RMax: 74, LevelMin: 1, LevelMax: 5,
		AngleMin: 340, AngleMax: 20, Region: "Leste de Teste"}
	for seed := uint64(1); seed <= 10; seed++ {
		rng := rand.New(rand.NewPCG(seed, seed))
		var north, south int // dos dois lados do 0°
		for _, sp := range ExpandSpawns([]Spawn{testSector("skeleton_rogue", 300, WhenAny), east}, rng) {
			r, a := math.Hypot(sp.X, sp.Z), AngleDeg(sp.X, sp.Z)
			if sp.Region == "Bosque de Teste" {
				if a < 100-1e-9 || a > 170+1e-9 || r < 80-1e-9 || r > 150+1e-9 {
					t.Fatalf("seed %d: monstro do setor 100°–170° em %.1f° e r %.1f", seed, a, r)
				}
				if sp.Level < 7 || sp.Level > 15 {
					t.Fatalf("seed %d: nível %d fora da faixa do setor", seed, sp.Level)
				}
				continue
			}
			if a > 20+1e-9 && a < 340-1e-9 {
				t.Fatalf("seed %d: monstro do setor 340°–20° em %.1f°", seed, a)
			}
			if r < 46-1e-9 || r > 74+1e-9 {
				t.Fatalf("seed %d: monstro do setor leste a r %.1f", seed, r)
			}
			if a >= 340 {
				south++
			} else {
				north++
			}
		}
		if north < 100 || south < 100 {
			t.Fatalf("seed %d: o setor que cruza o 0° não foi sorteado dos dois lados (%d/%d)", seed, north, south)
		}
	}
}

func TestRingsDoNotSpawnInsideOwnedAreas(t *testing.T) {
	lake := Spawn{Monster: "skeleton_mage", Count: 4, Area: true, X: -95, Z: 0, Radius: 14, LevelMin: 8, LevelMax: 10, Region: "Lago de Teste"}
	unowned := Spawn{Monster: "stone_golem", Count: 2, Area: true, X: 0, Z: 94, Radius: 10, LevelMin: 8, LevelMax: 10} // sem Region: não tem dono
	spawns := append(DefaultSpawns(), testSector("skeleton_rogue", 10, WhenDay), testSector("skeleton_mage", 10, WhenNight), lake, unowned)
	sector := testSector("", 0, WhenAny)
	var inUnowned int
	for seed := uint64(1); seed <= 20; seed++ {
		for _, sp := range ExpandSpawns(spawns, rand.New(rand.NewPCG(seed, seed))) {
			if sp.Monster.Raid || sp.Region != "" || sp.Monster.ID == "stone_golem" { // só os anéis sem dono
				continue
			}
			if sector.Contains(sp.X, sp.Z, 0) {
				t.Fatalf("seed %d: %s do anel dentro do setor com dono (%.1f°, r %.1f)", seed, sp.Monster.ID, AngleDeg(sp.X, sp.Z), math.Hypot(sp.X, sp.Z))
			}
			if lake.Contains(sp.X, sp.Z, 0) {
				t.Fatalf("seed %d: %s do anel dentro do disco com dono", seed, sp.Monster.ID)
			}
			if d := math.Hypot(sp.X-RaidLairX, sp.Z-RaidLairZ); d <= RaidLairRadius+ownedMargin {
				t.Fatalf("seed %d: %s dentro do covil (%.1f m)", seed, sp.Monster.ID, d)
			}
			if unowned.Contains(sp.X, sp.Z, 0) {
				inUnowned++
			}
		}
	}
	if inUnowned == 0 {
		t.Error("um disco sem Region não tem dono: os anéis podiam sortear lá dentro")
	}
}

// A decisão da T2: spawn novo vai para o fim e não muda o id (a posição na lista) de quem
// já existia. Sem área com dono, nem as posições mudam.
func TestAppendingSpawnsKeepsExistingMonsters(t *testing.T) {
	base := DefaultSpawns()
	rng := func() *rand.Rand { return rand.New(rand.NewPCG(20261112, 7)) }
	before := ExpandSpawns(base, rng())

	plain := ExpandSpawns(append(DefaultSpawns(), Spawn{Monster: "skeleton_minion", Count: 3, RMin: 46, RMax: 74, LevelMin: 1, LevelMax: 5, When: WhenNight}), rng())
	for i, p := range before {
		if plain[i].Monster != p.Monster || plain[i].X != p.X || plain[i].Z != p.Z || plain[i].Level != p.Level {
			t.Fatalf("monstro %d mudou ao acrescentar um spawn sem dono no fim: %+v → %+v", i, p, plain[i])
		}
	}
	if len(plain) != len(before)+3 || plain[len(before)].When != WhenNight {
		t.Fatalf("o spawn novo devia vir no fim: %d pontos", len(plain))
	}

	owned := ExpandSpawns(append(DefaultSpawns(), testSector("skeleton_rogue", 9, WhenAny)), rng())
	moved := 0
	for i, p := range before {
		if owned[i].Monster != p.Monster {
			t.Fatalf("monstro %d virou %s com o setor novo", i, owned[i].Monster.ID)
		}
		if owned[i].X != p.X || owned[i].Z != p.Z {
			moved++
		}
	}
	if moved == 0 {
		t.Error("ninguém do anel saiu do setor novo (devia mudar de lugar, mantendo o id)")
	}
}

func TestWhenActive(t *testing.T) {
	for _, c := range []struct {
		w            When
		day, atNight bool
	}{{WhenAny, true, true}, {WhenDay, true, false}, {WhenNight, false, true}} {
		if c.w.Active(false) != c.day || c.w.Active(true) != c.atNight {
			t.Errorf("%v: de dia %v, à noite %v", c.w, c.w.Active(false), c.w.Active(true))
		}
	}
}

// Numa região, os noturnos trocam os do dia: as duas contas têm que bater.
func TestDefaultSpawnsBalanceDayAndNight(t *testing.T) {
	day, night := map[string]int{}, map[string]int{}
	for _, s := range DefaultSpawns() {
		switch s.When {
		case WhenDay:
			day[s.Region] += s.Count
		case WhenNight:
			night[s.Region] += s.Count
		}
	}
	for r, n := range day {
		if night[r] != n {
			t.Errorf("região %q: %d de dia e %d à noite", r, n, night[r])
		}
	}
	for r, n := range night {
		if day[r] != n {
			t.Errorf("região %q: %d à noite e %d de dia", r, n, day[r])
		}
	}
}

// O círculo de caça das missões respeita o setor e as áreas com dono.
func TestHuntPlaceRespectsSectorsAndOwnedAreas(t *testing.T) {
	spawns := append(DefaultSpawns(), testSector("skeleton_rogue", 9, WhenAny))
	sector := testSector("", 0, WhenAny)
	for _, at := range [][2]float64{{120, 0}, {0, -120}, {-10, 120}, {-120, 10}, {-100, 90}, {60, 60}} {
		// Um monstro só do setor: o círculo fica dentro dele, venha o jogador de onde vier.
		pl, ok := HuntPlace([]Spawn{testSector("skeleton_rogue", 9, WhenAny)}, []string{"skeleton_rogue"}, 10, at[0], at[1])
		if !ok || !sector.Contains(pl.X, pl.Z, 0) {
			t.Errorf("jogador em %v: círculo do setor fora dele (%.1f°, r %.1f)", at, AngleDeg(pl.X, pl.Z), math.Hypot(pl.X, pl.Z))
		}
		if a := AngleDeg(pl.X, pl.Z); a < 100+0.1 || a > 170-0.1 {
			t.Errorf("jogador em %v: círculo do setor na borda de fora (%.1f°)", at, a)
		}
		// O anel sem dono, com o jogador no noroeste: o círculo sai do setor.
		pl, ok = HuntPlace(spawns, []string{"skeleton_warrior"}, 9, at[0], at[1])
		if !ok || sector.Contains(pl.X, pl.Z, 0) {
			t.Errorf("jogador em %v: círculo do anel dentro do setor com dono (%.1f°, r %.1f)", at, AngleDeg(pl.X, pl.Z), math.Hypot(pl.X, pl.Z))
		}
	}
}
