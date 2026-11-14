package zone

import (
	"fmt"
	"maps"
	"slices"

	"google.golang.org/protobuf/proto"

	"erevia/server/internal/pb"
)

// Grupo entre zonas (com o gerente; sem ele, party.go cuida de tudo como sempre).
//
// O grupo de verdade mora no barramento (Bus.parties): membros e líder. Cada zona tem uma
// cópia (z.parties[id], o mesmo *party de p.pl.party), com a lista inteira de membros,
// inclusive os de outras zonas. As operações (entrar, sair, expulsar, passar a liderança)
// são feitas no barramento, que devolve a versão nova; a zona aplica na cópia dela e
// cutuca as outras zonas do grupo (que conferem na hora, ou no máximo 1 s depois, em
// syncParties). Assim:
//   - XP e moedas do grupo (creditGroup, splitGold) e o saque reservado continuam usando
//     só quem está nesta zona: a XP divide entre os membros no mesmo mapa;
//   - a janela do grupo mostra todo mundo; quem está em outro mapa vem com o nome dele
//     (PartyMember.map), e a vida e o nível de 1 s atrás.

// applyParty põe a cópia local igual à versão do barramento e liga os membros daqui.
// Quem saiu do grupo e está aqui é desligado (com o aviso guardado para ele, se houver).
func (z *Zone) applyParty(v partyView) *party {
	pt := z.parties[v.id]
	if pt == nil {
		pt = &party{id: v.id}
		z.parties[v.id] = pt
	}
	for _, id := range pt.members {
		if !slices.Contains(v.members, id) {
			if m := z.players[id]; m != nil && m.pl.party == pt {
				z.detachParty(m, z.mgr.bus.takeNote(id), "Você saiu do grupo.")
			}
		}
	}
	pt.leader = v.leader
	pt.members = append(pt.members[:0], v.members...)
	for _, id := range pt.members {
		if m := z.players[id]; m != nil {
			m.pl.party = pt
		}
	}
	z.dropPartyCopy(pt)
	return pt
}

// detachParty: o jogador daqui não está mais no grupo (janela vazia e o aviso).
func (z *Zone) detachParty(m *entity, note, fallback string) {
	m.pl.party = nil
	z.sendTo(m, &pb.ServerMessage{Msg: &pb.ServerMessage_Party{Party: &pb.PartyState{}}})
	if note == "" {
		note = fallback
	}
	if note != "" {
		z.system(m, note)
	}
}

// dropPartyCopy apaga a cópia se ninguém desta zona aponta para ela (invariante
// checkParties: grupo na tabela sem membro aqui não existe).
func (z *Zone) dropPartyCopy(pt *party) {
	for _, id := range pt.members {
		if m := z.players[id]; m != nil && m.pl.party == pt {
			return
		}
	}
	if z.parties[pt.id] == pt {
		delete(z.parties, pt.id)
	}
}

// syncParty confere a cópia com o barramento (acabou, alguém saiu, novo líder...).
func (z *Zone) syncParty(id uint32) {
	pt := z.parties[id]
	if pt == nil {
		return
	}
	v, ok := z.mgr.bus.party(id)
	if !ok {
		for _, mid := range pt.members {
			if m := z.players[mid]; m != nil && m.pl.party == pt {
				z.detachParty(m, z.mgr.bus.takeNote(mid), "O grupo acabou.")
			}
		}
		delete(z.parties, id)
		return
	}
	pt = z.applyParty(v)
	if z.parties[id] == pt {
		z.sendParty(pt)
	}
}

// pokeParty: as outras zonas do grupo conferem já (sem esperar o 1×/s). A fila cheia só
// atrasa: o syncParties de lá pega a mudança.
func (z *Zone) pokeParty(members []uint32) {
	ids := slices.Clone(members)
	for _, oz := range z.mgr.bus.zonesOf(ids, z) {
		ev := socialDone{fn: func() {
			var due []uint32
			for id, pt := range oz.parties {
				if slices.ContainsFunc(ids, pt.has) {
					due = append(due, id)
				}
			}
			for _, id := range due {
				oz.syncParty(id)
			}
		}}
		select {
		case oz.events <- ev:
		default:
		}
	}
}

func (z *Zone) partyJoinZones(leader, t *entity) {
	if t.pl.party != nil {
		z.system(t, "Você já está em um grupo.")
		return
	}
	v, bad := z.mgr.bus.partyAdd(leader.id, t.id)
	if bad != "" {
		z.system(t, bad)
		return
	}
	pt := z.applyParty(v)
	z.partyBroadcastSystem(pt, fmt.Sprintf("%s entrou no grupo.", t.name), 0)
	z.sendParty(pt)
	z.pokeParty(v.members)
}

