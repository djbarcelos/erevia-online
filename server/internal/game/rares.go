package game

import (
	"math"
	"slices"
	"strings"
	"time"
)

// Monstros raros ("named", como no Lineage 2; roadmap 5.9 e T6). Um raro nasce no lugar
// de um monstro comum (o hospedeiro) quando ele renasce, com uma chance pequena, no máximo
// uma vez a cada 45 min (relógio de parede, salvo no banco) e só onde e quando a condição
// deixa. É o hospedeiro mais forte (nível +2, vida ×4, 1,2× maior), com o nome dourado, um
// saque garantido e, às vezes, uma mecânica própria. A simulação fica em zone/rares.go.

const (
	RareCooldown   = 45 * time.Minute // mínimo entre duas aparições do mesmo raro
	RareLevelBonus = 2                // níveis acima do hospedeiro
	RareHPMult     = 4.0
	RareScale      = 1.2
	RareXPMult     = 4.0 // a XP acompanha a vida (é 4× mais luta)
	RareGoldMult   = 2.0
	RareRumorRange = 60.0 // quem está a esta distância (m) lê o aviso de que ele nasceu
)

// RareArea é a parte de uma área em que o raro pode nascer (vazia = em qualquer lugar).
type RareArea struct {
	// Anel (RMax > 0) em volta do centro do mapa; com AngleMin ≠ AngleMax, só o setor (na
	// convenção de Spawn.AngleMin: 0° = leste, 90° = norte).
	RMin, RMax, AngleMin, AngleMax float64
	// Disco (Radius > 0) de centro X/Z. South: só a metade sul (z abaixo do centro).
	X, Z, Radius float64
	South        bool
}

// Contains informa se (x, z) está na área.
func (a RareArea) Contains(x, z float64) bool {
	switch {
	case a.Radius > 0:
		return math.Hypot(x-a.X, z-a.Z) <= a.Radius && (!a.South || z <= a.Z)
	case a.RMax > 0:
		if a.AngleMin != a.AngleMax {
			return Spawn{RMin: a.RMin, RMax: a.RMax, AngleMin: a.AngleMin, AngleMax: a.AngleMax}.Contains(x, z, 0)
		}
		r := math.Hypot(x, z)
		return r >= a.RMin && r <= a.RMax
	}
	return true
}

// RareDef é um monstro raro.
type RareDef struct {
	ID     string  // id do MonsterDef do raro (em Monsters, montado a partir do hospedeiro)
	Name   string  // nome dourado no cliente
	Host   string  // monstro comum em cujo renascimento ele pode nascer
	Chance float64 // por renascimento do hospedeiro (0,02–0,04)
	// Level: nível mínimo (o da tabela 5.9). O raro tem o nível do hospedeiro +2, nunca
	// abaixo deste.
	Level int
	When  When     // período (só à noite, só de dia ou sempre)
	Where RareArea // onde o hospedeiro precisa estar (a casa dele)
	Rumor string   // aviso no chat para quem está a até RareRumorRange
	Loot  []LootEntry
	// Mecânica.
	Abilities []AbilityDef // com aviso no chão (zone/abilities.go), além das do hospedeiro
	Rise      float64      // levanta uma vez ao morrer, com esta fração da vida (0 = não)
	Guards    int          // "Formação!": chama Guards escudeiros (RareGuardID) ao chegar a GuardsAt
	GuardsAt  float64
	Shout     string // o que ele grita ao chamar a guarda ou levantar
}

// Escudeiros do Capitão Ossudo: chegam com um escudo que segura RareGuardShield × a vida.
const (
	RareGuardID         = "captain_guard"
	RareGuardShield     = 0.5
	RareGuardShieldTime = 20.0 // segundos
)

