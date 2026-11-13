package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

func TestKarmaEnchantAndStorageRoundTrip(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "pk", "pk")
	a, err := s.CreateCharacter(ctx, acc, "Rubro", "rogue", game.DefaultLook("rogue"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.CreateCharacter(ctx, acc, "Alt", "mage", game.DefaultLook("mage"))

	items := []game.ItemRecord{{Equipped: true, Slot: game.SlotWeapon, ItemID: "daggers_good", Count: 1, Enchant: 7}}
	stored := []game.ItemRecord{
		{Slot: 0, ItemID: "sword_rare", Count: 1, Enchant: 4},
		{Slot: 5, ItemID: "bone_old", Count: 30},
	}
	st := &store.StorageState{AccountID: acc, Gold: 1234, Items: stored, Seq: 1794527065649062000}
	if err := s.SaveStates(ctx, []store.CharacterState{{
		CharacterID: a.ID, Level: 9, XP: 900, HP: -1, MP: -1, Gold: 10, Items: items,
		Karma: 360, PKs: 3, PvPKills: 2, Seq: 1794527065649062000, Storage: st,
	}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.CharacterForAccount(ctx, acc, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Karma != 360 || got.PKs != 3 || got.PvPKills != 2 {
		t.Fatalf("pvp = %d %d %d", got.Karma, got.PKs, got.PvPKills)
	}
	if len(got.Items) != 1 || got.Items[0] != items[0] {
		t.Fatalf("items = %+v", got.Items)
	}
	if got.Storage.Gold != 1234 || len(got.Storage.Items) != 2 || got.Storage.Items[0] != stored[0] || got.Storage.Items[1] != stored[1] {
		t.Fatalf("storage = %+v", got.Storage)
	}

	// O outro personagem da conta vê o mesmo armazém.
	alt, _ := s.CharacterForAccount(ctx, acc, b.ID)
	if alt.Storage.Gold != 1234 || len(alt.Storage.Items) != 2 {
		t.Fatalf("shared storage = %+v", alt.Storage)
	}

	// Um armazém mais velho (Seq menor) não desfaz o mais novo, mesmo vindo de outro personagem.
	old := &store.StorageState{AccountID: acc, Gold: 1, Seq: 1794527065649061000}
	if err := s.SaveStates(ctx, []store.CharacterState{{CharacterID: b.ID, Level: 1, HP: -1, MP: -1,
		Seq: 1794527065649063000, Storage: old}}); err != nil {
		t.Fatal(err)
	}
	alt, _ = s.CharacterForAccount(ctx, acc, b.ID)
	if alt.Storage.Gold != 1234 || len(alt.Storage.Items) != 2 {
		t.Fatalf("old storage overwrote the newer one: %+v", alt.Storage)
	}
}

func TestClans(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "cl", "cl")
	acc2, _ := s.LoginIdentity(ctx, "dev", "cl2", "cl2")
	lead, _ := s.CreateCharacter(ctx, acc, "Lider", "knight", game.DefaultLook("knight"))
	mem, _ := s.CreateCharacter(ctx, acc2, "Membro", "mage", game.DefaultLook("mage"))
	other, _ := s.CreateCharacter(ctx, acc2, "Outro", "rogue", game.DefaultLook("rogue"))

	id, err := s.CreateClan(ctx, "Lobos", lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateClan(ctx, "lobos", other.ID); !errors.Is(err, store.ErrNameTaken) {
		t.Fatalf("duplicate clan name: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	ops := []store.ClanOp{
		{Kind: store.ClanOpJoin, ClanID: id, CharacterID: mem.ID, Rank: store.ClanRankMember},
		{Kind: store.ClanOpRank, CharacterID: mem.ID, Rank: store.ClanRankOfficer},
		{Kind: store.ClanOpNotice, ClanID: id, Text: "Caçada às 20h"},
		{Kind: store.ClanOpLeader, ClanID: id, CharacterID: mem.ID},
		{Kind: store.ClanOpJoin, ClanID: id, CharacterID: other.ID},
		{Kind: store.ClanOpLeave, CharacterID: other.ID, LeftAt: now},
	}
	for _, op := range ops {
		if err := s.ApplyClanOp(ctx, op); err != nil {
			t.Fatalf("op %+v: %v", op, err)
		}
	}
	clans, err := s.LoadClans(ctx)
	if err != nil || len(clans) != 1 {
		t.Fatalf("clans = %+v, %v", clans, err)
	}
	c := clans[0]
	if c.Name != "Lobos" || c.LeaderID != mem.ID || c.Notice != "Caçada às 20h" || len(c.Members) != 2 {
		t.Fatalf("clan = %+v", c)
	}
	ranks := map[int64]int{}
	for _, m := range c.Members {
		ranks[m.CharacterID] = m.Rank
	}
	if ranks[mem.ID] != store.ClanRankLeader || ranks[lead.ID] != store.ClanRankOfficer {
		t.Fatalf("ranks = %+v", ranks)
	}
	left, _ := s.CharacterForAccount(ctx, acc2, other.ID)
	if left.ClanLeftAt == nil || !left.ClanLeftAt.Equal(now) {
		t.Fatalf("clan_left_at = %v", left.ClanLeftAt)
	}

	if err := s.ApplyClanOp(ctx, store.ClanOp{Kind: store.ClanOpDisband, ClanID: id}); err != nil {
		t.Fatal(err)
	}
	if clans, _ := s.LoadClans(ctx); len(clans) != 0 {
		t.Fatalf("disband left %+v", clans)
	}
	// O nome fica livre de novo.
	if _, err := s.CreateClan(ctx, "Lobos", other.ID); err != nil {
		t.Fatal(err)
	}
}
