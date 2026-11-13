package zone

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/pb"
)

// fakeClans é um banco de clãs em memória (grava as operações em ordem).
type fakeClans struct {
	taken map[string]bool
	ops   []ClanOp
	seq   int64
}

func (f *fakeClans) CreateClan(_ context.Context, name string, leader int64) (int64, error) {
	if f.taken[strings.ToLower(name)] {
		return 0, ErrClanNameTaken
	}
	f.taken[strings.ToLower(name)] = true
	f.seq++
	return f.seq, nil
}

func (f *fakeClans) ApplyClanOp(_ context.Context, op ClanOp) error {
	f.ops = append(f.ops, op)
	return nil
}

func newZoneWith(cfg Config) *Zone {
	return New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

// drainClan faz o papel do worker: executa os trabalhos na fila e entrega as respostas.
func (z *Zone) drainClan() {
	for {
		select {
		case job := <-z.clanJobs:
			if ev := z.doClanJob(context.Background(), job); ev != nil {
				z.handle(ev)
			}
		default:
			return
		}
	}
}

func clanCmd(z *Zone, p *entity, m *pb.ClanCmd) {
	z.handle(socialEvent{id: p.id, msg: m})
	z.drainClan()
}

func (f *fakePeer) lastClan() *pb.ClanState {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if c := f.msgs[i].GetClan(); c != nil {
			return c
		}
		if w := f.msgs[i].GetWelcome(); w != nil {
			return w.Clan
		}
	}
	return nil
}

// clanZone: líder de nível 10 com moedas para criar.
func clanZone(t *testing.T) (*Zone, *fakeClans, *entity, *fakePeer) {
	z := newCombatZone()
	f := &fakeClans{taken: map[string]bool{}}
	z.SetClanBackend(f)
	lead, pl := named(z, 1, "Lider", "knight", 30, 0)
	atLevel(z, lead, clanCreateLevel)
	lead.pl.inv.gold = clanCreateCost + 100
	return z, f, lead, pl
}

func TestClanCreateCostsGoldAndRefundsTakenName(t *testing.T) {
	z, f, lead, pl := clanZone(t)
	f.taken["lobos"] = true // outro servidor/banco já tem
	clanCmd(z, lead, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_CREATE, Name: "Lobos"})
	if lead.pl.clan != nil || lead.pl.inv.gold != clanCreateCost+100 || !pl.systemContains("devolvidas") {
		t.Fatal("a taken name refunds the gold")
	}
	clanCmd(z, lead, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_CREATE, Name: "Corvos"})
	if lead.pl.clan == nil || lead.pl.clan.name != "Corvos" || lead.pl.inv.gold != 100 {
		t.Fatalf("clan not created: gold %d", lead.pl.inv.gold)
	}
	if z.myRank(lead) != rankLeader {
		t.Fatal("the founder is the leader")
	}
	if st := pl.lastClan(); st == nil || st.Name != "Corvos" || st.MyRank != pb.ClanRank_CLAN_RANK_LEADER {
		t.Fatalf("clan state = %+v", st)
	}
	if !pl.has(func(m *pb.ServerMessage) bool {
		s := m.GetPlayerStatus()
		return s != nil && s.Id == lead.id && s.ClanName == "Corvos"
	}) {
		t.Fatal("everyone should see the clan name above the head")
	}
	// Mesmo nome (sem diferenciar maiúsculas) é recusado na hora, sem ir ao banco.
	other, po := named(z, 2, "Outro", "mage", 31, 0)
	atLevel(z, other, clanCreateLevel)
	other.pl.inv.gold = clanCreateCost
	clanCmd(z, other, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_CREATE, Name: "corvos"})
	if other.pl.clan != nil || other.pl.inv.gold != clanCreateCost || !po.systemContains("Já existe") {
		t.Fatal("duplicate names are refused")
	}
}

