package store_test

import (
	"context"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

// As pedras descobertas e a última cidade vão junto com o personagem (coluna discovered,
// migração 00036) e voltam no carregamento.
func TestDiscoveredRoundTrip(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "Andarilho", "Andarilho")
	c, err := s.CreateCharacter(ctx, acc, "Andarilho", "rogue", game.DefaultLook("rogue"))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Discovered.Stones) != 0 || c.Discovered.Town != "" {
		t.Fatalf("personagem novo com descobertas: %+v", c.Discovered)
	}
	d := game.DiscoveryState{Stones: []string{"cidade", "posto", "ossos"}, Town: "cidade"}
	if err := s.SaveStates(ctx, []store.CharacterState{{CharacterID: c.ID, Level: 1, HP: -1, MP: -1, Discovered: d, Seq: 1}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.CharacterForAccount(ctx, acc, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Discovered.Stones) != 3 || got.Discovered.Stones[1] != "posto" || got.Discovered.Town != "cidade" {
		t.Fatalf("descobertas carregadas: %+v", got.Discovered)
	}
}
