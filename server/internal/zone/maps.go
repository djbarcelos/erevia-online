package zone

import (
	"slices"
	"strings"

	"erevia/server/internal/game"
	"erevia/server/internal/world"
)

// Os mapas que o processo sabe montar (cada um vira uma zona; ver manager.go).
//
// Por enquanto: o mapa de sempre ("starter") e a Clareira de Testes ("teste"), pequena,
// só para a troca de zona. A de testes só liga com DEV_LOGIN=true ou EREVIA_ZONES=teste:
// em produção nada muda para quem joga (sem o destino, o portão nem aparece).

// MapTest é o id do mapa de testes.
const MapTest = "teste"

type mapDef struct {
	title  string
	spawns func() []game.Spawn
	gates  []GateDef
	noTown bool
}

var mapDefs = map[string]mapDef{
	world.StarterMap: {
		title:  "Vale de Vel'Aran",
		spawns: game.DefaultSpawns,
		// Na praça, ao sul do poço, no começo da estrada sul.
		gates: []GateDef{{Name: "Portão da Clareira", X: 0, Z: -12, To: MapTest, ArriveX: 0, ArriveZ: 1}},
	},
	MapTest: {
		title: "Clareira de Testes",
		spawns: func() []game.Spawn {
			return []game.Spawn{
				{Monster: "skeleton_minion", Count: 10, RMin: 18, RMax: 52, LevelMin: 1, LevelMax: 4, Respawn: 20},
			}
		},
		gates:  []GateDef{{Name: "Portão do Vale", X: 0, Z: 5, To: world.StarterMap, ArriveX: 0, ArriveZ: -15}},
		noTown: true,
	},
}

// EnabledMaps: os mapas desta execução, o principal primeiro. zonesEnv é a variável
// EREVIA_ZONES (ids separados por vírgula); dev (DEV_LOGIN) liga o de testes.
func EnabledMaps(dev bool, zonesEnv string) []string {
	out := []string{world.StarterMap}
	add := func(id string) {
		if _, ok := mapDefs[id]; ok && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	if dev {
		add(MapTest)
	}
	for _, id := range strings.Split(zonesEnv, ",") {
		add(strings.TrimSpace(id))
	}
	return out
}

// MapConfig monta a configuração da zona do mapa id a partir de base (taxas, eventos,
// clãs...). Portões para mapas fora de enabled não aparecem.
func MapConfig(base Config, id string, enabled []string) (Config, error) {
	d := mapDefs[id]
	w, err := world.LoadMap(id)
	if err != nil {
		return base, err
	}
	cfg := base
	cfg.Map, cfg.MapTitle = id, d.title
	cfg.World, cfg.Size, cfg.SafeRadius = w, w.Size, w.SafeRadius
	if d.spawns != nil {
		cfg.Spawns = d.spawns()
	}
	cfg.Gates = nil
	for _, g := range d.gates {
		g.ToTitle = mapDefs[g.To].title
		g.disabled = !slices.Contains(enabled, g.To)
		cfg.Gates = append(cfg.Gates, g)
	}
	if d.noTown {
		// Sem a cidade: sem NPCs da praça, armazém, invasão nem raid boss; os clãs ficam
		// na zona da cidade (ver o PR das zonas: clã entre mapas fica para depois).
		cfg.NoTown = true
		cfg.InvasionChance = 0
		cfg.Clans, cfg.Bosses = nil, nil
	}
	return cfg, nil
}
