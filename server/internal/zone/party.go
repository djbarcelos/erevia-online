package zone

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Grupo: até partyMax jogadores dividem a XP dos monstros (com bônus), o saque reservado
// e as moedas. Fica só em memória: sair do jogo é sair do grupo.
const (
	partyMax      = 6
	inviteTTL     = 30.0 // segundos para aceitar um convite
	partyRange    = 30.0 // distância para receber XP e moedas do grupo
	partyLevelGap = 9    // mais que isto abaixo do mais alto não recebe XP
)

// partyBonus: XP total do monstro por tamanho do grupo que recebe (1 a 6).
var partyBonus = [...]float64{1.0, 1.1, 1.2, 1.3, 1.4, 1.5}

// party é um grupo de jogadores.
type party struct {
	id      uint32
	leader  uint32
	members []uint32 // ordem de entrada (quem herda a liderança é o próximo da lista)
}

func (pt *party) has(id uint32) bool { return slices.Contains(pt.members, id) }

// invite é um convite pendente (grupo ou clã).
type invite struct {
	id      uint32
	kind    pb.InviteKind
	from    uint32
	to      uint32
	expires float64
	clanID  int64 // convite de clã: para qual clã
}

// socialEvent carrega um pedido de grupo/clã/convite/amigos/correio (*pb.PartyCmd, *pb.ClanCmd,
// *pb.InviteReply, *pb.FriendCmd, *pb.MailCmd).
type socialEvent struct {
	id  uint32
	msg any
}

func (socialEvent) isEvent() {}

func (z *Zone) onSocial(e socialEvent) {
	p := z.players[e.id]
	if p == nil || !z.spendAction(p) {
		return
	}
	switch m := e.msg.(type) {
	case *pb.PartyCmd:
		switch m.Kind {
		case pb.PartyCmdKind_PARTY_CMD_INVITE:
			z.partyInvite(p, z.findPlayer(m.TargetId, m.Name))
		case pb.PartyCmdKind_PARTY_CMD_LEAVE:
			z.partyLeave(p, "Você saiu do grupo.")
		case pb.PartyCmdKind_PARTY_CMD_KICK:
			z.partyKick(p, m.TargetId)
		case pb.PartyCmdKind_PARTY_CMD_LEADER:
			z.partyPromote(p, m.TargetId)
		}
	case *pb.InviteReply:
		z.replyInvite(p, m.InviteId, m.Accept)
	case *pb.ClanCmd:
		z.onClanCmd(p, m)
	case *pb.FriendCmd:
		z.onFriendCmd(p, m)
	case *pb.MailCmd:
		z.onMailCmd(p, m)
	case *pb.RankingRequest:
		z.sendRanking(p)
	case *pb.AchievementCmd:
		z.onAchievementCmd(p, m)
	}
}

// findPlayer: por id (botão do quadro do alvo) ou pelo nome (/convidar Nome).
func (z *Zone) findPlayer(id uint32, name string) *entity {
	if id != 0 {
		return z.players[id]
	}
	if pid, ok := z.byName[strings.ToLower(strings.TrimSpace(name))]; ok {
		return z.players[pid]
	}
	return nil
}

// ---------- Convites (grupo e clã) ----------

// pendingInviteTo: o convite ainda válido que alguém recebeu (um por vez).
func (z *Zone) pendingInviteTo(to uint32) *invite {
	for _, inv := range z.invites {
		if inv.to == to && z.now < inv.expires {
			return inv
		}
	}
	return nil
}

// sendInvite registra e envia o convite ao convidado.
func (z *Zone) sendInvite(from, to *entity, kind pb.InviteKind, extra string, clanID int64) {
	z.inviteSeq++
	inv := &invite{id: z.inviteSeq, kind: kind, from: from.id, to: to.id, expires: z.now + inviteTTL, clanID: clanID}
	z.invites[inv.id] = inv
	z.sendTo(to, &pb.ServerMessage{Msg: &pb.ServerMessage_Invite{Invite: &pb.Invite{
		Id: inv.id, Kind: kind, FromName: from.name, Extra: extra, ExpiresMs: uint32(inviteTTL * 1000),
	}}})
	z.system(from, fmt.Sprintf("Convite enviado para %s.", to.name))
}

