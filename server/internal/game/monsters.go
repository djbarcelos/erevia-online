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
	// BodyRadius: raio do corpo (m). Golpes e skills contam da borda do corpo, não do
	// centro: em monstros grandes o atacante para encostado, não dentro das pedras.
	// 0 = corpo de tamanho normal (ninguém mudou de alcance).
	BodyRadius float64
	Boss       bool // chefe (marcado no mapa)
	Elite      bool // elite: mais forte que o comum (o aviso das habilidades dura mais)
	// Habilidades com aviso no chão (abilities.go e zone/abilities.go).
	Abilities []AbilityDef
	// Raid boss: covil com coleira própria, fases e habilidades (zone/raid.go), recompensa
	// dividida por dano e renascimento longo salvo no banco.
	Raid        bool
	Immune      []EffectKind // efeitos que não pegam (raid: atordoar, enraizar…)
	LeashRadius float64      // distância máxima do ponto de origem (0 = a padrão da zona)
	// Renascimento do raid em segundos de relógio, sorteado entre os dois.
	RespawnMin, RespawnMax float64
	NoGear                 bool     // nunca deixa cair equipamento (invocações)
	Rare                   *RareDef // monstro raro (rares.go): nome dourado e saque garantido
	// Drops: moedas (por nível do monstro) e itens.
	GoldChance       float64
	GoldMin, GoldMax float64
	Loot             []LootEntry
}

var (
	lootMinion  = []LootEntry{{"bone_old", 0.4, 1, 2}, {"skull_cracked", 0.08, 1, 1}, {"potion_hp_s", 0.06, 1, 1}}
	lootFighter = []LootEntry{{"bone_old", 0.35, 1, 3}, {"skull_cracked", 0.15, 1, 1}, {"potion_hp_s", 0.08, 1, 2}, {"potion_mp_s", 0.05, 1, 1},
		{"scroll_ench_armor", 0.012, 1, 1}, {"scroll_ench_weapon", 0.006, 1, 1}}
	lootMage = []LootEntry{{"bone_old", 0.35, 1, 2}, {"skull_cracked", 0.15, 1, 1}, {"shadow_essence", 0.12, 1, 1}, {"potion_mp_s", 0.1, 1, 2},
		{"scroll_ench_armor", 0.015, 1, 1}, {"scroll_ench_weapon", 0.008, 1, 1},
		{"scroll_ench_armor_b", 0.003, 1, 1}, {"scroll_ench_weapon_b", 0.002, 1, 1}}
	lootBoss = []LootEntry{{"broken_crown", 1, 1, 1}, {"shadow_essence", 1, 2, 4}, {"potion_hp_l", 1, 2, 3}, {"potion_mp_l", 1, 1, 2},
		{"scroll_ench_weapon", 0.5, 1, 1}, {"scroll_ench_armor", 0.6, 1, 2},
		{"scroll_ench_weapon_b", 0.15, 1, 1}, {"scroll_ench_armor_b", 0.2, 1, 1}}
	// Raid: tudo garantido, menos os pergaminhos (os equipamentos vêm de gearDrops).
	lootRaid = []LootEntry{{"raid_fragment", 1, 1, 1}, {"shadow_essence", 1, 5, 8}, {"potion_hp_l", 1, 4, 6}, {"potion_mp_l", 1, 3, 4},
		{"scroll_ench_weapon", 0.8, 1, 2}, {"scroll_ench_armor", 1, 2, 3},
		{"scroll_ench_weapon_b", 0.35, 1, 1}, {"scroll_ench_armor_b", 0.45, 1, 1}}
	lootGolem = []LootEntry{{"rune_core", 0.3, 1, 1}, {"potion_hp_s", 0.12, 1, 2}, {"potion_hp_l", 0.03, 1, 1},
		{"scroll_ench_armor", 0.02, 1, 1}, {"scroll_ench_weapon", 0.01, 1, 1},
		{"scroll_ench_armor_b", 0.004, 1, 1}, {"scroll_ench_weapon_b", 0.002, 1, 1}}
)

// Marca da Morte do Ozrhal (os testes do raid usam os números).
const RaidMarkRadius, RaidMarkDelay, RaidMarkMult = 5.0, 5.0, 1.4

