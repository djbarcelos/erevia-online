package game

import (
	"math"
	"math/rand/v2"
)

// MonsterDef descreve um tipo de monstro. Os atributos crescem por nível.
type MonsterDef struct {
	ID         string
	Name       string
	Model      string // modelo 3D no cliente
	HP, HPLvl  float64
	Atk, AtkLv float64
	Def, DefLv float64
	Range      float64
	Interval   float64
	Speed      float64
	Magic      bool
	Aggressive bool
	XPBase     float64
	Scale      float64 // tamanho visual (1 = normal)
	Boss       bool    // chefe (marcado no mapa)
	// Drops: moedas (por nível do monstro) e itens.
	GoldChance       float64
	GoldMin, GoldMax float64
	Loot             []LootEntry
}

var (
	lootMinion  = []LootEntry{{"bone_old", 0.4, 1, 2}, {"skull_cracked", 0.08, 1, 1}, {"potion_hp_s", 0.06, 1, 1}}
	lootFighter = []LootEntry{{"bone_old", 0.35, 1, 3}, {"skull_cracked", 0.15, 1, 1}, {"potion_hp_s", 0.08, 1, 2}, {"potion_mp_s", 0.05, 1, 1}}
	lootMage    = []LootEntry{{"bone_old", 0.35, 1, 2}, {"skull_cracked", 0.15, 1, 1}, {"shadow_essence", 0.12, 1, 1}, {"potion_mp_s", 0.1, 1, 2}}
	lootBoss    = []LootEntry{{"broken_crown", 1, 1, 1}, {"shadow_essence", 1, 2, 4}, {"potion_hp_l", 1, 2, 3}, {"potion_mp_l", 1, 1, 2}}
)

var Monsters = map[string]*MonsterDef{
	"skeleton_minion": {ID: "skeleton_minion", Name: "Esqueleto Lacaio", Model: "skeleton_minion",
		HP: 50, HPLvl: 18, Atk: 6, AtkLv: 1.6, Def: 8, DefLv: 2.2, Range: 2, Interval: 1.6, Speed: 3.5, XPBase: 12, Scale: 1,
		GoldChance: 0.7, GoldMin: 2, GoldMax: 4, Loot: lootMinion},
	"skeleton_warrior": {ID: "skeleton_warrior", Name: "Esqueleto Guerreiro", Model: "skeleton_warrior",
		HP: 70, HPLvl: 22, Atk: 8, AtkLv: 2.0, Def: 12, DefLv: 3, Range: 2.2, Interval: 1.5, Speed: 4, Aggressive: true, XPBase: 16, Scale: 1,
		GoldChance: 0.7, GoldMin: 2, GoldMax: 4, Loot: lootFighter},
	"skeleton_rogue": {ID: "skeleton_rogue", Name: "Esqueleto Ladino", Model: "skeleton_rogue",
		HP: 55, HPLvl: 17, Atk: 9, AtkLv: 2.2, Def: 8, DefLv: 2, Range: 2, Interval: 1.0, Speed: 4.5, Aggressive: true, XPBase: 16, Scale: 1,
		GoldChance: 0.7, GoldMin: 2, GoldMax: 4, Loot: lootFighter},
	"skeleton_mage": {ID: "skeleton_mage", Name: "Esqueleto Mago", Model: "skeleton_mage",
		HP: 45, HPLvl: 14, Atk: 10, AtkLv: 2.4, Def: 6, DefLv: 1.6, Range: 10, Interval: 2.0, Speed: 3.5, Magic: true, Aggressive: true, XPBase: 18, Scale: 1,
		GoldChance: 0.7, GoldMin: 2.5, GoldMax: 4.5, Loot: lootMage},
	"bone_lord": {ID: "bone_lord", Name: "Senhor dos Ossos", Model: "skeleton_warrior",
		HP: 400, HPLvl: 60, Atk: 14, AtkLv: 3, Def: 20, DefLv: 4, Range: 2.8, Interval: 1.8, Speed: 4, Aggressive: true, XPBase: 120, Scale: 1.6, Boss: true,
		GoldChance: 1, GoldMin: 30, GoldMax: 45, Loot: lootBoss},
}