func (z *Zone) replyInvite(p *entity, id uint32, accept bool) {
	inv := z.invites[id]
	if inv == nil || inv.to != p.id {
		return
	}
	delete(z.invites, id)
	from := z.players[inv.from]
	if z.now >= inv.expires || from == nil {
		z.system(p, "Este convite não vale mais.")
		return
	}
	if !accept {
		z.system(from, fmt.Sprintf("%s recusou o convite.", p.name))
		return
	}
	switch inv.kind {
	case pb.InviteKind_INVITE_KIND_PARTY:
		z.partyJoin(from, p)
	case pb.InviteKind_INVITE_KIND_CLAN:
		z.clanJoin(from, p, inv.clanID)
	case pb.InviteKind_INVITE_KIND_TRADE:
		z.openTrade(from, p)
	}
}

// expireInvites tira os convites vencidos (conferido 1×/s).
func (z *Zone) expireInvites() {
	for id, inv := range z.invites {
		if z.now >= inv.expires {
			delete(z.invites, id)
		}
	}
}

// dropInvites apaga os convites de/para quem saiu do jogo.
func (z *Zone) dropInvites(pid uint32) {
	for id, inv := range z.invites {
		if inv.from == pid || inv.to == pid {
			delete(z.invites, id)
		}
	}
}

// ---------- Grupo ----------

func (z *Zone) partyInvite(p, t *entity) {
	switch {
	case t == nil:
		z.system(p, "Esse jogador não está online.")
	case t == p:
		z.system(p, "Você não pode convidar a si mesmo.")
	case t.pl.party != nil && t.pl.party == p.pl.party:
		z.system(p, fmt.Sprintf("%s já está no seu grupo.", t.name))
	case t.pl.party != nil:
		z.system(p, fmt.Sprintf("%s já está em outro grupo.", t.name))
	case p.pl.party != nil && p.pl.party.leader != p.id:
		z.system(p, "Só o líder do grupo pode convidar.")
	case p.pl.party != nil && len(p.pl.party.members) >= partyMax:
		z.system(p, fmt.Sprintf("O grupo está cheio (%d pessoas).", partyMax))
	case z.pendingInviteTo(t.id) != nil:
		z.system(p, fmt.Sprintf("%s já tem um convite esperando resposta.", t.name))
	default:
		z.sendInvite(p, t, pb.InviteKind_INVITE_KIND_PARTY, "", 0)
	}
}

// partyJoin: t aceitou o convite de leader (que pode ainda não ter grupo).
func (z *Zone) partyJoin(leader, t *entity) {
	if t.pl.party != nil {
		z.system(t, "Você já está em um grupo.")
		return
	}
	pt := leader.pl.party
	if pt == nil {
		z.partySeq++
		pt = &party{id: z.partySeq, leader: leader.id, members: []uint32{leader.id}}
		z.parties[pt.id] = pt
		leader.pl.party = pt
	}
	if pt.leader != leader.id {
		z.system(t, "Quem convidou não é mais o líder do grupo.")
		return
	}
	if len(pt.members) >= partyMax {
		z.system(t, "O grupo já está cheio.")
		return
	}
	pt.members = append(pt.members, t.id)
	t.pl.party = pt
	z.partyBroadcastSystem(pt, fmt.Sprintf("%s entrou no grupo.", t.name), 0)
	z.sendParty(pt)
}

// partyLeave tira o jogador do grupo (sair, ser expulso ou sair do jogo).
func (z *Zone) partyLeave(p *entity, msg string) {
	pt := p.pl.party
	if pt == nil {
		if msg != "" {
			z.system(p, "Você não está em um grupo.")
		}
		return
	}
	pt.members = slices.DeleteFunc(pt.members, func(id uint32) bool { return id == p.id })
	p.pl.party = nil
	z.sendTo(p, &pb.ServerMessage{Msg: &pb.ServerMessage_Party{Party: &pb.PartyState{}}})
	if msg != "" {
		z.system(p, msg)
	}
	if len(pt.members) <= 1 {
		// Sobrou um: o grupo acaba.
		for _, id := range pt.members {
			if m := z.players[id]; m != nil {
				m.pl.party = nil
				z.sendTo(m, &pb.ServerMessage{Msg: &pb.ServerMessage_Party{Party: &pb.PartyState{}}})
				z.system(m, "O grupo acabou.")
			}
		}
		delete(z.parties, pt.id)
		return
	}
	text := fmt.Sprintf("%s saiu do grupo.", p.name)
	if pt.leader == p.id {
		pt.leader = pt.members[0]
		if l := z.players[pt.leader]; l != nil {
			text += fmt.Sprintf(" %s agora é o líder.", l.name)
		}
	}
	z.partyBroadcastSystem(pt, text, 0)
	z.sendParty(pt)
}

