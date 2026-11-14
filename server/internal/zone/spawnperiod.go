package zone

import (
	"cmp"
	"slices"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Spawns por período (game.When). Na virada do dia para a noite, os monstros WhenDay que
// estão fora de combate saem do mundo (ficam dormant) e os WhenNight entram no lugar; no
// amanhecer, o contrário. A troca é por região (Spawn.Region): os noturnos de uma região
// ocupam as vagas dos diurnos da mesma região.
//
// Quem está lutando não some no meio da luta: ele continua no mundo e segura a vaga de um
// monstro do período novo. Quando a luta acaba (ou ele morre), ele sai e o do período novo
// entra no mesmo segundo. Assim, com a mesma quantidade de spawns de dia e de noite em cada
// região, a contagem de monstros no mundo fica igual na virada e depois dela.
//
// Roda na virada e 1×/s (para quem saiu de combate depois dela).
func (z *Zone) updatePeriodSpawns() {
	type group struct {
		inPeriod []*entity // do período atual (no mundo ou esperando a vaga)
		awake    int       // dos do período atual, os que não estão dormant
		holding  int       // do outro período, ainda no mundo porque estão lutando
	}
	var groups map[string]*group
	for _, m := range z.monsters {
		ms := m.mon
		w := ms.spawn.When
		if w == game.WhenAny || ms.summon {
			continue
		}
		if groups == nil {
			groups = map[string]*group{}
		}
		g := groups[ms.spawn.Region]
		if g == nil {
			g = &group{}
			groups[ms.spawn.Region] = g
		}
		if w.Active(z.night) {
			g.inPeriod = append(g.inPeriod, m)
			if !ms.dormant {
				g.awake++
			}
			continue
		}
		if ms.dormant {
			continue
		}
		if m.dead || !z.monsterFighting(m) {
			z.putToSleep(m)
			continue
		}
		g.holding++
	}
	for _, g := range groups {
		want := len(g.inPeriod) - g.holding
		if g.awake >= want {
			continue
		}
		// Entra primeiro quem tem o menor id (a ordem não depende do mapa).
		slices.SortFunc(g.inPeriod, func(a, b *entity) int { return cmp.Compare(a.id, b.id) })
		for _, m := range g.inPeriod {
			if g.awake >= want {
				break
			}
			if m.mon.dormant {
				m.mon.dormant = false
				z.reviveMonster(m) // no ponto de origem, inteiro (a AOI manda o Spawn)
				z.bossChanged(m)
				g.awake++
			}
		}
	}
}

// monsterFighting: o monstro está em combate (tem ódio de alguém, está atacando ou levou ou
// deu um golpe há pouco, como o veneno de quem já foi embora).
func (z *Zone) monsterFighting(m *entity) bool {
	return len(m.mon.hate) > 0 || m.act.kind == actAttack || z.now-m.lastCombat <= outOfCombatAfter
}

// putToSleep tira o monstro do mundo até o período dele voltar. Vivo, ele some sem morrer
// (sem saque nem XP); morto, o corpo some no tempo de sempre e ele não renasce. Os avisos
// no chão pendentes dele são cancelados no próximo tick (abilGen muda) e, quando ele
// voltar, as habilidades esperam o First de novo.
func (z *Zone) putToSleep(m *entity) {
	ms := m.mon
	ms.dormant = true
	ms.stopAbilities()
	ms.windowStart, ms.windowEnd = time.Time{}, time.Time{}
	if m.dead {
		return
	}
	// Sai da luta e da IA já (dead), mas o corpo só some depois de mais um tick inteiro
	// (corpseEnd): quem o conhece ainda recebe o cancelamento dos avisos no chão dele
	// (updateTelegraphs vê o dead) e só depois o Despawn. Sem frame.deaths nem HP zerado: o
	// cliente não vê uma morte.
	m.dead = true
	ms.corpseEnd = z.now + 1.5/float64(z.cfg.TickRate)
	ms.returning = false
	ms.hate = map[uint32]float64{}
	for _, ef := range m.effects {
		z.frame.ends = append(z.frame.ends, &pb.EffectEnd{TargetId: m.id, Effect: pb.Effect(ef.kind)})
	}
	m.effects = nil
	m.clearAction()
	m.stopMoving()
	z.bossChanged(m)
}