// MonsterStats calcula os atributos do monstro num nível.
func MonsterStats(m *MonsterDef, level int) Stats {
	l := float64(clampLevel(level) - 1)
	s := Stats{
		MaxHP: m.HP + m.HPLvl*l,
		PAtk:  m.Atk + m.AtkLv*l,
		PDef:  m.Def + m.DefLv*l,
		Range: m.Range, AtkInterval: m.Interval, Speed: m.Speed,
		Crit: 0.05, Magic: m.Magic, CritDamage: CritMult, CastSpeed: 1,
		// Mesma conta de um jogador com Destreza 10 no mesmo nível.
		Accuracy: accuracyFor(10, level), Evasion: evasionFor(10, level),
	}
	s.MAtk = s.PAtk
	s.MDef = s.PDef * 0.8
	return s
}

// Spawn é um grupo de monstros espalhados num anel ao redor da cidade.
// O nível cresce com a distância: perto de RMin = LevelMin, perto de RMax = LevelMax.
type Spawn struct {
	Monster            string
	Count              int
	RMin, RMax         float64
	LevelMin, LevelMax int
	Respawn            float64 // segundos (sorteado entre Respawn e 2×Respawn)
	// Ponto fixo opcional (chefes). Se Fixed, usa X/Z e LevelMin.
	Fixed bool
	X, Z  float64
}

// SpawnPoint é a posição concreta de um monstro.
type SpawnPoint struct {
	Monster *MonsterDef
	Level   int
	X, Z    float64
	Respawn float64
}

// DefaultSpawns: 3 anéis de caça + chefe.
func DefaultSpawns() []Spawn {
	return []Spawn{
		{Monster: "skeleton_minion", Count: 30, RMin: 46, RMax: 74, LevelMin: 1, LevelMax: 5, Respawn: 20},
		{Monster: "skeleton_warrior", Count: 12, RMin: 80, RMax: 108, LevelMin: 6, LevelMax: 12, Respawn: 30},
		{Monster: "skeleton_rogue", Count: 12, RMin: 80, RMax: 108, LevelMin: 6, LevelMax: 12, Respawn: 30},
		{Monster: "skeleton_mage", Count: 10, RMin: 116, RMax: 140, LevelMin: 13, LevelMax: 20, Respawn: 40},
		{Monster: "skeleton_warrior", Count: 10, RMin: 116, RMax: 140, LevelMin: 13, LevelMax: 20, Respawn: 40},
		{Monster: "bone_lord", Count: 1, Fixed: true, X: 0, Z: -132, LevelMin: 22, Respawn: 300},
	}
}

// ExpandSpawns sorteia as posições (determinístico para a mesma semente).
func ExpandSpawns(spawns []Spawn, rng *rand.Rand) []SpawnPoint {
	var out []SpawnPoint
	for _, s := range spawns {
		def := Monsters[s.Monster]
		if def == nil {
			continue
		}
		for i := 0; i < s.Count; i++ {
			if s.Fixed {
				out = append(out, SpawnPoint{Monster: def, Level: s.LevelMin, X: s.X, Z: s.Z, Respawn: s.Respawn})
				continue
			}
			// Raio uniforme em área (sqrt) para não amontoar perto do centro do anel.
			u := rng.Float64()
			r := math.Sqrt(s.RMin*s.RMin + u*(s.RMax*s.RMax-s.RMin*s.RMin))
			a := rng.Float64() * 2 * math.Pi
			t := (r - s.RMin) / math.Max(1e-9, s.RMax-s.RMin)
			level := s.LevelMin + int(math.Round(t*float64(s.LevelMax-s.LevelMin)))
			out = append(out, SpawnPoint{Monster: def, Level: level, X: r * math.Cos(a), Z: r * math.Sin(a), Respawn: s.Respawn})
		}
	}
	return out
}
