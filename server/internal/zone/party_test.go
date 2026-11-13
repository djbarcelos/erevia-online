package zone

import (
	"fmt"
	"math"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// named entra com um nome próprio (os auxiliares antigos repetem o nome por classe).
func named(z *Zone, id uint32, name, class string, x, zz float64) (*entity, *fakePeer) {
	peer := &fakePeer{}
	z.handle(joinEvent{info: JoinInfo{ID: id, Name: name, Class: class, X: x, Z: zz, Items: game.StarterKit(class), Peer: peer}})
	return z.players[id], peer
}

func (f *fakePeer) lastInvite() *pb.Invite {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if inv := f.msgs[i].GetInvite(); inv != nil {
			return inv
		}
	}
	return nil
}

func (f *fakePeer) lastParty() *pb.PartyState {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if pt := f.msgs[i].GetParty(); pt != nil {
			return pt
		}
	}
	return nil
}

func partyInviteByName(z *Zone, from *entity, name string) {
	z.handle(socialEvent{id: from.id, msg: &pb.PartyCmd{Kind: pb.PartyCmdKind_PARTY_CMD_INVITE, Name: name}})
}

func reply(z *Zone, p *entity, peer *fakePeer, accept bool) {
	inv := peer.lastInvite()
	if inv == nil {
		panic("no invite")
	}
	z.handle(socialEvent{id: p.id, msg: &pb.InviteReply{InviteId: inv.Id, Accept: accept}})
}

// joinParty: from convida e to aceita.
func joinParty(z *Zone, from, to *entity, toPeer *fakePeer) {
	partyInviteByName(z, from, to.name)
	reply(z, to, toPeer, true)
}

func TestPartyInviteAcceptAndLeadership(t *testing.T) {
	z := newCombatZone()
	a, pa := named(z, 1, "Ana", "knight", 30, 0)
	b, pb_ := named(z, 2, "Beto", "mage", 31, 0)
	c, pc := named(z, 3, "Caio", "rogue", 32, 0)

	partyInviteByName(z, a, "beto")
	inv := pb_.lastInvite()
	if inv == nil || inv.FromName != "Ana" || inv.Kind != pb.InviteKind_INVITE_KIND_PARTY {
		t.Fatalf("invite = %+v", inv)
	}
	reply(z, b, pb_, true)
	if a.pl.party == nil || a.pl.party != b.pl.party || a.pl.party.leader != a.id {
		t.Fatal("accepting forms a party led by the inviter")
	}
	if st := pa.lastParty(); st == nil || len(st.Members) != 2 || st.LeaderId != a.id {
		t.Fatalf("party state = %+v", st)
	}
	if !z.allies(a, b) {
		t.Fatal("party members are allies")
	}

	// Só o líder convida.
	partyInviteByName(z, b, "Caio")
	if pc.lastInvite() != nil || !pb_.systemContains("Só o líder") {
		t.Fatal("only the leader can invite")
	}
	// Recusar avisa quem convidou.
	partyInviteByName(z, a, "Caio")
	reply(z, c, pc, false)
	if !pa.systemContains("Caio recusou") || c.pl.party != nil {
		t.Fatal("declining should tell the inviter")
	}
	// Convite vence em 30 s.
	partyInviteByName(z, a, "Caio")
	z.tickFor(inviteTTL + 1)
	pc.reset()
	z.handle(socialEvent{id: c.id, msg: &pb.InviteReply{InviteId: z.inviteSeq, Accept: true}})
	if c.pl.party != nil {
		t.Fatal("expired invites cannot be accepted")
	}
	joinParty(z, a, c, pc)
	if len(a.pl.party.members) != 3 {
		t.Fatal("c should be in the party")
	}

	// Líder sai: a liderança passa para o próximo.
	pt := a.pl.party
	z.handle(socialEvent{id: a.id, msg: &pb.PartyCmd{Kind: pb.PartyCmdKind_PARTY_CMD_LEAVE}})
	if a.pl.party != nil || pt.leader != b.id || len(pt.members) != 2 {
		t.Fatalf("leader left: leader=%d members=%v", pt.leader, pt.members)
	}
	if st := pa.lastParty(); st == nil || len(st.Members) != 0 {
		t.Fatal("whoever leaves gets an empty party state")
	}
	// Só o líder expulsa; sobrando 1, o grupo acaba.
	z.handle(socialEvent{id: c.id, msg: &pb.PartyCmd{Kind: pb.PartyCmdKind_PARTY_CMD_KICK, TargetId: b.id}})
	if b.pl.party == nil {
		t.Fatal("only the leader can kick")
	}
	z.handle(socialEvent{id: b.id, msg: &pb.PartyCmd{Kind: pb.PartyCmdKind_PARTY_CMD_KICK, TargetId: c.id}})
	if b.pl.party != nil || c.pl.party != nil || len(z.parties) != 0 {
		t.Fatal("a party of one ends")
	}
}

func TestPartyLimitAndLogout(t *testing.T) {
	z := newCombatZone()
	lead, _ := named(z, 1, "Lider", "knight", 30, 0)
	for i := 2; i <= partyMax; i++ {
		p, peer := named(z, uint32(i), fmt.Sprintf("Mem%d", i), "mage", 31, 0)
		joinParty(z, lead, p, peer)
	}
	extra, pe := named(z, uint32(partyMax+1), "Sobra", "mage", 31, 0)
	partyInviteByName(z, lead, extra.name)
	if pe.lastInvite() != nil {
		t.Fatal("a full party cannot invite")
	}
	if n := len(lead.pl.party.members); n != partyMax {
		t.Fatalf("party has %d members, max %d", n, partyMax)
	}
	if z.players[uint32(partyMax+1)].pl.party != nil {
		t.Fatal("the 7th player must not join")
	}

	// Sair do jogo tira do grupo.
	z.removePlayer(2)
	if lead.pl.party.has(2) {
		t.Fatal("logging out leaves the party")
	}
}