func TestClanCreateRequirements(t *testing.T) {
	z, _, lead, pl := clanZone(t)
	atLevel(z, lead, clanCreateLevel-1)
	clanCmd(z, lead, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_CREATE, Name: "Corvos"})
	if lead.pl.clan != nil || !pl.systemContains("nível") {
		t.Fatal("creating needs the minimum level")
	}
	atLevel(z, lead, clanCreateLevel)
	lead.pl.inv.gold = 10
	clanCmd(z, lead, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_CREATE, Name: "Corvos"})
	if lead.pl.clan != nil {
		t.Fatal("creating needs the gold")
	}
	lead.pl.inv.gold = clanCreateCost
	clanCmd(z, lead, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_CREATE, Name: "a b"})
	if lead.pl.clan != nil {
		t.Fatal("invalid names are refused")
	}
}

func TestClanInviteRanksKickAndLeave(t *testing.T) {
	z, f, lead, _ := clanZone(t)
	clanCmd(z, lead, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_CREATE, Name: "Corvos"})
	c := lead.pl.clan
	mem, pm := named(z, 2, "Membro", "mage", 31, 0)
	off, po := named(z, 3, "Oficial", "rogue", 32, 0)
	out, pout := named(z, 4, "Fora", "barbarian", 33, 0)

	for _, p := range []struct {
		e    *entity
		peer *fakePeer
	}{{mem, pm}, {off, po}} {
		clanCmd(z, lead, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_INVITE, Name: p.e.name})
		inv := p.peer.lastInvite()
		if inv == nil || inv.Kind != pb.InviteKind_INVITE_KIND_CLAN || inv.Extra != "Corvos" {
			t.Fatalf("clan invite = %+v", inv)
		}
		z.handle(socialEvent{id: p.e.id, msg: &pb.InviteReply{InviteId: inv.Id, Accept: true}})
		z.drainClan()
	}
	if mem.pl.clan != c || off.pl.clan != c || len(c.members) != 3 {
		t.Fatal("accepted members join the clan")
	}
	if !z.allies(mem, off) {
		t.Fatal("clan members are allies")
	}

	// Membro comum não convida; oficial convida e expulsa membro, mas não oficial/líder.
	clanCmd(z, mem, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_INVITE, Name: "Fora"})
	if pout.lastInvite() != nil {
		t.Fatal("members cannot invite")
	}
	clanCmd(z, lead, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_RANK, TargetId: off.id, Rank: pb.ClanRank_CLAN_RANK_OFFICER})
	if z.myRank(off) != rankOfficer {
		t.Fatal("the leader promotes")
	}
	clanCmd(z, off, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_KICK, TargetId: lead.id})
	if z.myRank(lead) != rankLeader {
		t.Fatal("officers cannot kick the leader")
	}
	clanCmd(z, off, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_NOTICE, Text: "Caçada às 20h"})
	if c.notice != "Caçada às 20h" {
		t.Fatal("officers edit the notice")
	}
	clanCmd(z, off, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_KICK, TargetId: mem.id})
	if mem.pl.clan != nil || !pm.systemContains("expulso") {
		t.Fatal("officers kick members")
	}

	// Quem sai espera 24 h para entrar em outro clã.
	before := pm.lastInvite().Id
	clanCmd(z, lead, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_INVITE, Name: "Membro"})
	if pm.lastInvite().Id != before {
		t.Fatal("whoever left must wait before joining a clan")
	}
	if st := pm.lastClan(); st == nil || st.Id != 0 || st.JoinWaitMs == 0 {
		t.Fatalf("the kicked player sees the wait: %+v", st)
	}
	_ = out

	// O líder não sai sem passar a liderança; passa e sai.
	clanCmd(z, lead, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_LEAVE})
	if lead.pl.clan == nil {
		t.Fatal("the leader must pass leadership before leaving")
	}
	clanCmd(z, lead, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_LEADER, TargetId: off.id})
	if c.leader != int64(off.id) || z.myRank(lead) != rankOfficer {
		t.Fatal("passing leadership makes the old leader an officer")
	}
	clanCmd(z, lead, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_LEAVE})
	if lead.pl.clan != nil || len(c.members) != 1 {
		t.Fatal("after passing leadership the old leader can leave")
	}

	// O banco recebeu as mudanças em ordem.
	kinds := []ClanOpKind{}
	for _, op := range f.ops {
		kinds = append(kinds, op.Kind)
	}
	want := []ClanOpKind{ClanOpJoin, ClanOpJoin, ClanOpRank, ClanOpNotice, ClanOpLeave, ClanOpLeader, ClanOpLeave}
	if len(kinds) != len(want) {
		t.Fatalf("ops = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("ops = %v, want %v", kinds, want)
		}
	}
}