// Habilidades do Ozrhal, por fase. A ordem é a de checagem a cada think (Onda, Chuva,
// Marca), e os ids ligam cada uma ao relógio dela no raidState (zone/raid.go).
var raidAbilities = []AbilityDef{
	{ID: "wave", Look: LookWave, Shape: ShapeCircle, Target: AimSelf, Radius: 8, Delay: 2.5, Cooldown: 14, First: 8,
		Mult: 1.6, Effect: &EffectDef{Kind: EffectSlow, Duration: 3, Value: 0.4}, MinPhase: 1, HitID: HitRaidWave},
	{ID: "rain", Look: LookRain, Shape: ShapeCircle, Target: AimRandom, Count: 3, Radius: 4, Delay: 2, Cooldown: 12, First: 3,
		Mult: 1, Effect: &EffectDef{Kind: EffectStun, Duration: 1}, MinPhase: 2, HitID: HitRaidRain},
	{ID: "mark", Look: LookMark, Shape: ShapeCircle, Target: AimMark, Radius: RaidMarkRadius, Delay: RaidMarkDelay, Cooldown: 16, First: 4,
		Mult: RaidMarkMult, OnMarked: &EffectDef{Kind: EffectDeathMark, Duration: RaidMarkDelay}, MinPhase: 3, HitID: HitRaidMark,
		Warn: "Você foi marcado pela morte! Afaste-se do seu grupo antes que a marca exploda."},
}

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
	// Lento e muito resistente; bate forte e devagar.
	"stone_golem": {ID: "stone_golem", Name: "Golem de Pedra", Model: "stone_golem",
		HP: 180, HPLvl: 40, Atk: 14, AtkLv: 2.6, Def: 26, DefLv: 4, Range: 2.6, Interval: 2.4, Speed: 2.8, Aggressive: true, XPBase: 40, Scale: 1.25, BodyRadius: 1.0,
		GoldChance: 0.8, GoldMin: 3, GoldMax: 6, Loot: lootGolem,
		// Pancada em cone à frente: ~25% do dano da luta (4 golpes de 2,4 s + 1,5 a cada 10 s).
		Abilities: []AbilityDef{{ID: "golem_slam", Shape: ShapeCone, Target: AimSelf, Radius: 5, Arc: 90,
			Delay: 1.2, Cooldown: 10, First: 4, Mult: 1.5, Effect: &EffectDef{Kind: EffectSlow, Duration: 2, Value: 0.3}}}},
	"bone_lord": {ID: "bone_lord", Name: "Senhor dos Ossos", Model: "skeleton_warrior",
		HP: 400, HPLvl: 60, Atk: 14, AtkLv: 3, Def: 20, DefLv: 4, Range: 2.8, Interval: 1.8, Speed: 4, Aggressive: true, XPBase: 120, Scale: 1.6, Boss: true,
		GoldChance: 1, GoldMin: 30, GoldMax: 45, Loot: lootBoss,
		// Anel de ossos: quem está colado nele escapa; quem bate de longe precisa entrar ou
		// sair. ~25% do dano da luta (6,7 golpes de 1,8 s + 2,2 a cada 12 s).
		Abilities: []AbilityDef{{ID: "bone_ring", Shape: ShapeDonut, Target: AimSelf, Inner: 3.5, Radius: 9,
			Delay: 2, Cooldown: 12, First: 5, Mult: 2.2}}},
	// Raid boss da Cripta Esquecida: um lich gigante. Imune a controle (menos provocar),
	// preso ao covil e com habilidades por fase (zone/raid.go).
	"ozrhal": {ID: "ozrhal", Name: "Ozrhal, o Rei Ossário", Model: "skeleton_mage",
		HP: 6000, HPLvl: 2700, Atk: 40, AtkLv: 4, Def: 40, DefLv: 5, Range: 3, Interval: 2.0, Speed: 4.2, Magic: true,
		Aggressive: true, XPBase: 260, Scale: 2.6, BodyRadius: 1.2, Boss: true, Raid: true,
		Immune:      []EffectKind{EffectStun, EffectRoot, EffectSlow, EffectBlind},
		LeashRadius: 30, RespawnMin: 3 * 3600, RespawnMax: 4 * 3600,
		GoldChance: 1, GoldMin: 120, GoldMax: 160, Loot: lootRaid,
		Abilities: raidAbilities},
	// Invocação do Ozrhal: não renasce e não deixa equipamento.
	"bone_guard": {ID: "bone_guard", Name: "Guarda Ossário", Model: "skeleton_warrior",
		HP: 120, HPLvl: 30, Atk: 12, AtkLv: 2.4, Def: 14, DefLv: 3, Range: 2.2, Interval: 1.6, Speed: 4.4, Aggressive: true, XPBase: 6, Scale: 1.15,
		NoGear: true},
}