// Rares: os 4 primeiros (tabela 5.9 do roadmap), nas regiões que já existem.
var Rares = []*RareDef{
	{ID: "rare_ossudo", Name: "Ossudo, o Teimoso", Host: "skeleton_minion", Chance: 0.03, Level: 6,
		Where: RareArea{RMin: 46, RMax: 74},
		Rumor: "Algo grande se move no bosque… ossos que não querem ficar no chão.",
		Loot:  []LootEntry{{"skull_cracked", 1, 3, 3}},
		Rise:  0.3, Shout: "Ossudo, o Teimoso, se recompõe: \"Ainda não!\""},
	{ID: "rare_captain", Name: "Capitão Ossudo", Host: "skeleton_warrior", Chance: 0.02, Level: 14, When: WhenNight,
		Where:  RareArea{RMin: 80, RMax: 108}, // os Guerreiros dos Campos (os dos Ermos ficam fora)
		Rumor:  "Algo grande se move nos Campos dos Ossos… passos em marcha na escuridão.",
		Loot:   []LootEntry{{"captain_insignia", 1, 1, 1}, {"helm_rare", 0.2, 1, 1}},
		Guards: 2, GuardsAt: 0.5, Shout: "Capitão Ossudo: \"Formação!\""},
	{ID: "rare_vossk", Name: "Vossk, o Cinzento", Host: "skeleton_mage", Chance: 0.02, Level: 21, When: WhenNight,
		Where: RareArea{RMin: 116, RMax: 140, AngleMin: 135, AngleMax: 225}, // o oeste dos Ermos
		Rumor: "Algo grande se move nos Ermos… cinzas giram no ar sem vento.",
		Loot:  []LootEntry{{"shadow_essence", 1, 2, 2}},
		// Chuva de Ossos mirim: 2 círculos de 3 m em quem está na luta, 1,5 s de aviso.
		Abilities: []AbilityDef{{ID: "vossk_rain", Look: LookRain, Shape: ShapeCircle, Target: AimRandom, Count: 2, Radius: 3,
			Delay: 1.5, Cooldown: 9, First: 4, Mult: 1.3}}},
	{ID: "rare_omrun", Name: "Ômrun, Golem Ancião", Host: "stone_golem", Chance: 0.02, Level: 31,
		Where: RareArea{X: 0, Z: -270, Radius: 52, South: true}, // a metade sul (funda) do Vale
		Rumor: "Algo grande se move no Vale… o chão treme sob as pedras.",
		Loot:  []LootEntry{{"rune_core", 1, 2, 2}},
		// Pisão: anel de 3 a 7 m em volta dele, 2,0 s de aviso (colado nele, escapa).
		Abilities: []AbilityDef{{ID: "omrun_stomp", Shape: ShapeDonut, Target: AimSelf, Inner: 3, Radius: 7,
			Delay: 2, Cooldown: 12, First: 6, Mult: 1.8, Effect: &EffectDef{Kind: EffectSlow, Duration: 3, Value: 0.4}}}},
}

// RaresByHost: os raros de cada hospedeiro.
var RaresByHost = func() map[string][]*RareDef {
	m := map[string][]*RareDef{}
	for _, r := range Rares {
		m[r.Host] = append(m[r.Host], r)
	}
	return m
}()

// RareIDs: os ids dos raros, separados por "|" (alvo da conquista Caçador de Raros).
func RareIDs() string {
	ids := make([]string, len(Rares))
	for i, r := range Rares {
		ids[i] = r.ID
	}
	return strings.Join(ids, "|")
}

// RareLevel: o nível do raro que nasce no lugar de um hospedeiro de nível hostLevel.
func (r *RareDef) RareLevel(hostLevel int) int {
	return clampLevel(max(hostLevel+RareLevelBonus, r.Level))
}

// Monster: o MonsterDef do raro.
func (r *RareDef) Monster() *MonsterDef { return Monsters[r.ID] }

// rareMonster monta o MonsterDef do raro a partir do hospedeiro.
func rareMonster(r *RareDef, host *MonsterDef) *MonsterDef {
	d := *host
	d.ID, d.Name, d.Rare = r.ID, r.Name, r
	d.HP, d.HPLvl = host.HP*RareHPMult, host.HPLvl*RareHPMult
	scale := host.Scale
	if scale <= 0 {
		scale = 1
	}
	d.Scale = scale * RareScale
	d.BodyRadius = host.BodyRadius * RareScale
	d.XPBase = host.XPBase * RareXPMult
	d.GoldChance, d.GoldMin, d.GoldMax = 1, host.GoldMin*RareGoldMult, host.GoldMax*RareGoldMult
	d.Loot = append(slices.Clone(r.Loot), host.Loot...)
	d.Abilities = append(slices.Clone(host.Abilities), r.Abilities...)
	return &d
}

// Os MonsterDef dos raros e dos escudeiros entram em Monsters antes dos init (a validação
// das habilidades, em abilities.go, já os vê).
var _ = func() bool {
	for _, r := range Rares {
		host := Monsters[r.Host]
		if host == nil {
			panic("raro " + r.ID + ": hospedeiro desconhecido " + r.Host)
		}
		Monsters[r.ID] = rareMonster(r, host)
	}
	// Invocação do Capitão: não renasce e não deixa saque.
	Monsters[RareGuardID] = &MonsterDef{ID: RareGuardID, Name: "Escudeiro Ossudo", Model: "skeleton_warrior",
		HP: 70, HPLvl: 20, Atk: 7, AtkLv: 1.8, Def: 14, DefLv: 3.2, Range: 2.2, Interval: 1.6, Speed: 4, Aggressive: true,
		XPBase: 4, Scale: 1, NoGear: true}
	return true
}()
