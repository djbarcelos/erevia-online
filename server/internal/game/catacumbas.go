package game

import (
	"math"
	"math/rand/v2"
	"slices"
)

// Masmorra 1: Catacumbas de Vel'Aran (tunnel#140, roadmap seção 8, fecha a #80).
//
// Os monstros e o chefe moram aqui (arquivo próprio: o mundo aberto não os conhece). O
// layout (salas, escadas e onde cada monstro nasce) sai do gerador de masmorras do cliente
// (client/scripts/dungeons/catacumbas.mjs, em maps/catacumbas.json) e a instância em si é
// de zone/instance.go.
const (
	// CatacombsID é o id da masmorra (e do mapa).
	CatacombsID = "catacumbas"
	// Faixa da masmorra: o andar 1 é do 12 ao 14, o andar 2 e o chefe, do 14 ao 16.
	CatacombsMinLevel, CatacombsMaxLevel = 12, 16
	// CatacombsPartyMax: o grupo máximo (projetada para 3).
	CatacombsPartyMax = 6
	// CatacombsMobCap: o teto de monstros por cópia (roadmap seção 8: no máximo 40).
	CatacombsMobCap = 40
	// DungeonNoRespawn: segundos até um monstro da masmorra renascer. Passa da vida da
	// cópia (45 min), então na prática não renascem: é uma luta só.
	DungeonNoRespawn = 3600.0
)

// Ids dos monstros das Catacumbas.
const (
	CatacombMinion = "cata_minion"
	CatacombGuard  = "cata_guard"
	CatacombMage   = "cata_acolyte"
	CatacombRogue  = "cata_lurker"
	AbadeUlric     = "abade_ulric"
)

// Saque do Abade Ulric (o comum, que cai no chão; o baú é por jogador, em RollDungeonChest).
var lootUlric = []LootEntry{
	{"shadow_essence", 1, 2, 3}, {"skull_cracked", 1, 2, 3}, {"potion_hp_l", 1, 1, 2},
	{"scroll_ench_weapon", 0.4, 1, 1}, {"scroll_ench_armor", 0.5, 1, 2},
	{"scroll_ench_weapon_b", 0.06, 1, 1}, {"scroll_ench_armor_b", 0.08, 1, 1},
}

// catacombMonsters: os esqueletos do remaster com papel (lacaio em matilha, guerreiro de escudo,
// mago atirador e ladino emboscador) e o Abade. Todos extras do bestiário, como os outros
// de papel, e fora da conta do simulador de equilíbrio.
var catacombMonsters = map[string]*MonsterDef{
	CatacombMinion: {ID: CatacombMinion, Name: "Ossada do Ossuário", Model: "skeleton_minion", Tint: "#a9a4b8",
		HP: 54, HPLvl: 18, Atk: 7, AtkLv: 1.7, Def: 9, DefLv: 2.3, Range: 2, Interval: 1.6, Speed: 4.0, Aggressive: true, XPBase: 14, Scale: 0.95,
		Archetype: ArchPack, BestiaryExtra: true, GoldChance: 0.7, GoldMin: 2.5, GoldMax: 4.5, Loot: lootMinion},
	CatacombGuard: {ID: CatacombGuard, Name: "Guarda do Ossuário", Model: "skeleton_warrior", Tint: "#8d95b3",
		HP: 80, HPLvl: 24, Atk: 8.5, AtkLv: 2.1, Def: 14, DefLv: 3.2, Range: 2.2, Interval: 1.7, Speed: 3.7, Aggressive: true, XPBase: 20, Scale: 1.1,
		Archetype: ArchShield, BestiaryExtra: true, GoldChance: 0.7, GoldMin: 3, GoldMax: 5, Loot: lootFighter},
	CatacombMage: {ID: CatacombMage, Name: "Acólito Sepulto", Model: "skeleton_mage", Tint: "#a07fd0",
		HP: 48, HPLvl: 15, Atk: 10, AtkLv: 2.4, Def: 7, DefLv: 1.7, Range: 10, Interval: 2.0, Speed: 3.6, Magic: true, Aggressive: true, XPBase: 21, Scale: 1.05,
		Archetype: ArchArcher, BestiaryExtra: true, GoldChance: 0.7, GoldMin: 3, GoldMax: 5, Loot: lootMage},
	CatacombRogue: {ID: CatacombRogue, Name: "Coveiro Espreitador", Model: "skeleton_rogue", Tint: "#6a7088",
		HP: 58, HPLvl: 18, Atk: 9.5, AtkLv: 2.3, Def: 9, DefLv: 2.1, Range: 2, Interval: 1.2, Speed: 4.6, Aggressive: true, XPBase: 20, Scale: 1,
		Archetype: ArchAmbusher, BestiaryExtra: true, GoldChance: 0.7, GoldMin: 3, GoldMax: 5, Loot: lootFighter},

	// Abade Ulric (nível 16): o monge que enterrou os primeiros mortos da Partição. É o
	// esqueleto mago em escala 2, num tom roxo-acinzentado. Chefe de campo dentro da
	// instância: nasce com a cópia e não renasce. Ensina os 3 avisos do raid antes do raid:
	//   F1 Responso: anel de 3 a 9 m em volta dele (o meio e a borda são seguros).
	//   F2 (60%) Ossuário: chama 4 ossadas e sorteia 2 círculos de 3 m em quem luta.
	//   F3 (25%) Vela Final: marca que segue um jogador e explode em quem está perto dele.
	AbadeUlric: {ID: AbadeUlric, Name: "Abade Ulric", Model: "skeleton_mage", Tint: "#b9a4d8",
		HP: 700, HPLvl: 230, Atk: 14, AtkLv: 3.0, Def: 20, DefLv: 4, Range: 3, Interval: 2.0, Speed: 3.8, Magic: true,
		Aggressive: true, XPBase: 150, Scale: 2.0, BodyRadius: 1.0, Boss: true, LeashRadius: 26,
		GoldChance: 1, GoldMin: 25, GoldMax: 38, Loot: lootUlric, BestiaryExtra: true,
		FieldBoss: &FieldBossDef{Instance: true, Place: "nas Catacumbas de Vel'Aran", Phases: []FieldPhase{
			{AtHP: 0.60, Say: "Abade Ulric: \"Os mortos que enterrei se levantam para me servir.\"", Summon: CatacombMinion, SummonCount: 4},
			{AtHP: 0.25, Say: "Abade Ulric: \"Apaguem a última vela!\"", Enrage: true},
		}},
		Abilities: []AbilityDef{
			{ID: "ulric_responso", Look: LookStrike, Shape: ShapeDonut, Target: AimSelf, Inner: 3, Radius: 9,
				Delay: 2.0, Cooldown: 11, First: 5, Mult: 1.6, MinPhase: 1},
			{ID: "ulric_ossuario", Look: LookStrike, Shape: ShapeCircle, Target: AimRandom, Count: 2, Radius: 3,
				Delay: 1.8, Cooldown: 13, First: 4, Mult: 1.3, MinPhase: 2,
				Effect: &EffectDef{Kind: EffectSlow, Duration: 3, Value: 0.3}},
			{ID: "ulric_vela", Look: LookMark, Shape: ShapeCircle, Target: AimMark, Radius: 4,
				Delay: 4.5, Cooldown: 18, First: 4, Mult: 1.5, MinPhase: 3,
				OnMarked: &EffectDef{Kind: EffectDeathMark, Duration: 4.5},
				Warn:     "Você foi marcado pela Vela Final! Afaste-se do seu grupo antes que ela se apague."},
		}},
}

