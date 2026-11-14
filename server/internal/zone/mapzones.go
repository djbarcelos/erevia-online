package zone

import (
	"slices"
	"strings"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// mapZones resume os spawns para o mapa do cliente: um anel por faixa de raio (juntando
// os monstros que dividem o mesmo anel), um setor por região declarada num pedaço do anel
// (juntando os monstros do dia e da noite), um ponto para cada spawn fixo (chefes) e um
// disco para cada área própria (biomas; os monstros do mesmo disco também se juntam, como a
// Vigília Ossária e os Confins Partidos). Monstro de um período só leva o período no nome
// ("Aranha-de-Musgo (à noite)").
func mapZones(spawns []game.Spawn) []*pb.MapZone {
	var out []*pb.MapZone
	type ringKey struct {
		rMin, rMax, aMin, aMax float64
		region                 string
	}
	rings := map[ringKey]*pb.MapZone{}
	areas := map[[3]float64]*pb.MapZone{}
	for _, s := range spawns {
		def := game.Monsters[s.Monster]
		if def == nil {
			continue
		}
		name := def.Name
		if w := s.When.String(); w != "" {
			name += " (" + w + ")"
		}
		if s.Area {
			key := [3]float64{s.X, s.Z, s.Radius}
			if z, ok := areas[key]; ok {
				if !slices.Contains(strings.Split(z.Name, ", "), name) {
					z.Name += ", " + name
				}
				z.LevelMin = min(z.LevelMin, uint32(s.LevelMin))
				z.LevelMax = max(z.LevelMax, uint32(s.LevelMax))
				continue
			}
			z := &pb.MapZone{
				Name: name, Area: true, X: float32(s.X), Z: float32(s.Z), Radius: float32(s.Radius),
				LevelMin: uint32(s.LevelMin), LevelMax: uint32(s.LevelMax), Region: s.Region,
			}
			areas[key] = z
			out = append(out, z)
			continue
		}
		if s.Fixed {
			out = append(out, &pb.MapZone{
				Name: name, Fixed: true, X: float32(s.X), Z: float32(s.Z),
				LevelMin: uint32(s.LevelMin), LevelMax: uint32(s.LevelMin), Boss: def.Boss, Raid: def.Raid,
			})
			continue
		}
		key := ringKey{rMin: s.RMin, rMax: s.RMax}
		var aMin, aMax float64
		if s.Sector() {
			// No protocolo, o setor vai de angle_min (0–360) a angle_max > angle_min.
			start, span := s.SectorSpan()
			aMin, aMax = start, start+span
			key = ringKey{s.RMin, s.RMax, aMin, aMax, s.Region}
		}
		if z, ok := rings[key]; ok {
			if !slices.Contains(strings.Split(z.Name, ", "), name) {
				z.Name += ", " + name
			}
			z.LevelMin = min(z.LevelMin, uint32(s.LevelMin))
			z.LevelMax = max(z.LevelMax, uint32(s.LevelMax))
			continue
		}
		z := &pb.MapZone{
			Name: name, RMin: float32(s.RMin), RMax: float32(s.RMax),
			LevelMin: uint32(s.LevelMin), LevelMax: uint32(s.LevelMax),
			AngleMin: float32(aMin), AngleMax: float32(aMax),
		}
		if s.Sector() {
			z.Region = s.Region
		}
		rings[key] = z
		out = append(out, z)
	}
	return out
}
