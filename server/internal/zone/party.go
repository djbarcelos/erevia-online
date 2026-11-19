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
// As regras da XP em grupo (bônus, diferença de nível, distância e ajuda) estão em
// game/party.go e no fim deste arquivo.
const (
	partyMax   = game.PartyMaxMembers
	inviteTTL  = 30.0              // segundos para aceitar um convite
	partyRange = game.PartyXPRange // distância para receber XP e moedas do grupo
	// Raid boss: a XP é por fatia de dano de cada lado (raid.go) e quem está mais que isto
	// abaixo do mais alto do lado fica sem; o corte do grupo inteiro (game.PartyGapMult)
	// não vale lá, porque o lado pode ser um clã inteiro.
	raidLevelGap = 9
)

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
			if t := z.findPlayer(m.TargetId, m.Name); t == nil && m.TargetId == 0 && z.elsewhere(m.Name) != "" {
				z.system(p, fmt.Sprintf("%s está em outro mapa (%s): convide quando estiverem no mesmo.", m.Name, z.elsewhere(m.Name)))
			} else {
				z.partyInvite(p, t)
			}
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
		if z.cfg.NoTown { // os clãs moram na zona da cidade (maps.go)
			z.system(p, "O clã só pode ser gerenciado no Vale de Vel'Aran, por enquanto.")
			return
		}
		z.onClanCmd(p, m)
	case *pb.FriendCmd:
		z.onFriendCmd(p, m)
	case *pb.MailCmd:
		z.onMailCmd(p, m)
	case *pb.RankingRequest:
		z.sendRanking(p)
	case *pb.AchievementCmd:
		z.onAchievementCmd(p, m)
	case *pb.AuctionCmd:
		z.onAuctionCmd(p, m)
	case *pb.PrivateShopCmd:
		z.onPrivateShopCmd(p, m)
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
	if z.mgr != nil { // grupo entre zonas (party_zones.go)
		z.partyJoinZones(leader, t)
		return
	}
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
	if z.mgr != nil {
		z.partyLeaveZones(p, msg)
		return
	}
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
	case z.mgr != nil:
		z.partyKickZones(p, id)
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
	case z.mgr != nil:
		z.partyPromoteZones(p, id)
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
	z.partyRemotePB(pt, out) // quem está em outro mapa (party_zones.go)
	return out
}

// sendParty manda o estado do grupo a todos os membros (com a XP do grupo vista por cada um).
func (z *Zone) sendParty(pt *party) {
	base := z.partyPB(pt)
	members := z.partyMembers(pt)
	for _, m := range members {
		st := &pb.PartyState{LeaderId: base.LeaderId, Members: base.Members}
		z.partyXPView(members, m, st)
		z.sendTo(m, &pb.ServerMessage{Msg: &pb.ServerMessage_Party{Party: st}})
	}
}

// partyXPView: o bônus de grupo que o membro teria agora com quem está perto dele (vivo,
// neste mapa, a até partyRange), e o corte pela diferença de nível. A janela mostra pela
// proximidade; no abate, conta também quem ajudou nos últimos 30 s (xpTakers).
func (z *Zone) partyXPView(members []*entity, me *entity, st *pb.PartyState) {
	if me.dead {
		return
	}
	n, top, low := 0, 0, math.MaxInt
	for _, m := range members {
		if !m.dead && dist(m.pos, me.pos) <= partyRange {
			n++
			top, low = max(top, m.level), min(low, m.level)
		}
	}
	st.XpNear = uint32(n)
	st.XpBonusPct = uint32(math.Round((game.PartyXPBonus(n) - 1) * 100))
	if n > 1 {
		st.XpLevelGap = uint32(top - low)
		st.XpGapCutPct = uint32(math.Round((1 - game.PartyGapMult(top-low)) * 100))
	}
}

// syncParties: 1×/s cada grupo recebe vida, mana e posição dos membros.
func (z *Zone) syncParties() {
	if z.mgr != nil {
		z.syncPartiesZones()
		return
	}
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
	z.partySendRemote(pt, &pb.ServerMessage{Msg: &pb.ServerMessage_Chat{Chat: &pb.ChatMessage{
		Channel: pb.ChatChannel_CHAT_CHANNEL_SYSTEM, Text: text,
	}}}, skip)
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
	z.partySendRemote(p.pl.party, msg, 0)
}

// ---------- XP e moedas do grupo ----------
//
// Quem leva um abate (tunnel#151):
//   - O lado que causou mais dano: o grupo soma o dano dos membros, quem está sem grupo
//     conta sozinho. O último golpe não importa (sem "roubo" de abate). O dano é a vida que
//     cada um tirou (monsterState.dmg); sem dano registrado vale o ódio.
//   - Desse lado, contam os membros vivos, nesta zona (online), a até partyRange do
//     monstro e que ajudaram: deram dano neste monstro, ou deram dano, curaram ou buffaram
//     alguém nos últimos game.PartyHelpSeconds (markHelp). Se ninguém do lado conta (todos
//     longe ou mortos), o próximo lado leva.
//   - A XP total sobe com quantos contam (game.PartyXPBonus) e cai com a diferença de
//     nível entre eles (game.PartyGapMult); cada um recebe pelo próprio nível e aplica os
//     próprios extras (vitalidade, recuperação, Duas Sendas, Maestria no máximo).
//   - Quem conta também leva o crédito de missão, contrato, conquista e bestiário; o saque
//     fica reservado para quem deles mais causou dano (o grupo inteiro pode pegar).

