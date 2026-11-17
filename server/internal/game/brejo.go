package game

// Brejo de Lamagris (tunnel#139, roadmap 5.3): a expansão leste do Mapa 1, x de 150 a 330 e
// z de −150 a 60, do nível 14 (na borda oeste, junto ao Posto do Barqueiro) ao 20 (no leste).
// Aqui ficam os lugares e os spawns; os monstros estão em monsters.go.

const (
	// BrejoRegion: o nome da região (spawns, mapa e missões usam o mesmo).
	BrejoRegion = "Brejo de Lamagris"
	// O retângulo do bioma (igual a client/scripts/biomes/brejo.mjs).
	BrejoMinX, BrejoMaxX = 150.0, 330.0
	BrejoMinZ, BrejoMaxZ = -150.0, 60.0
	// A lagoa de Vau Velho: a Mãe-Lodo fica no meio dela (a água é rasa e andável ali).
	MotherX, MotherZ = 250.0, -40.0
	// Posto do Barqueiro: o fim da estrada leste, na entrada do Brejo.
	PostX, PostZ = 165.0, 0.0
	// Cais do Posto (o barco) e a Ilha dos Vaga-lumes, no meio da lagoa (igual ao cliente).
	DockX, DockZ     = 188.0, -10.0
	IslandX, IslandZ = 268.0, -20.0
)

// brejoGroup: um grupo de monstros do Brejo (um disco). Os níveis sobem de oeste para leste.
type brejoGroup struct {
	x, z, r  float64
	lo, hi   int
	lanceiro int // acampamento (a matilha: bater em um puxa todos)
	xama     int
	sapo     int // sempre
	sapoDay  int // só de dia (à noite, o mesmo tanto de Afogados)
	lodo     int
}

// brejoGroups: 36 monstros ao mesmo tempo, de dia e de noite (8 grupos de 4 a 5; 9 sapos, 7 sapos que de noite
// viram 7 Afogados, 8 Lodos, 8 Lanceiros e 4 Xamãs). Os centros saem de uma busca do menor pico num raio de 70 m
// (ficam a 50 m ou mais um do outro, fora da lagoa onde a Mãe-Lodo luta e longe do Posto).
var brejoGroups = []brejoGroup{
	{x: 172, z: -34, r: 12, lo: 14, hi: 15, sapo: 2, sapoDay: 1, lodo: 2},
	{x: 182, z: -116, r: 12, lo: 14, hi: 15, sapo: 1, sapoDay: 1, lodo: 2, xama: 1},
	{x: 193, z: 40, r: 12, lo: 15, hi: 16, sapo: 2, sapoDay: 1, lodo: 1, xama: 1},
	{x: 247, z: -122, r: 12, lo: 16, hi: 17, lanceiro: 4, xama: 1},
	{x: 257, z: 31, r: 12, lo: 17, hi: 18, sapo: 1, sapoDay: 2, lodo: 1},
	{x: 306, z: 43, r: 12, lo: 19, hi: 20, lanceiro: 4},
	{x: 313, z: -135, r: 12, lo: 19, hi: 20, sapo: 1, sapoDay: 1, lodo: 1, xama: 1},
	{x: 314, z: -49, r: 12, lo: 19, hi: 20, sapo: 2, sapoDay: 1, lodo: 1},
}

const brejoRespawn = 40.0

func brejoSpawns() []Spawn {
	var out []Spawn
	add := func(monster string, n int, g brejoGroup, r float64, when When) {
		if n == 0 {
			return
		}
		out = append(out, Spawn{Monster: monster, Count: n, Area: true, X: g.x, Z: g.z, Radius: r, LevelMin: g.lo, LevelMax: g.hi,
			Respawn: brejoRespawn, Region: BrejoRegion, When: when})
	}
	for _, g := range brejoGroups {
		add("brejeiro_lanceiro", g.lanceiro, g, 6, WhenAny) // o acampamento fica junto
		add("brejeiro_xama", g.xama, g, g.r, WhenAny)
		add("sapo_boi", g.sapo, g, g.r, WhenAny)
		add("sapo_boi", g.sapoDay, g, g.r, WhenDay)
		add("afogado", g.sapoDay, g, g.r, WhenNight)
		add("lodo_vivo", g.lodo, g, g.r, WhenAny)
	}
	// O chefe de campo, no meio da lagoa de Vau Velho: 50 a 100 min depois de cair.
	out = append(out, Spawn{Monster: "mae_lodo", Count: 1, Fixed: true, X: MotherX, Z: MotherZ, LevelMin: 21, Respawn: 3000, Region: BrejoRegion})
	return out
}

// BrejoHuntArea: a área (m²) que o simulador de equilíbrio usa para o Brejo (balance/hunt.go). O bioma tem uns 28.000 m²
// de caça e 36 monstros, mas só os 16 sapos entram na conta (os outros têm papel, ver BalanceBaseline): a fatia
// deles é 28.000 × 16/36, para o tempo entre um sapo e o próximo ser o do pântano inteiro, com todos os 36.
const BrejoHuntArea = 12400.0
