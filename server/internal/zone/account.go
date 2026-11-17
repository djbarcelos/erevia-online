package zone

import (
	"context"
	"fmt"
	"slices"

	"google.golang.org/protobuf/proto"

	"erevia/server/internal/pb"
)

// Conta apagada (LGPD). O banco apaga primeiro (store.DeleteAccount, que também passa a
// liderança dos clãs); depois a zona derruba os personagens da conta que estiverem online,
// esquece o que guardava deles em memória e acerta os clãs do mesmo jeito que o banco
// ficou. O salvamento que sair depois disso (ao derrubar ou o periódico) não acha a linha
// no banco e não grava nada.

// AccountGone é o que sumiu do banco.
type AccountGone struct {
	AccountID  int64
	Characters []int64
	// Clans: clãs que eram liderados por um personagem da conta e quem o banco pôs no
	// lugar (0 = o clã foi desfeito).
	Clans []ClanSuccession
}

type ClanSuccession struct {
	ClanID   int64
	LeaderID int64
}

const accountGoneReason = "Esta conta foi apagada."

type accountGoneEvent struct{ d AccountGone }

func (accountGoneEvent) isEvent() {}

// AccountDeleted avisa a zona que a conta foi apagada do banco.
func (z *Zone) AccountDeleted(ctx context.Context, d AccountGone) {
	z.post(ctx, accountGoneEvent{d: d})
}

func (z *Zone) onAccountGone(d AccountGone) {
	if z.gone == nil {
		z.gone = map[uint32]bool{}
	}
	touched := map[*clan]bool{}
	for _, cid := range d.Characters {
		id := uint32(cid)
		z.gone[id] = true // um Join atrasado (personagem carregado antes de apagar) é recusado
		if p := z.players[id]; p != nil {
			z.sendTo(p, kickedMsg(accountGoneReason))
			p.pl.peer.Close("account deleted")
			z.removePlayer(id)
		}
		delete(z.recent, id)
		delete(z.savedCooldowns, id)
		delete(z.pending, id)
		delete(z.clanLeft, cid)
		if c := z.clanOf[cid]; c != nil {
			if m := c.members[cid]; m != nil && c.leader != cid {
				z.clanSystem(c, fmt.Sprintf("%s saiu do clã.", m.name))
			}
			delete(c.members, cid)
			delete(z.clanOf, cid)
			touched[c] = true
		}
	}
	delete(z.storages, d.AccountID)

	succ := map[int64]int64{}
	for _, s := range d.Clans {
		succ[s.ClanID] = s.LeaderID
	}
	for c := range touched {
		if z.clans[c.id] != c {
			continue
		}
		if !slices.Contains(d.Characters, c.leader) {
			z.sendClan(c)
			continue
		}
		next, known := succ[c.id]
		if known && next == 0 { // o banco desfez o clã: a memória segue
			z.disbandGone(c)
			continue
		}
		if c.members[next] == nil { // o banco não sabia (ou escolheu quem já saiu): escolhe aqui
			next = clanSuccessor(c)
		}
		if next == 0 {
			z.disbandGone(c)
			continue
		}
		m := c.members[next]
		m.rank = rankLeader
		c.leader = next
		// Repete o que o banco fez: idempotente, e passa na frente de nada que já esteja na fila.
		z.clanOp(ClanOp{Kind: ClanOpLeader, ClanID: c.id, CharacterID: next})
		z.clanSystem(c, fmt.Sprintf("%s agora é o líder do clã.", m.name))
		z.sendClan(c)
	}
	z.log.Info("account deleted", "account", d.AccountID, "characters", len(d.Characters))
}

// clanSuccessor: cargo mais alto, depois o personagem mais antigo (menor id). 0 = ninguém.
func clanSuccessor(c *clan) int64 {
	var best *clanMember
	for _, m := range c.members {
		if best == nil || m.rank > best.rank || (m.rank == best.rank && m.id < best.id) {
			best = m
		}
	}
	if best == nil {
		return 0
	}
	return best.id
}

// disbandGone desfaz o clã cujo líder foi apagado e não tem quem assuma.
func (z *Zone) disbandGone(c *clan) {
	z.clanOp(ClanOp{Kind: ClanOpDisband, ClanID: c.id})
	delete(z.clans, c.id)
	for id := range c.members {
		delete(z.clanOf, id)
		if m := z.players[uint32(id)]; m != nil {
			m.pl.clan = nil
			z.system(m, fmt.Sprintf("O clã ‹%s› foi desfeito.", c.name))
			z.broadcastStatus(m)
			z.sendTo(m, &pb.ServerMessage{Msg: &pb.ServerMessage_Clan{Clan: z.clanPB(m)}})
		}
	}
	z.log.Info("clan disbanded", "clan", c.id, "name", c.name, "reason", "leader account deleted")
}

// refuseGone derruba a conexão de um personagem que já foi apagado.
func (z *Zone) refuseGone(info JoinInfo) bool {
	if !z.gone[info.ID] {
		return false
	}
	if data, err := proto.Marshal(kickedMsg(accountGoneReason)); err == nil {
		info.Peer.Send(data)
	}
	info.Peer.Close("account deleted")
	return true
}

func kickedMsg(reason string) *pb.ServerMessage {
	return &pb.ServerMessage{Msg: &pb.ServerMessage_Kicked{Kicked: &pb.Kicked{Reason: reason}}}
}
