package zone

import (
	"math"

	"erevia/server/internal/world"
)

// Vec2 é uma posição no plano do chão (X, Z). A altura (Y) virá depois, com o terreno.
type Vec2 struct {
	X, Z float64
}

// clampToZone limita v ao retângulo jogável.
func clampToZone(v Vec2, b world.Bounds) Vec2 {
	p := b.Clamp(world.Point{X: v.X, Z: v.Z})
	return Vec2{X: p.X, Z: p.Z}
}

// bounds é o retângulo jogável: o do mapa carregado ou, sem mapa, o quadrado Size.
func (z *Zone) bounds() world.Bounds {
	if z.cfg.World != nil {
		return z.cfg.World.Bounds
	}
	return world.Square(z.cfg.Size)
}

// step move pos em direção a target, andando no máximo dist.
// Retorna a nova posição e se houve movimento.
func step(pos, target Vec2, dist float64) (Vec2, bool) {
	dx, dz := target.X-pos.X, target.Z-pos.Z
	remaining := math.Hypot(dx, dz)
	if remaining == 0 {
		return pos, false
	}
	if remaining <= dist {
		return target, true
	}
	k := dist / remaining
	return Vec2{X: pos.X + dx*k, Z: pos.Z + dz*k}, true
}