func TestClanWaitAfterLeaving(t *testing.T) {
	z, _, lead, _ := clanZone(t)
	clanCmd(z, lead, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_CREATE, Name: "Corvos"})
	p, pp := named(z, 2, "Recente", "mage", 31, 0)
	p.pl.clanLeftAt = z.cfg.Clock().Add(-time.Hour) // saiu de outro clã há 1 h
	clanCmd(z, lead, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_INVITE, Name: "Recente"})
	if pp.lastInvite() != nil {
		t.Fatal("players who left a clan less than 24 h ago cannot be invited")
	}
	p.pl.clanLeftAt = z.cfg.Clock().Add(-25 * time.Hour)
	clanCmd(z, lead, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_INVITE, Name: "Recente"})
	if pp.lastInvite() == nil {
		t.Fatal("after 24 h the invite is allowed")
	}
}

func TestClanChatDisbandAndRelogin(t *testing.T) {
	z, _, lead, _ := clanZone(t)
	clanCmd(z, lead, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_CREATE, Name: "Corvos"})
	mem, pm := named(z, 2, "Membro", "mage", 31, 0)
	_, pout := named(z, 3, "Fora", "rogue", 32, 0)
	clanCmd(z, lead, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_INVITE, Name: "Membro"})
	z.handle(socialEvent{id: mem.id, msg: &pb.InviteReply{InviteId: pm.lastInvite().Id, Accept: true}})
	z.drainClan()

	z.handle(chatEvent{id: lead.id, text: "reunião", channel: pb.ChatChannel_CHAT_CHANNEL_CLAN})
	got := func(f *fakePeer) bool {
		return f.has(func(m *pb.ServerMessage) bool {
			ch := m.GetChat()
			return ch != nil && ch.Channel == pb.ChatChannel_CHAT_CHANNEL_CLAN && ch.Text == "reunião"
		})
	}
	if !got(pm) || got(pout) {
		t.Fatal("clan chat reaches only clan members")
	}

	// Sair e voltar: continua no clã, e os colegas veem online/offline.
	z.removePlayer(mem.id)
	st := z.clanPB(lead)
	for _, m := range st.Members {
		if m.CharacterId == mem.id && m.Online {
			t.Fatal("logged out members show as offline")
		}
	}
	mem2, pm2 := named(z, 2, "Membro", "mage", 31, 0)
	if mem2.pl.clan == nil || pm2.lastClan().Name != "Corvos" {
		t.Fatal("members stay in the clan after relogging")
	}

	clanCmd(z, lead, &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_DISBAND})
	if lead.pl.clan != nil || mem2.pl.clan != nil || len(z.clans) != 0 {
		t.Fatal("disbanding removes everyone")
	}
	if !pm2.systemContains("desfeito") {
		t.Fatal("members should be told")
	}
}

func TestClansLoadedAtStart(t *testing.T) {
	cfg := testConfig()
	cfg.Spawns = nil
	cfg.Clans = []ClanData{{ID: 9, Name: "Antigos", LeaderID: 5, Members: []ClanMemberData{
		{CharacterID: 5, Name: "Velho", Class: "knight", Level: 15, Rank: rankLeader},
		{CharacterID: 6, Name: "Novo", Class: "mage", Level: 8, Rank: rankMember},
	}}}
	z := newZoneWith(cfg)
	p, peer := named(z, 6, "Novo", "mage", 30, 0)
	if p.pl.clan == nil || p.pl.clan.id != 9 {
		t.Fatal("saved clan membership is restored on join")
	}
	if st := peer.lastClan(); st == nil || len(st.Members) != 2 || st.MyRank != pb.ClanRank_CLAN_RANK_MEMBER {
		t.Fatalf("welcome clan = %+v", st)
	}
	if z.entityPB(p).ClanName != "Antigos" {
		t.Fatal("others see the clan name")
	}
}