// ImmuneTo informa se o efeito não pega no monstro.
func (m *MonsterDef) ImmuneTo(k EffectKind) bool {
	for _, i := range m.Immune {
		if i == k {
			return true
		}
	}
	return false
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
	// Área: espalha no disco de centro X/Z e raio Radius (bioma próprio, fora dos anéis).
	// O nível sobe com a profundidade: LevelMin na borda norte, LevelMax na borda sul.
	Area   bool
	Radius float64
	// Região: o nome que aparece no mapa. Um disco (Area) ou setor com Region tem dono: os
	// anéis das outras regiões não sorteiam monstros lá dentro (ver Owned).
	Region string
	// Setor do anel, em graus: o anel só sorteia entre AngleMin e AngleMax. 0° = leste (+X)
	// e o ângulo cresce para o norte (+Z), o sentido de atan2(z, x) e anti-horário no mapa:
	// 90° = norte, 180° = oeste, 270° = sul. Com AngleMin > AngleMax o setor cruza o 0°
	// (340 → 20 é o leste, 40° de largura). Os dois iguais (0 e 0) = o anel inteiro.
	AngleMin, AngleMax float64
	// When: período em que o spawn existe. Na virada, os do outro período saem quando
	// estão fora de combate e os deste entram no lugar (zone/spawnperiod.go).
	When When
}

// When é o período em que um spawn existe.
type When uint8

const (
	WhenAny   When = iota // sempre (o padrão)
	WhenDay               // só de dia (some à noite)
	WhenNight             // só à noite
)

// Active informa se o spawn existe no período (night = game.IsNight).
func (w When) Active(night bool) bool {
	return w == WhenAny || (w == WhenNight) == night
}

func (w When) String() string {
	switch w {
	case WhenDay:
		return "de dia"
	case WhenNight:
		return "à noite"
	}
	return ""
}

// Sector informa se o spawn é um setor de anel (e não o anel inteiro).
func (s Spawn) Sector() bool {
	return !s.Fixed && !s.Area && s.AngleMin != s.AngleMax
}

// SectorSpan devolve o começo do setor em [0, 360) e a largura em (0, 360]: o setor vai
// de start a start+span no sentido anti-horário (passando do 360 quando cruza o 0°).
func (s Spawn) SectorSpan() (start, span float64) {
	start = wrapDeg(s.AngleMin)
	span = wrapDeg(s.AngleMax - s.AngleMin)
	if span == 0 {
		span = 360 // 0 → 360: o anel inteiro, mas declarado como setor
	}
	return start, span
}

// Owned informa se o spawn declara uma área com dono (disco ou setor com Region).
func (s Spawn) Owned() bool {
	return s.Region != "" && (s.Area || s.Sector())
}

// Contains informa se (x, z) está no disco ou setor do spawn, com uma folga em metros.
func (s Spawn) Contains(x, z, margin float64) bool {
	switch {
	case s.Area:
		return math.Hypot(x-s.X, z-s.Z) <= s.Radius+margin
	case s.Sector():
		r := math.Hypot(x, z)
		if r < s.RMin-margin || r > s.RMax+margin {
			return false
		}
		start, span := s.SectorSpan()
		pad := margin / math.Max(r, 1) * 180 / math.Pi // a folga em graus naquele raio
		return wrapDeg(AngleDeg(x, z)-start+pad) <= span+2*pad
	}
	return false
}

// AngleDeg é o ângulo de (x, z) em graus, em [0, 360), na convenção de Spawn.AngleMin.
func AngleDeg(x, z float64) float64 {
	return wrapDeg(math.Atan2(z, x) * 180 / math.Pi)
}

func wrapDeg(a float64) float64 {
	a = math.Mod(a, 360)
	if a < 0 {
		a += 360
	}
	if a >= 360 { // -1e-15 + 360 arredonda para 360
		a = 0
	}
	return a
}

// Covil do raid boss (a Cripta Esquecida, no nordeste do mapa).
const (
	RaidLairX      = 108.0
	RaidLairZ      = 108.0
	RaidLairRadius = 24.0
)

// SpawnPoint é a posição concreta de um monstro.
type SpawnPoint struct {
	Monster *MonsterDef
	Level   int
	X, Z    float64
	Respawn float64
	When    When   // período do spawn de origem
	Region  string // região do spawn de origem (a troca dia/noite é por região)
}

// ownedMargin: folga (m) em volta das áreas com dono e do covil em que os anéis não sorteiam.
const ownedMargin = 6.0

