package zone

import (
	"testing"

	"erevia/server/internal/pb"
)

func (f *fakePeer) kicked() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range f.msgs {
		if k := m.GetKicked(); k != nil {
			return k.Reason
		}
	}
	return ""
}

// Conta apagada com personagem online: cai na hora, o clã passa para quem o banco escolheu,
// o clã vazio é desfeito e um Join atrasado do personagem apagado é recusado.
func TestAccountDeletedKicksAndFixesClans(t *testing.T) {
	cfg := testConfig()
	cfg.Spawns = nil
	cfg.Clans = []ClanData{
		{ID: 1, Name: "Lobos", LeaderID: 10, Members: []ClanMemberData{
			{CharacterID: 10, Name: "Alfa", Rank: rankLeader},
			{CharacterID: 20, Name: "Beto", Rank: rankMember},
			{CharacterID: 21, Name: "Caio", Rank: rankOfficer},
		}},
		{ID: 2, Name: "Sos", LeaderID: 11, Members: []ClanMemberData{{CharacterID: 11, Name: "Gama", Rank: rankLeader}}},
		{ID: 3, Name: "Corvos", LeaderID: 22, Members: []ClanMemberData{
			{CharacterID: 22, Name: "Dani", Rank: rankLeader},
			{CharacterID: 12, Name: "Ema", Rank: rankMember},
		}},
	}
	z := newZoneWith(cfg)
	f := &fakeClans{taken: map[string]bool{}}
	z.SetClanBackend(f)

	alfa := join(z, 10, "Alfa") // da conta apagada, online
	beto := join(z, 20, "Beto")
	dani := join(z, 22, "Dani")
	z.players[10].pl.accountID = 7
	z.storages[7] = newStorage(7, nil)

	z.handle(accountGoneEvent{d: AccountGone{
		AccountID:  7,
		Characters: []int64{10, 11, 12},
		Clans:      []ClanSuccession{{ClanID: 1, LeaderID: 21}, {ClanID: 2, LeaderID: 0}},
	}})
	z.drainClan()

	if alfa.kicked() != accountGoneReason || alfa.closed == "" {
		t.Fatalf("alfa kicked=%q closed=%q", alfa.kicked(), alfa.closed)
	}
	if z.players[10] != nil {
		t.Fatal("alfa still in the zone")
	}
	if _, ok := z.recent[10]; ok {
		t.Fatal("zone still remembers alfa's state")
	}
	if z.storages[7] != nil {
		t.Fatal("zone still remembers the account storage")
	}

	// Lobos: Caio (o oficial, escolhido pelo banco) assume; Beto vê.
	lobos := z.clans[1]
	if lobos == nil || lobos.leader != 21 || lobos.members[21].rank != rankLeader || len(lobos.members) != 2 || z.clanOf[10] != nil {
		t.Fatalf("lobos = %+v", lobos)
	}
	if c := beto.lastClan(); c == nil || len(c.Members) != 2 {
		t.Fatalf("beto clan = %+v", c)
	}
	// Sos: desfeito. Corvos: só a Ema sai.
	if z.clans[2] != nil || z.clanOf[11] != nil {
		t.Fatal("empty clan not disbanded")
	}
	if c := z.clans[3]; c == nil || c.leader != 22 || len(c.members) != 1 || z.clanOf[12] != nil {
		t.Fatalf("corvos = %+v", c)
	}
	if c := dani.lastClan(); c == nil || len(c.Members) != 1 {
		t.Fatalf("dani clan = %+v", c)
	}
	want := map[ClanOpKind]int64{ClanOpLeader: 1, ClanOpDisband: 2}
	if len(f.ops) != 2 {
		t.Fatalf("clan ops = %+v", f.ops)
	}
	for _, op := range f.ops {
		if want[op.Kind] != op.ClanID {
			t.Fatalf("clan ops = %+v", f.ops)
		}
	}

	// O personagem carregado antes de apagar tenta entrar depois: é recusado.
	late := join(z, 10, "Alfa")
	if late.kicked() != accountGoneReason || late.closed == "" || z.players[10] != nil {
		t.Fatalf("late join kicked=%q closed=%q", late.kicked(), late.closed)
	}
	// Os outros continuam jogando.
	if z.players[20] == nil || z.players[22] == nil {
		t.Fatal("other players were removed")
	}
}

// Se o escolhido pelo banco já saiu do clã na memória, a zona escolhe outro e grava.
func TestAccountDeletedClanSuccessorFallback(t *testing.T) {
	cfg := testConfig()
	cfg.Spawns = nil
	cfg.Clans = []ClanData{{ID: 1, Name: "Lobos", LeaderID: 10, Members: []ClanMemberData{
		{CharacterID: 10, Name: "Alfa", Rank: rankLeader},
		{CharacterID: 30, Name: "Novo", Rank: rankMember},
		{CharacterID: 20, Name: "Velho", Rank: rankMember},
	}}}
	z := newZoneWith(cfg)
	f := &fakeClans{taken: map[string]bool{}}
	z.SetClanBackend(f)

	z.handle(accountGoneEvent{d: AccountGone{AccountID: 7, Characters: []int64{10},
		Clans: []ClanSuccession{{ClanID: 1, LeaderID: 99}}}})
	z.drainClan()
	if c := z.clans[1]; c == nil || c.leader != 20 {
		t.Fatalf("lobos = %+v", c)
	}
	if len(f.ops) != 1 || f.ops[0].Kind != ClanOpLeader || f.ops[0].CharacterID != 20 {
		t.Fatalf("ops = %+v", f.ops)
	}
}

// A conta foi apagada enquanto o banco criava o clã dela: o clã novo é desfeito.
func TestAccountDeletedWhileCreatingClan(t *testing.T) {
	z, f, lead, _ := clanZone(t)
	z.handle(socialEvent{id: lead.id, msg: &pb.ClanCmd{Kind: pb.ClanCmdKind_CLAN_CMD_CREATE, Name: "Fantasma"}})
	z.handle(accountGoneEvent{d: AccountGone{AccountID: 1, Characters: []int64{int64(lead.id)}}})
	z.drainClan()
	if len(z.clans) != 0 || z.clanOf[int64(lead.id)] != nil {
		t.Fatalf("clans = %+v", z.clans)
	}
	if len(f.ops) != 1 || f.ops[0].Kind != ClanOpDisband {
		t.Fatalf("ops = %+v", f.ops)
	}
}