func (z *Zone) partyKick(p *entity, id uint32) {
	pt := p.pl.party
	switch {
	case pt == nil:
		z.system(p, "Você não está em um grupo.")
	case pt.leader != p.id:
		z.system(p, "Só o líder pode remover alguém do grupo.")
	case id == p.id || !pt.has(id):
		return
	default:
		if t := z.players[id]; t != nil {
			z.partyLeave(t, "Você foi removido do grupo.")
		}
	}
}

func (z *Zone) partyPromote(p *entity, id uint32) {
	pt := p.pl.party
	switch {
	case pt == nil || pt.leader != p.id:
		z.system(p, "Só o líder pode passar a liderança.")
	case id == p.id || !pt.has(id):
		return
	default:
		pt.leader = id
		if t := z.players[id]; t != nil {
			z.partyBroadcastSystem(pt, fmt.Sprintf("%s agora é o líder do grupo.", t.name), 0)
		}
		z.sendParty(pt)
	}
}

// partyMembers: os membros online do grupo.
func (z *Zone) partyMembers(pt *party) []*entity {
	out := make([]*entity, 0, len(pt.members))
	for _, id := range pt.members {
		if m := z.players[id]; m != nil {
			out = append(out, m)
		}
	}
	return out
}

func (z *Zone) partyPB(pt *party) *pb.PartyState {
	out := &pb.PartyState{LeaderId: pt.leader}
	for _, m := range z.partyMembers(pt) {
		out.Members = append(out.Members, &pb.PartyMember{
			Id: m.id, Name: m.name, Class: m.pl.class, Level: uint32(m.level),
			Hp: uint32(m.hpInt()), MaxHp: uint32(m.maxHPInt()),
			Mp: uint32(m.mpInt()), MaxMp: uint32(m.maxMPInt()),
			X: float32(m.pos.X), Z: float32(m.pos.Z), Dead: m.dead,
		})
	}
	return out
}

// sendParty manda o estado do grupo a todos os membros.
func (z *Zone) sendParty(pt *party) {
	msg := &pb.ServerMessage{Msg: &pb.ServerMessage_Party{Party: z.partyPB(pt)}}
	for _, m := range z.partyMembers(pt) {
		z.sendTo(m, msg)
	}
}

// syncParties: 1×/s cada grupo recebe vida, mana e posição dos membros.
func (z *Zone) syncParties() {
	for _, pt := range z.parties {
		z.sendParty(pt)
	}
}

func (z *Zone) partyBroadcastSystem(pt *party, text string, skip uint32) {
	for _, m := range z.partyMembers(pt) {
		if m.id != skip {
			z.system(m, text)
		}
	}
}

// partyChat manda a mensagem só aos membros do grupo.
func (z *Zone) partyChat(p *entity, text string) {
	if p.pl.party == nil {
		z.system(p, "Você não está em um grupo.")
		return
	}
	msg := &pb.ServerMessage{Msg: &pb.ServerMessage_Chat{Chat: &pb.ChatMessage{
		Channel: pb.ChatChannel_CHAT_CHANNEL_PARTY, FromId: p.id, FromName: p.name, Text: text,
	}}}
	for _, m := range z.partyMembers(p.pl.party) {
		z.sendTo(m, msg)
	}
}

// ---------- XP e moedas do grupo ----------

// creditGroup: quem leva a XP do monstro. Soma o dano (ódio) por grupo (ou por jogador
// sozinho); o maior total vence. Recebem os membros vivos a até partyRange do monstro e
// no máximo partyLevelGap níveis abaixo do mais alto deles.
func (z *Zone) creditGroup(hate map[uint32]float64, at Vec2) []*entity {
	type key struct {
		party uint32
		solo  uint32
	}
	totals := map[key]float64{}
	var best key
	bestV := -1.0
	for id, v := range hate {
		p := z.players[id]
		if p == nil {
			continue
		}
		k := key{solo: id}
		if p.pl.party != nil {
			k = key{party: p.pl.party.id}
		}
		totals[k] += v
	}
	// Maior dano vence; empate fica com o menor id (resultado estável).
	for k, v := range totals {
		if v > bestV || (v == bestV && k.party+k.solo < best.party+best.solo) {
			best, bestV = k, v
		}
	}
	if bestV < 0 {
		return nil
	}
	var cands []*entity
	if best.party != 0 {
		if pt := z.parties[best.party]; pt != nil {
			cands = z.partyMembers(pt)
		}
	} else if p := z.players[best.solo]; p != nil {
		cands = []*entity{p}
	}
	return z.eligible(cands, at)
}