// DefaultSpawns: 3 anéis de caça + chefe + o Vale das Pedras Vivas (golens, 20–30), no sul.
//
// O id de cada monstro é a sua posição na lista expandida: spawn novo vai sempre para o
// fim, e mudar Count de um spawn do meio muda o id de todos os que vêm depois. Para tirar
// monstros de um pedaço do anel, declare a área (setor ou disco com Region): o anel sorteia
// fora dela sem mudar a contagem. Numa região, os spawns WhenDay e WhenNight devem somar a
// mesma quantidade (os noturnos trocam os do dia, não somam).
func DefaultSpawns() []Spawn {
	return []Spawn{
		{Monster: "skeleton_minion", Count: 30, RMin: 46, RMax: 74, LevelMin: 1, LevelMax: 5, Respawn: 20},
		{Monster: "skeleton_warrior", Count: 12, RMin: 80, RMax: 108, LevelMin: 6, LevelMax: 12, Respawn: 30},
		{Monster: "skeleton_rogue", Count: 12, RMin: 80, RMax: 108, LevelMin: 6, LevelMax: 12, Respawn: 30},
		{Monster: "skeleton_mage", Count: 10, RMin: 116, RMax: 140, LevelMin: 13, LevelMax: 20, Respawn: 40},
		{Monster: "skeleton_warrior", Count: 10, RMin: 116, RMax: 140, LevelMin: 13, LevelMax: 20, Respawn: 40},
		{Monster: "bone_lord", Count: 1, Fixed: true, X: 0, Z: -132, LevelMin: 22, Respawn: 300},
		// No fim da lista para não mudar os ids dos monstros que já existiam.
		// Níveis 20 (entrada, ao norte) a 30 (fundo, ao sul): a caça do 25 ao 30 é a metade
		// funda do vale. Mesma quantidade de golens (o mesmo pico de malhas no cliente), com
		// renascimento um pouco mais rápido para caber mais gente.
		{Monster: "stone_golem", Count: 12, Area: true, X: 0, Z: -270, Radius: 52, LevelMin: 20, LevelMax: 30, Respawn: 45,
			Region: "Vale das Pedras Vivas"},
		// Raid boss no covil da Cripta Esquecida (o renascimento vem da def, em horas).
		{Monster: "ozrhal", Count: 1, Fixed: true, X: RaidLairX, Z: RaidLairZ, LevelMin: 25},
	}
}

// ExpandSpawns sorteia as posições (determinístico para a mesma semente).
func ExpandSpawns(spawns []Spawn, rng *rand.Rand) []SpawnPoint {
	var out []SpawnPoint
	var owned []Spawn // discos e setores com dono: os anéis das outras regiões ficam fora
	for _, s := range spawns {
		if s.Owned() {
			owned = append(owned, s)
		}
	}
	// blocked: o ponto do anel s cai no covil ou na área de outra região.
	blocked := func(s Spawn, x, z float64) bool {
		if math.Hypot(x-RaidLairX, z-RaidLairZ) <= RaidLairRadius+ownedMargin {
			return true
		}
		for _, o := range owned {
			if o.Region != s.Region && o.Contains(x, z, ownedMargin) {
				return true
			}
		}
		return false
	}
	for _, s := range spawns {
		def := Monsters[s.Monster]
		if def == nil {
			continue
		}
		start, span := s.SectorSpan()
		for i := 0; i < s.Count; i++ {
			if s.Fixed {
				out = append(out, SpawnPoint{Monster: def, Level: s.LevelMin, X: s.X, Z: s.Z, Respawn: s.Respawn,
					When: s.When, Region: s.Region})
				continue
			}
			if s.Area {
				// Uniforme no disco (sqrt no raio para não amontoar no centro).
				r := s.Radius * math.Sqrt(rng.Float64())
				a := rng.Float64() * 2 * math.Pi
				x, z := s.X+r*math.Cos(a), s.Z+r*math.Sin(a)
				depth := (s.Z + s.Radius - z) / math.Max(1e-9, 2*s.Radius) // 0 na borda norte, 1 na sul
				level := s.LevelMin + int(math.Round(depth*float64(s.LevelMax-s.LevelMin)))
				out = append(out, SpawnPoint{Monster: def, Level: level, X: x, Z: z, Respawn: s.Respawn,
					When: s.When, Region: s.Region})
				continue
			}
			// Raio uniforme em área (sqrt) para não amontoar perto do centro do anel; o ângulo
			// só dentro do setor, se houver. Fora do covil do raid boss e das áreas com dono de
			// outras regiões (sorteia de novo; só quem cairia lá dentro muda de lugar).
			var r, a float64
			for range 64 {
				u := rng.Float64()
				r = math.Sqrt(s.RMin*s.RMin + u*(s.RMax*s.RMax-s.RMin*s.RMin))
				if s.Sector() {
					a = (start + rng.Float64()*span) * math.Pi / 180
				} else {
					a = rng.Float64() * 2 * math.Pi // a mesma conta de antes: as posições não mudam
				}
				if !blocked(s, r*math.Cos(a), r*math.Sin(a)) {
					break
				}
			}
			t := (r - s.RMin) / math.Max(1e-9, s.RMax-s.RMin)
			level := s.LevelMin + int(math.Round(t*float64(s.LevelMax-s.LevelMin)))
			out = append(out, SpawnPoint{Monster: def, Level: level, X: r * math.Cos(a), Z: r * math.Sin(a), Respawn: s.Respawn,
				When: s.When, Region: s.Region})
		}
	}
	return out
}