// catacombsRegistered registra os monstros no catálogo. É uma variável (e não um init) para
// rodar antes de todos os init do pacote, que preenchem e validam os papéis e as habilidades
// de tudo o que está em Monsters.
var catacombsRegistered = registerCatacombs()

func registerCatacombs() bool {
	for id, m := range catacombMonsters {
		Monsters[id] = m
	}
	return true
}

// CatacombsMonsterIDs: os ids de monstro da masmorra, em ordem.
func CatacombsMonsterIDs() []string {
	ids := make([]string, 0, len(catacombMonsters))
	for id := range catacombMonsters {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// ---------- Baú do dia ----------

// DungeonChestDef é o baú de uma masmorra: o que cada personagem abre uma vez por dia ao
// matar o chefe. O saque é do personagem (cada um sorteia o seu), não do chão.
type DungeonChestDef struct {
	Dungeon string
	Name    string
	Gold    float64     // moedas por nível do personagem
	Loot    []LootEntry // fixos e sorteados
	// Equipamento: uma peça do grau, que o personagem pode usar (da sua classe). A chance de
	// cada raridade; o resto do sorteio é comum.
	Tier                   Tier
	EpicChance, RareChance float64
}

// DungeonChests: o baú de cada masmorra, por id.
var DungeonChests = map[string]*DungeonChestDef{
	CatacombsID: {
		Dungeon: CatacombsID, Name: "Baú do Abade", Gold: 40,
		Loot: []LootEntry{
			{"potion_hp_l", 1, 2, 3}, {"potion_mp_l", 1, 1, 2}, {"shadow_essence", 1, 2, 4},
			{"scroll_ench_armor", 0.7, 1, 2}, {"scroll_ench_weapon", 0.45, 1, 1},
			{"scroll_ench_armor_b", 0.12, 1, 1}, {"scroll_ench_weapon_b", 0.08, 1, 1},
		},
		Tier: TierD, EpicChance: 0.05, RareChance: 0.30,
	},
}

// RollDungeonChest sorteia o baú de um personagem (classe e nível dele): moedas, o saque
// da lista e uma peça de equipamento do grau que a classe usa.
func RollDungeonChest(def *DungeonChestDef, class string, level int, rng *rand.Rand) []Drop {
	var out []Drop
	if n := int(math.Round(def.Gold * float64(level))); n > 0 {
		out = append(out, Drop{Item: Items[GoldID], Count: n})
	}
	for _, e := range def.Loot {
		if rng.Float64() >= e.Chance {
			continue
		}
		n := e.Min
		if e.Max > e.Min {
			n += rng.IntN(e.Max - e.Min + 1)
		}
		if d := Items[e.ItemID]; d != nil && n > 0 {
			out = append(out, Drop{Item: d, Count: n})
		}
	}
	// Uma peça: épica, rara ou comum, a primeira raridade que tiver peça para a classe.
	roll := rng.Float64()
	grades := []Grade{GradeCommon}
	switch {
	case roll < def.EpicChance:
		grades = []Grade{GradeEpic, GradeRare, GradeCommon}
	case roll < def.EpicChance+def.RareChance:
		grades = []Grade{GradeRare, GradeCommon}
	}
	for _, g := range grades {
		var pool []*ItemDef
		for _, d := range TierPieces(def.Tier, g) {
			if d.UsableBy(class) {
				pool = append(pool, d)
			}
		}
		if len(pool) > 0 {
			out = append(out, Drop{Item: pool[rng.IntN(len(pool))], Count: 1})
			break
		}
	}
	return out
}

// NormalChests limpa o que veio do banco: só masmorras que existem e dias positivos.
func NormalChests(in map[string]int) map[string]int {
	var out map[string]int
	for id, day := range in {
		if DungeonChests[id] == nil || day <= 0 {
			continue
		}
		if out == nil {
			out = map[string]int{}
		}
		out[id] = day
	}
	return out
}
