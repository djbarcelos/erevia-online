package zone

import (
	"fmt"
	"slices"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Descoberta das Pedras de Teleporte (teleport.go): o personagem só vê um destino depois de
// chegar perto da pedra andando. Chegar por teleporte, pergaminho, portão, login ou qualquer
// salto (entity.teleportTo) não conta: as pedras perto de onde o personagem pousou ficam
// "emudecidas" até ele sair do raio delas, e só uma volta a pé as descobre.
//
// Gravação: characters.discovered (migração 00036), junto com o personagem. A última cidade
// visitada (destino do Pergaminho de Retorno) vai no mesmo estado.
type discovery struct {
	known map[string]bool
	order []string // a ordem em que foram descobertas (o estado salvo é estável)
	town  string   // chave da última cidade visitada
	// jumped: o personagem pousou (login ou salto); o próximo tick recalcula quais pedras
	// estão perto (near) e não conta como descoberta.
	jumped bool
	near   map[int]bool
}

// loadDiscovery monta o estado salvo. Chaves que não existem mais são descartadas; quem
// não tem nada (personagem novo) começa com a pedra da cidade.
func loadDiscovery(st game.DiscoveryState) discovery {
	d := discovery{known: map[string]bool{}, jumped: true}
	for _, k := range st.Stones {
		if stoneIndex(k) >= 0 && !d.known[k] {
			d.known[k] = true
			d.order = append(d.order, k)
		}
	}
	if len(d.order) == 0 {
		d.known[allStones[0].key] = true
		d.order = append(d.order, allStones[0].key)
	}
	if i := stoneIndex(st.Town); i >= 0 && allStones[i].town {
		d.town = st.Town
	}
	return d
}

func (d *discovery) state() game.DiscoveryState {
	return game.DiscoveryState{Stones: slices.Clone(d.order), Town: d.town}
}

// updateDiscovery: a cada tick, descobre as pedras de perto a que o jogador chegou a pé e
// guarda a última cidade visitada.
func (z *Zone) updateDiscovery(p *entity) {
	if len(z.stoneIdx) == 0 { // mapa sem pedras
		return
	}
	d := &p.pl.disc
	near := func(i int) bool { return stoneNear(p.pos, allStones[i]) }
	if d.jumped {
		d.jumped = false
		d.near = map[int]bool{}
		for _, i := range z.stoneIdx {
			if near(i) {
				d.near[i] = true
			}
		}
	}
	for _, i := range z.stoneIdx {
		s := allStones[i]
		if s.ferry {
			continue
		}
		if !near(i) {
			delete(d.near, i)
			continue
		}
		if s.town && d.known[s.key] {
			d.town = s.key
		}
		if d.near[i] || d.known[s.key] {
			continue
		}
		d.known[s.key] = true
		d.order = append(d.order, s.key)
		if s.town {
			d.town = s.key
		}
		z.system(p, fmt.Sprintf("Pedra de Teleporte descoberta: %s (%s).", s.name, s.region))
		z.track(p, "stone", s.key, 0, nil)
		z.sendStonesKnown(p)
	}
}

// stoneNear: pos está a discoverRange ou menos da pedra.
func stoneNear(pos Vec2, s stoneDef) bool {
	dx, dz := pos.X-s.x, pos.Z-s.z
	return dx*dx+dz*dz <= discoverRange*discoverRange
}

// sendStonesKnown manda os ids dos destinos que o jogador conhece (a balsa vale sempre).
func (z *Zone) sendStonesKnown(p *entity) {
	var ids []uint32
	for i, s := range allStones {
		if s.ferry || p.pl.disc.known[s.key] {
			ids = append(ids, uint32(i+1))
		}
	}
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_StonesKnown{StonesKnown: &pb.StonesKnown{Ids: ids}}})
}