// eligible: dos candidatos, os vivos a até partyRange de at e no máximo partyLevelGap
// níveis abaixo do mais alto deles.
func (z *Zone) eligible(cands []*entity, at Vec2) []*entity {
	var near []*entity
	top := 0
	for _, m := range cands {
		if !m.dead && dist(m.pos, at) <= partyRange {
			near = append(near, m)
			top = max(top, m.level)
		}
	}
	out := near[:0]
	for _, m := range near {
		if top-m.level <= partyLevelGap {
			out = append(out, m)
		}
	}
	return out
}

// shareMonsterXP divide a XP do monstro entre quem recebe (proporcional ao nível).
func (z *Zone) shareMonsterXP(dead *entity, takers []*entity) {
	z.shareMonsterXPScaled(dead, takers, 1)
}

// shareMonsterXPScaled: como shareMonsterXP, com só uma fração da XP (raid: a parte do
// dano daquele lado).
func (z *Zone) shareMonsterXPScaled(dead *entity, takers []*entity, frac float64) {
	if len(takers) == 0 {
		return
	}
	top, sum := 0, 0
	for _, m := range takers {
		top = max(top, m.level)
		sum += m.level
	}
	total := float64(game.MonsterXP(dead.mon.def, dead.level, top))
	if z.night {
		total *= game.NightXP
	}
	total *= z.xpEventMult() // XP em dobro (events.go)
	total *= partyBonus[min(len(takers), len(partyBonus))-1] * frac
	for _, m := range takers {
		if xp := int64(math.Round(total * float64(m.level) / float64(sum))); xp > 0 {
			z.tallyMonsterXP(m, xp, z.night, z.xpEventMult(), len(takers) > 1) // telemetria (xptally.go)
			z.grantXP(m, xp)
		}
		z.burnKarma(m, dead.level)
		z.track(m, "kill", dead.mon.def.ID, int64(dead.level), nil)
		z.questEvent(m, game.GoalKill, dead.mon.def.ID)
		z.achEvent(m, game.AchKill, dead.mon.def.ID)
		if dead.mon.raid == nil { // raid boss: conta para todos que causaram dano (raidKilled)
			z.bestiaryKill(m, dead)
		}
	}
}

// canTakeReserved: o item reservado pode ser pego pelo dono, por quem está no grupo dele
// ou (saque do raid boss ganho por um clã) por quem é do clã.
func (z *Zone) canTakeReserved(p *entity, g *groundItem) bool {
	if g.clan != 0 && p.pl.clan != nil && p.pl.clan.id == g.clan {
		return true
	}
	owner := g.owner
	if owner == 0 || owner == p.id {
		return true
	}
	o := z.players[owner]
	return o != nil && o.pl.party != nil && o.pl.party == p.pl.party
}

// splitGold: moedas pegas por alguém do grupo são divididas entre os membros vivos perto
// (o resto da divisão fica com quem pegou). Devolve quanto ficou com quem pegou.
func (z *Zone) splitGold(p *entity, amount int) int {
	if p.pl.party == nil || amount <= 0 {
		return amount
	}
	var share []*entity
	for _, m := range z.partyMembers(p.pl.party) {
		if m != p && !m.dead && dist(m.pos, p.pos) <= partyRange {
			share = append(share, m)
		}
	}
	if len(share) == 0 {
		return amount
	}
	each := amount / (len(share) + 1)
	if each <= 0 {
		return amount
	}
	for _, m := range share {
		m.pl.inv.gold += int64(each)
		z.trackGold(m, "loot", int64(each))
		z.system(m, fmt.Sprintf("+%d moedas (grupo).", each))
		z.inventoryChanged(m, false)
	}
	return amount - each*len(share)
}