func TestPartySharesMonsterXP(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 8, 40, 0))
	m := z.onlyMonster()
	a, _ := named(z, 1, "Ana", "knight", 38, 0)
	b, _ := named(z, 2, "Beto", "mage", 39, 1)
	far, _ := named(z, 3, "Longe", "rogue", 90, 0)
	atLevel(z, a, 8)
	atLevel(z, b, 6)
	atLevel(z, far, 8)
	makeParty(z, a, b, far)
	xa, xb, xf := a.pl.xp, b.pl.xp, far.pl.xp

	m.mon.hate[a.id] = 50
	z.damage(a, m, m.hp+10, nil)

	total := float64(game.MonsterXP(m.mon.def, 8, 8)) * partyBonus[1]
	wantA := int64(math.Round(total * 8 / 14))
	wantB := int64(math.Round(total * 6 / 14))
	if a.pl.xp-xa != wantA || b.pl.xp-xb != wantB {
		t.Fatalf("xp a=%d (want %d) b=%d (want %d)", a.pl.xp-xa, wantA, b.pl.xp-xb, wantB)
	}
	if far.pl.xp != xf {
		t.Fatal("members farther than 30 m get no XP")
	}
}

func TestPartyLevelGapAndTopDamageGroup(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 8, 40, 0), fixed("skeleton_warrior", 8, 40, 6))
	ms := []*entity{}
	for _, m := range z.monsters {
		ms = append(ms, m)
	}
	hi, _ := named(z, 1, "Alto", "knight", 38, 2)
	lo, _ := named(z, 2, "Baixo", "mage", 39, 2)
	atLevel(z, hi, 16)
	atLevel(z, lo, 6) // 10 abaixo: fica sem XP
	makeParty(z, hi, lo)
	xl := lo.pl.xp
	ms[0].mon.hate[hi.id] = 10
	z.damage(hi, ms[0], ms[0].hp+10, nil)
	if lo.pl.xp != xl {
		t.Fatal("members more than 9 levels below the highest get no XP")
	}

	// Quem causou mais dano leva a XP, mesmo sem dar o último golpe.
	solo, _ := named(z, 3, "Solo", "rogue", 39, 6)
	thief, _ := named(z, 4, "Ladrao", "rogue", 39, 7)
	atLevel(z, solo, 8)
	atLevel(z, thief, 8)
	xs, xt := solo.pl.xp, thief.pl.xp
	ms[1].mon.hate[solo.id] = 500
	ms[1].mon.hate[thief.id] = 5
	z.damage(thief, ms[1], ms[1].hp+10, nil)
	if solo.pl.xp == xs || thief.pl.xp != xt {
		t.Fatal("XP goes to whoever dealt the most damage, not the last hit")
	}
}

func TestPartyLootReservationAndGoldSplit(t *testing.T) {
	z := newCombatZone()
	a, _ := named(z, 1, "Ana", "knight", 30, 0)
	b, pb_ := named(z, 2, "Beto", "mage", 30.5, 0)
	c, pc := named(z, 3, "Caio", "rogue", 30.5, 0.5)
	makeParty(z, a, b)

	g := z.placeItem(Vec2{X: 30.5, Z: 0.3}, item("bone_old"), 3, 0, a.id, reservedFor)
	z.handle(itemEvent{id: c.id, msg: &pb.PickUp{Id: g.id}})
	z.tickFor(1)
	if z.ground[g.id] == nil || !pc.systemContains("reservado") {
		t.Fatal("strangers cannot take reserved loot")
	}
	z.handle(itemEvent{id: b.id, msg: &pb.PickUp{Id: g.id}})
	z.tickFor(1)
	if z.ground[g.id] != nil || b.pl.inv.countOf("bone_old") != 3 {
		t.Fatal("party members can take the leader's reserved loot")
	}

	ga, gb := a.pl.inv.gold, b.pl.inv.gold
	gold := z.placeItem(Vec2{X: 30.2, Z: 0}, item(game.GoldID), 11, 0, a.id, reservedFor)
	z.handle(itemEvent{id: a.id, msg: &pb.PickUp{Id: gold.id}})
	z.tickFor(1)
	if a.pl.inv.gold-ga != 6 || b.pl.inv.gold-gb != 5 {
		t.Fatalf("gold split a=+%d b=+%d (want 6/5)", a.pl.inv.gold-ga, b.pl.inv.gold-gb)
	}
	if !pb_.systemContains("+5 moedas (grupo)") {
		t.Fatal("members should be told about their share")
	}
}

func TestPartyChat(t *testing.T) {
	z := newCombatZone()
	a, _ := named(z, 1, "Ana", "knight", 30, 0)
	b, pb_ := named(z, 2, "Beto", "mage", 31, 0)
	c, pc := named(z, 3, "Caio", "rogue", 32, 0)
	makeParty(z, a, b)
	z.handle(chatEvent{id: a.id, text: "puxando", channel: pb.ChatChannel_CHAT_CHANNEL_PARTY})
	got := func(f *fakePeer) bool {
		return f.has(func(m *pb.ServerMessage) bool {
			ch := m.GetChat()
			return ch != nil && ch.Channel == pb.ChatChannel_CHAT_CHANNEL_PARTY && ch.Text == "puxando"
		})
	}
	if !got(pb_) || got(pc) {
		t.Fatal("party chat reaches only party members")
	}
	z.handle(chatEvent{id: c.id, text: "oi", channel: pb.ChatChannel_CHAT_CHANNEL_PARTY})
	if !pc.systemContains("não está em um grupo") {
		t.Fatal("party chat without a party should explain")
	}
}