// markHelp: o jogador ajudou agora (dano em monstro, buff ou cura em alguém).
func (z *Zone) markHelp(p *entity) {
	if p != nil && p.pl != nil {
		p.pl.helpUntil = z.now + game.PartyHelpSeconds
	}
}

// creditKill: quem leva o abate do monstro (os que contam do lado com mais dano) e o dono
// do saque reservado.
func (z *Zone) creditKill(ms *monsterState, at Vec2) ([]*entity, uint32) {
	score := ms.dmg
	if len(score) == 0 {
		score = ms.hate
	}
	return z.creditScore(score, at)
}

// creditScore: creditKill com o placar dado (dano, ou ódio).
func (z *Zone) creditScore(score map[uint32]float64, at Vec2) ([]*entity, uint32) {
	type side struct {
		party, solo uint32
		total       float64
	}
	var sides []side
	idx := map[[2]uint32]int{}
	for id, v := range score {
		p := z.players[id]
		if p == nil || v <= 0 {
			continue
		}
		k := [2]uint32{0, id}
		if p.pl.party != nil {
			k = [2]uint32{p.pl.party.id, 0}
		}
		i, ok := idx[k]
		if !ok {
			i = len(sides)
			idx[k] = i
			sides = append(sides, side{party: k[0], solo: k[1]})
		}
		sides[i].total += v
	}
	// Maior dano primeiro; empate fica com o menor id (resultado estável).
	slices.SortFunc(sides, func(a, b side) int {
		if a.total != b.total {
			if a.total > b.total {
				return -1
			}
			return 1
		}
		return int(a.party+a.solo) - int(b.party+b.solo)
	})
	for _, s := range sides {
		var cands []*entity
		if s.party != 0 {
			if pt := z.parties[s.party]; pt != nil {
				cands = z.partyMembers(pt)
			}
		} else if p := z.players[s.solo]; p != nil {
			cands = []*entity{p}
		}
		takers := z.xpTakers(cands, at, score)
		if len(takers) == 0 {
			continue
		}
		owner, best := takers[0].id, -1.0
		for _, m := range takers {
			if v := score[m.id]; v > best || (v == best && m.id < owner) {
				owner, best = m.id, v
			}
		}
		return takers, owner
	}
	return nil, 0
}

// xpTakers: dos candidatos (todos nesta zona), os que contam para um abate em at: vivos, a
// até partyRange e que ajudaram (dano neste monstro, ou ajuda nos últimos 30 s).
func (z *Zone) xpTakers(cands []*entity, at Vec2, score map[uint32]float64) []*entity {
	var out []*entity
	for _, m := range cands {
		if !m.dead && dist(m.pos, at) <= partyRange && (score[m.id] > 0 || z.now < m.pl.helpUntil) {
			out = append(out, m)
		}
	}
	return out
}

// raidTakers: no raid boss, de cada lado contam os vivos a até partyRange e no máximo
// raidLevelGap níveis abaixo do mais alto deles (regra própria do raid, sem a ajuda).
func (z *Zone) raidTakers(cands []*entity, at Vec2) []*entity {
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
		if top-m.level <= raidLevelGap {
			out = append(out, m)
		}
	}
	return out
}

// levelGap: a diferença entre o maior e o menor nível.
func levelGap(ps []*entity) int {
	if len(ps) == 0 {
		return 0
	}
	top, low := 0, math.MaxInt
	for _, m := range ps {
		top, low = max(top, m.level), min(low, m.level)
	}
	return top - low
}

// shareMonsterXP divide a XP do monstro entre quem recebe (proporcional ao nível), com o
// corte pela diferença de nível do grupo.
func (z *Zone) shareMonsterXP(dead *entity, takers []*entity) {
	z.shareMonsterXPScaled(dead, takers, game.PartyGapMult(levelGap(takers)))
}

// shareMonsterXPScaled: como shareMonsterXP, com só uma fração da XP (grupo: o corte pela
// diferença de nível; raid: a parte do dano daquele lado).
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
	total *= game.PartyXPBonus(len(takers)) * frac
	for _, m := range takers {
		if xp := int64(math.Round(total * float64(m.level) / float64(sum))); xp > 0 {
			z.tallyMonsterXP(m, xp, z.night, z.xpEventMult(), len(takers) > 1) // telemetria (xptally.go)
			// Com a recuperação e a vitalidade (vitality.go).
			z.grantXP(m, z.monsterXPBonus(m, xp))
		}
		z.burnKarma(m, dead.level)
		z.track(m, "kill", dead.mon.def.ID, int64(dead.level), nil)
		z.questEvent(m, game.GoalKill, dead.mon.def.ID)
		z.achEvent(m, game.AchKill, dead.mon.def.ID)
		z.contractKill(m, dead)
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
