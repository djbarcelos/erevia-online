package zone

// Fila de caminhos longos. O A* da grade de 1 m, num caminho que atravessa o mapa,
// expande dezenas de milhares de células (até ~4 ms num M2, ver nav_bench_test.go).
// Pedidos de movimento longos e com obstáculo no meio (o "Ir" das missões, o clique no
// mapa do mundo) entram nesta fila e no máximo longPathsPerTick são calculados por tick;
// os curtos (clicar no chão por perto, WASD, perseguir) seguem na hora como sempre.
// Cada jogador ocupa um lugar só: um pedido novo troca o destino do que já espera, então
// mandar muitos pedidos não fura a fila de ninguém (e o limite de taxa de sempre vale antes).
const (
	longPathDist     = 40.0 // metros em linha reta: acima disso (com obstáculo) vai para a fila
	longPathsPerTick = 2    // caminhos longos calculados por tick (20/s a 20 Hz)
)

// requestPath põe o jogador a caminho de goal: na hora, se for curto ou sem obstáculo;
// senão na fila (ele para onde está até a vez dele, em geral o próximo tick).
func (z *Zone) requestPath(p *entity, goal Vec2) {
	w := z.cfg.World
	if w == nil || dist(p.pos, goal) <= longPathDist || w.SegmentClear(toPt(p.pos), toPt(goal)) {
		p.pl.pathGoal = nil
		z.goTo(p, goal)
		return
	}
	p.stopMoving()
	if p.pl.pathGoal == nil {
		z.paths = append(z.paths, p)
	}
	p.pl.pathGoal = &goal
}

// updatePaths calcula os primeiros da fila (uma vez por tick). Quem saiu, morreu, atacou,
// começou a canalizar ou mandou outro movimento (pathGoal nil) é pulado sem gastar a vez.
func (z *Zone) updatePaths() {
	n, done := 0, 0
	for n < len(z.paths) && done < longPathsPerTick {
		p := z.paths[n]
		z.paths[n] = nil
		n++
		g := p.pl.pathGoal
		if g == nil || z.players[p.id] != p {
			continue
		}
		p.pl.pathGoal = nil
		if p.dead || p.act.kind != actNone || p.pl.channel != nil || p.stunned(z.now) {
			continue
		}
		z.goTo(p, *g)
		done++
	}
	if n == len(z.paths) {
		z.paths = z.paths[:0]
	} else {
		z.paths = z.paths[n:]
	}
}
