package zone

import "erevia/server/internal/world"

// Navegação: caminhos que contornam obstáculos e colisão ao andar.
// Sem Config.World (testes antigos), tudo anda em linha reta como antes.

const repathEvery = 0.5 // segundos entre recálculos de caminho ao perseguir

func toPt(v Vec2) world.Point   { return world.Point{X: v.X, Z: v.Z} }
func fromPt(p world.Point) Vec2 { return Vec2{X: p.X, Z: p.Z} }

func (z *Zone) walkable(v Vec2) bool {
	return z.cfg.World == nil || !z.cfg.World.Blocked(v.X, v.Z)
}

// freeSpot devolve v, ou o ponto livre mais próximo se v estiver dentro de um obstáculo.
func (z *Zone) freeSpot(v Vec2) Vec2 {
	if z.cfg.World == nil {
		return v
	}
	return fromPt(z.cfg.World.NearestFree(toPt(v)))
}

// goTo põe a entidade a caminho de goal, contornando obstáculos se a linha reta estiver bloqueada.
func (z *Zone) goTo(e *entity, goal Vec2) {
	w := z.cfg.World
	if w == nil || w.SegmentClear(toPt(e.pos), toPt(goal)) {
		e.path = nil
		if goal != e.dest {
			e.dest = goal
			e.moved = true
		}
		return
	}
	pts := w.FindPath(toPt(e.pos), toPt(goal))
	if len(pts) == 0 {
		e.stopMoving()
		return
	}
	e.dest = fromPt(pts[0])
	e.path = e.path[:0]
	for _, p := range pts[1:] {
		e.path = append(e.path, fromPt(p))
	}
	e.moved = true
}

// moveToward é goTo para alvos que se mexem (perseguição): linha reta quando dá,
// e recalcula o caminho no máximo a cada repathEvery quando há obstáculo.
func (z *Zone) moveToward(e *entity, goal Vec2) {
	w := z.cfg.World
	if w == nil || w.SegmentClear(toPt(e.pos), toPt(goal)) {
		e.path = nil
		e.setDest(goal)
		return
	}
	if z.now >= e.repathAt {
		e.repathAt = z.now + repathEvery
		z.goTo(e, goal)
	}
}

// advance anda a entidade um passo: segue o caminho, e ao bater num obstáculo desliza
// no eixo livre (X ou Z) ou para.
func (z *Zone) advance(e *entity, dist float64) {
	if e.pos == e.dest && len(e.path) > 0 {
		e.dest = e.path[0]
		e.path = e.path[1:]
		e.moved = true
	}
	newPos, moved := step(e.pos, e.dest, dist)
	if !moved {
		return
	}
	if !z.walkable(newPos) {
		slid := false
		for _, alt := range [2]Vec2{{X: newPos.X, Z: e.pos.Z}, {X: e.pos.X, Z: newPos.Z}} {
			if alt != e.pos && z.walkable(alt) {
				newPos, slid = alt, true
				break
			}
		}
		if !slid {
			e.stopMoving()
			return
		}
	}
	e.pos = newPos
	e.moved = true
}
