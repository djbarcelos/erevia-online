package zone

import (
	"strings"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// mapZones resume os spawns para o mapa do cliente: um anel por faixa de raio (juntando
// os monstros que dividem o mesmo anel), um ponto para cada spawn fixo (chefes) e um
// disco para cada área própria (biomas).
func mapZones(spawns []game.Spawn) []*pb.MapZone {
	var out []*pb.MapZone
	rings := map[[2]float64]*pb.MapZone{}
	for _, s := range spawns {
		def := game.Monsters[s.Monster]
		if def == nil {
			continue
		}
		if s.Area {
			out = append(out, &pb.MapZone{
				Name: def.Name, Area: true, X: float32(s.X), Z: float32(s.Z), Radius: float32(s.Radius),
				LevelMin: uint32(s.LevelMin), LevelMax: uint32(s.LevelMax), Region: s.Region,
			})
			continue
		}
		if s.Fixed {
			out = append(out, &pb.MapZone{
				Name: def.Name, Fixed: true, X: float32(s.X), Z: float32(s.Z),
				LevelMin: uint32(s.LevelMin), LevelMax: uint32(s.LevelMin), Boss: def.Boss, Raid: def.Raid,
			})
			continue
		}
		key := [2]float64{s.RMin, s.RMax}
		if z, ok := rings[key]; ok {
			if !strings.Contains(z.Name, def.Name) {
				z.Name += ", " + def.Name
			}
			z.LevelMin = min(z.LevelMin, uint32(s.LevelMin))
			z.LevelMax = max(z.LevelMax, uint32(s.LevelMax))
			continue
		}
		z := &pb.MapZone{
			Name: def.Name, RMin: float32(s.RMin), RMax: float32(s.RMax),
			LevelMin: uint32(s.LevelMin), LevelMax: uint32(s.LevelMax),
		}
		rings[key] = z
		out = append(out, z)
	}
	return out
}
