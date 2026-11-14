package store_test

import (
	"context"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

// Subclasse (tunnel#156): as classes vão e voltam do banco; as colunas de progressão são
// as da classe ativa, e a tela de personagens, o ranking, o clã e os amigos mostram ela.
func TestSubclassRoundTrip(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "sub", "sub")
	c, err := s.CreateCharacter(ctx, acc, "Sendas", "knight", game.DefaultLook("knight"))
	if err != nil {
		t.Fatal(err)
	}
	if c.ActiveClass != "knight" || c.ClassLevels != nil {
		t.Fatalf("personagem novo: ativa %q, classes %+v", c.ActiveClass, c.ClassLevels)
	}
	set := game.ClassSet{Main: "knight", Active: "mage", SwitchedAt: 1_700_000_000, List: []game.ClassProgress{
		{Class: "knight", Level: 32, XP: game.XPLevelStart(32) + 7, Attrs: game.Attrs{Str: 10, Con: 5},
			Ranks: map[uint32]int{game.SkillShieldBash: 3}, Hotbar: []string{"skill:1"}, Spec: "bastion"},
		{Class: "mage", Level: 21, XP: game.XPLevelStart(21)},
	}}
	st := store.CharacterState{CharacterID: c.ID, Level: 21, XP: game.XPLevelStart(21), HP: -1, MP: -1,
		Attrs: game.Attrs{Int: 3}, Classes: set, Seq: 1}
	if err := s.SaveStates(ctx, []store.CharacterState{st}); err != nil {
		t.Fatal(err)
	}
	got, err := s.CharacterForAccount(ctx, acc, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Class != "knight" || got.ActiveClass != "mage" || got.Level != 21 || got.Attrs != (game.Attrs{Int: 3}) {
		t.Fatalf("carregado: classe %s ativa %s nível %d attrs %+v", got.Class, got.ActiveClass, got.Level, got.Attrs)
	}
	k := got.Classes.Find("knight")
	if got.Classes.Active != "mage" || got.Classes.SwitchedAt != 1_700_000_000 || k == nil || k.Level != 32 ||
		k.XP != game.XPLevelStart(32)+7 || k.Attrs.Str != 10 || k.Ranks[game.SkillShieldBash] != 3 || k.Spec != "bastion" || k.Hotbar[0] != "skill:1" {
		t.Fatalf("classes carregadas: %+v", got.Classes)
	}
	if len(got.ClassLevels) != 2 || got.ClassLevels[0] != (store.ClassLevel{Class: "knight", Level: 32, Main: true}) ||
		got.ClassLevels[1] != (store.ClassLevel{Class: "mage", Level: 21}) {
		t.Fatalf("níveis por classe: %+v", got.ClassLevels)
	}

	// Um salvamento sem classes (Main vazio) não apaga a coluna.
	st2 := store.CharacterState{CharacterID: c.ID, Level: 21, XP: game.XPLevelStart(21) + 5, HP: -1, MP: -1, Seq: 2}
	if err := s.SaveStates(ctx, []store.CharacterState{st2}); err != nil {
		t.Fatal(err)
	}
	if got, err = s.CharacterForAccount(ctx, acc, c.ID); err != nil || got.Classes.Active != "mage" || len(got.Classes.List) != 2 {
		t.Fatalf("classes depois de um salvamento sem elas: %+v / %v", got.Classes, err)
	}

	// A lista de personagens mostra a classe ativa com o nível dela.
	list, err := s.ListCharacters(ctx, acc)
	if err != nil || len(list) != 1 || list[0].ActiveClass != "mage" || list[0].Class != "knight" || len(list[0].ClassLevels) != 2 {
		t.Fatalf("lista: %+v / %v", list, err)
	}
	// O ranking usa o maior nível entre as classes (com a classe dele): trocar para a
	// subclasse 21 não derruba o cavaleiro 32. Um outro personagem 25 fica abaixo.
	acc2, _ := s.LoginIdentity(ctx, "dev", "sub2", "sub2")
	o, err := s.CreateCharacter(ctx, acc2, "Outro", "rogue", game.DefaultLook("rogue"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveStates(ctx, []store.CharacterState{{CharacterID: o.ID, Level: 25, XP: game.XPLevelStart(25), HP: -1, MP: -1, Seq: 3}}); err != nil {
		t.Fatal(err)
	}
	r, err := s.Rankings(ctx, 10)
	if err != nil || len(r.Level) != 2 || r.Level[0].Name != "Sendas" || r.Level[0].Class != "knight" || r.Level[0].Level != 32 || r.LevelPos[c.ID] != 1 {
		t.Fatalf("ranking: %+v / %v", r.Level, err)
	}
	// Com a ativa acima das outras, vale a ativa.
	set.List[0].Level, set.List[0].XP = 20, game.XPLevelStart(20)
	set.Active = "mage"
	st3 := store.CharacterState{CharacterID: c.ID, Level: 26, XP: game.XPLevelStart(26), HP: -1, MP: -1, Classes: set, Seq: 4}
	if err := s.SaveStates(ctx, []store.CharacterState{st3}); err != nil {
		t.Fatal(err)
	}
	if r, err = s.Rankings(ctx, 10); err != nil || r.Level[0].Class != "mage" || r.Level[0].Level != 26 {
		t.Fatalf("ranking com a ativa mais alta: %+v / %v", r.Level, err)
	}
}
