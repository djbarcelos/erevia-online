package store_test

import (
	"context"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

// characters.zone (migração 00030): o personagem novo nasce no mapa principal, o portão
// grava o mapa de destino e um salvamento sem zona não muda a gravada.
func TestZoneRoundTrip(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	acc, _ := s.LoginIdentity(ctx, "dev", "zona", "zona")
	c, err := s.CreateCharacter(ctx, acc, "Viajante", "knight", game.DefaultLook("knight"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.CharacterForAccount(ctx, acc, c.ID)
	if err != nil || got.Zone != "starter" {
		t.Fatalf("zona inicial = %q / %v", got.Zone, err)
	}
	save := func(zone string, seq int64) {
		st := store.CharacterState{CharacterID: c.ID, Level: 1, HP: -1, MP: -1, X: 0, Z: 1, Zone: zone, Seq: seq}
		if err := s.SaveStates(ctx, []store.CharacterState{st}); err != nil {
			t.Fatal(err)
		}
	}
	save("teste", 1)
	if got, _ = s.CharacterForAccount(ctx, acc, c.ID); got.Zone != "teste" || got.Z != 1 {
		t.Fatalf("depois do portão: zona %q, z %v", got.Zone, got.Z)
	}
	save("", 2)
	if got, _ = s.CharacterForAccount(ctx, acc, c.ID); got.Zone != "teste" {
		t.Fatalf("salvamento sem zona mudou a zona: %q", got.Zone)
	}
}
