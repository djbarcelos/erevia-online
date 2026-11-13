package store_test

import (
	"context"
	"errors"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

func TestLoginIdentityAndSessions(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()

	a1, err := s.LoginIdentity(ctx, "google", "sub-1", "Ana")
	if err != nil {
		t.Fatal(err)
	}
	a2, err := s.LoginIdentity(ctx, "google", "sub-1", "Ana Maria")
	if err != nil {
		t.Fatal(err)
	}
	if a1 != a2 {
		t.Fatalf("same identity should map to same account: %d vs %d", a1, a2)
	}
	other, _ := s.LoginIdentity(ctx, "discord", "sub-1", "Outro")
	if other == a1 {
		t.Fatal("different provider must be a different account")
	}
	if name, _ := s.DisplayName(ctx, a1); name != "Ana Maria" {
		t.Fatalf("display name = %q", name)
	}

	token, err := s.CreateSession(ctx, a1)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.AccountBySession(ctx, token)
	if err != nil || got != a1 {
		t.Fatalf("AccountBySession = %d, %v", got, err)
	}
	if _, err := s.AccountBySession(ctx, "invalido"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown token: %v", err)
	}
	if err := s.DeleteSession(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AccountBySession(ctx, token); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted session should be gone: %v", err)
	}
}

func TestCharacters(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "a", "a")
	acc2, _ := s.LoginIdentity(ctx, "dev", "b", "b")

	c, err := s.CreateCharacter(ctx, acc, "Aria", "mage")
	if err != nil {
		t.Fatal(err)
	}
	if c.ID == 0 || c.Name != "Aria" || c.Class != "mage" {
		t.Fatalf("bad character %+v", c)
	}

	if _, err := s.CreateCharacter(ctx, acc2, "aria", "knight"); !errors.Is(err, store.ErrNameTaken) {
		t.Fatalf("case-insensitive duplicate name: %v", err)
	}

	s.CreateCharacter(ctx, acc, "Beto", "rogue")
	s.CreateCharacter(ctx, acc, "Caio", "knight")
	if _, err := s.CreateCharacter(ctx, acc, "Dani", "barbarian"); !errors.Is(err, store.ErrTooManyCharacters) {
		t.Fatalf("4th character: %v", err)
	}

	list, err := s.ListCharacters(ctx, acc)
	if err != nil || len(list) != 3 {
		t.Fatalf("list = %v, %v", list, err)
	}

	if _, err := s.CharacterForAccount(ctx, acc2, c.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("other account must not load character: %v", err)
	}

	if c.Level != 1 || c.XP != 0 || c.HP != -1 || c.MP != -1 {
		t.Fatalf("new character should start at level 1 with full hp/mp: %+v", c)
	}
	if err := s.SaveStates(ctx, []store.CharacterState{{CharacterID: c.ID, X: 12.5, Z: -7.25, Level: 4, XP: 1234, HP: 50, MP: -1}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.CharacterForAccount(ctx, acc, c.ID)
	if err != nil || got.X != 12.5 || got.Z != -7.25 || got.Level != 4 || got.XP != 1234 || got.HP != 50 || got.MP != -1 {
		t.Fatalf("saved state = %+v, %v", got, err)
	}
}

func TestStarterKitItemsAndSaveOrder(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "k", "k")
	c, err := s.CreateCharacter(ctx, acc, "Kit", "knight")
	if err != nil {
		t.Fatal(err)
	}

	// Primeiro carregamento: kit inicial e moedas, uma vez só.
	got, err := s.CharacterForAccount(ctx, acc, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Gold != game.StarterGold || len(got.Items) != len(game.StarterKit("knight")) {
		t.Fatalf("kit: gold %d items %+v", got.Gold, got.Items)
	}
	again, _ := s.CharacterForAccount(ctx, acc, c.ID)
	if again.Gold != game.StarterGold || len(again.Items) != len(got.Items) {
		t.Fatalf("kit given twice: %+v", again)
	}

	items := []game.ItemRecord{
		{Equipped: true, Slot: game.SlotWeapon, ItemID: "sword_good", Count: 1},
		{Slot: 3, ItemID: "bone_old", Count: 42},
	}
	newer := store.CharacterState{CharacterID: c.ID, Level: 2, XP: 200, HP: -1, MP: -1, Gold: 777, Items: items, Seq: 1794527065649061000} // nanossegundos, como a zona usa
	older := store.CharacterState{CharacterID: c.ID, Level: 1, XP: 100, HP: -1, MP: -1, Gold: 1, Seq: 1794527065649060000}
	if err := s.SaveStates(ctx, []store.CharacterState{newer}); err != nil {
		t.Fatal(err)
	}
	// Um salvamento atrasado (Seq menor) não pode desfazer o mais novo.
	if err := s.SaveStates(ctx, []store.CharacterState{older}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.CharacterForAccount(ctx, acc, c.ID)
	if got.Gold != 777 || got.XP != 200 || len(got.Items) != 2 {
		t.Fatalf("after saves: %+v", got)
	}
	if got.Items[0] != items[0] || got.Items[1] != items[1] {
		t.Fatalf("items = %+v", got.Items)
	}
}

func TestProgressionRoundTrip(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "prog", "prog")
	c, err := s.CreateCharacter(ctx, acc, "Arvore", "rogue")
	if err != nil {
		t.Fatal(err)
	}
	// Personagem novo: nada distribuído, árvore e barra vazias (a zona preenche o padrão).
	got, err := s.CharacterForAccount(ctx, acc, c.ID)
	if err != nil || got.Attrs.Sum() != 0 || len(got.Ranks) != 0 || len(got.Hotbar) != 0 {
		t.Fatalf("new character progression = %+v / %v", got, err)
	}
	st := store.CharacterState{
		CharacterID: c.ID, Level: 9, XP: 5000, HP: -1, MP: -1,
		Attrs:  game.Attrs{Str: 2, Dex: 14, Con: 4, Wis: 1, Spi: 3},
		Ranks:  map[uint32]int{game.SkillBackstab: 3, game.SkillWeakSpot: 2, game.SkillShadowStep: 1},
		Hotbar: []string{"skill:7", "skill:8", "item:potion_hp_s", "", "", "", "", "", "", ""},
		Seq:    1,
	}
	if err := s.SaveStates(ctx, []store.CharacterState{st}); err != nil {
		t.Fatal(err)
	}
	got, err = s.CharacterForAccount(ctx, acc, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Attrs != st.Attrs {
		t.Errorf("attrs = %+v", got.Attrs)
	}
	if len(got.Ranks) != 3 || got.Ranks[game.SkillBackstab] != 3 || got.Ranks[game.SkillWeakSpot] != 2 {
		t.Errorf("ranks = %v", got.Ranks)
	}
	if len(got.Hotbar) != 10 || got.Hotbar[0] != "skill:7" || got.Hotbar[2] != "item:potion_hp_s" {
		t.Errorf("hotbar = %v", got.Hotbar)
	}
}
