package zone

import "math"

// Vec2 é uma posição no plano do chão (X, Z). A altura (Y) virá depois, com o terreno.
type Vec2 struct {
	X, Z float64
}

// clampToZone limita v ao quadrado [-half, half] em X e Z.
func clampToZone(v Vec2, half float64) Vec2 {
	return Vec2{
		X: math.Max(-half, math.Min(half, v.X)),
		Z: math.Max(-half, math.Min(half, v.Z)),
	}
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