func (z *Zone) partyLeaveZones(p *entity, msg string) {
	pt := p.pl.party
	if pt == nil {
		if msg != "" {
			z.system(p, "Você não está em um grupo.")
		}
		return
	}
	v, ended, was := z.mgr.bus.partyRemove(p.id, "")
	p.pl.party = nil
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Party{Party: &pb.PartyState{}}})
	if msg != "" {
		z.system(p, msg)
	}
	if !was { // a cópia estava atrasada (já tinha saído por outra zona)
		z.syncParty(pt.id)
		return
	}
	if ended {
		for _, id := range v.members {
			if m := z.players[id]; m != nil && m.pl.party == pt {
				z.detachParty(m, "", "O grupo acabou.")
			}
		}
		delete(z.parties, pt.id)
		z.pokeParty(v.members)
		return
	}
	text := fmt.Sprintf("%s saiu do grupo.", p.name)
	if v.leader != pt.leader {
		if name := z.memberName(v.leader); name != "" {
			text += fmt.Sprintf(" %s agora é o líder.", name)
		}
	}
	pt = z.applyParty(v)
	z.partyBroadcastSystem(pt, text, 0)
	if z.parties[pt.id] == pt {
		z.sendParty(pt)
	}
	z.pokeParty(v.members)
}

func (z *Zone) partyKickZones(p *entity, id uint32) {
	if t := z.players[id]; t != nil && t.pl.party != nil && t.pl.party == p.pl.party {
		if p.pl.party.leader != p.id {
			z.system(p, "Só o líder pode remover alguém do grupo.")
			return
		}
		if id != p.id {
			z.partyLeaveZones(t, "Você foi removido do grupo.")
		}
		return
	}
	// Em outra zona: o barramento tira, e a zona dele avisa ao conferir.
	name := z.memberName(id)
	v, ended, bad := z.mgr.bus.partyKick(p.id, id, "Você foi removido do grupo.")
	switch bad {
	case "":
	case "-":
		return
	default:
		z.system(p, bad)
		return
	}
	pt := p.pl.party
	if ended {
		for _, mid := range v.members {
			if m := z.players[mid]; m != nil && m.pl.party == pt {
				z.detachParty(m, "", "O grupo acabou.")
			}
		}
		delete(z.parties, pt.id)
		z.pokeParty(append(v.members, id))
		return
	}
	pt = z.applyParty(v)
	z.partyBroadcastSystem(pt, fmt.Sprintf("%s saiu do grupo.", name), 0)
	z.sendParty(pt)
	z.pokeParty(append(v.members, id))
}

func (z *Zone) partyPromoteZones(p *entity, id uint32) {
	v, bad := z.mgr.bus.partyLead(p.id, id)
	switch bad {
	case "":
	case "-":
		return
	default:
		z.system(p, bad)
		return
	}
	pt := z.applyParty(v)
	z.partyBroadcastSystem(pt, fmt.Sprintf("%s agora é o líder do grupo.", z.memberName(id)), 0)
	z.sendParty(pt)
	z.pokeParty(v.members)
}

// memberName: o nome de um membro aqui ou em outra zona.
func (z *Zone) memberName(id uint32) string {
	if m := z.players[id]; m != nil {
		return m.name
	}
	if z.mgr != nil {
		z.mgr.bus.mu.RLock()
		defer z.mgr.bus.mu.RUnlock()
		if e := z.mgr.bus.players[id]; e != nil {
			return e.name
		}
	}
	return ""
}

// syncPartiesZones (1×/s): publica vida e nível dos membros daqui, confere cada cópia e
// manda a janela do grupo.
func (z *Zone) syncPartiesZones() {
	var pub []*entity
	for _, pt := range z.parties {
		for _, m := range z.partyMembers(pt) {
			pub = append(pub, m)
		}
	}
	z.mgr.bus.publish(pub)
	for _, id := range slices.Collect(maps.Keys(z.parties)) {
		z.syncParty(id)
	}
}

// partyRemotePB acrescenta à janela do grupo quem está em outra zona.
func (z *Zone) partyRemotePB(pt *party, out *pb.PartyState) {
	if z.mgr == nil {
		return
	}
	for _, r := range z.mgr.bus.remoteMembers(pt.members, z, nil) {
		s := r.m.sum
		out.Members = append(out.Members, &pb.PartyMember{
			Id: r.id, Name: r.m.name, Class: r.m.class, Level: uint32(s.level),
			Hp: uint32(s.hp), MaxHp: uint32(s.maxHP), Mp: uint32(s.mp), MaxMp: uint32(s.maxMP),
			Dead: s.dead, Map: r.m.mapTitle,
		})
	}
}

// partySendRemote manda a mensagem aos membros do grupo em outras zonas.
func (z *Zone) partySendRemote(pt *party, msg *pb.ServerMessage, skip uint32) {
	if z.mgr == nil {
		return
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		return
	}
	z.mgr.bus.sendMembers(pt.members, z, data, skip)
}
