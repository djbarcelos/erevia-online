package game

import (
	"testing"
	"time"
)

func TestArenaEloAndSeason(t *testing.T) {
	if d := ArenaDelta(1000, 1000, true); d != 16 {
		t.Fatalf("iguais: vencer dá 16, deu %d", d)
	}
	if d := ArenaDelta(1000, 1000, false); d != -16 {
		t.Fatalf("iguais: perder tira 16, tirou %d", d)
	}
	if w, l := ArenaDelta(1400, 1000, true), ArenaDelta(1000, 1400, true); w >= l || w < 1 {
		t.Fatalf("o favorito ganha menos que a zebra: %d contra %d", w, l)
	}
	if d := ArenaDelta(2400, 800, true); d != 1 {
		t.Fatalf("vencer sempre dá ao menos 1: %d", d)
	}
	brt := time.FixedZone("BRT", -3*3600)
	if s := ArenaSeason(time.Date(2026, 12, 1, 0, 30, 0, 0, brt)); s != 202612 {
		t.Fatalf("temporada: %d", s)
	}
	// 1º de dezembro, 00:30 em Brasília ainda é 30 de novembro em UTC−3? Não: já é dezembro lá.
	if s := ArenaSeason(time.Date(2026, 12, 1, 2, 0, 0, 0, time.UTC)); s != 202611 {
		t.Fatalf("às 23h de 30/11 em Brasília (02h UTC), ainda novembro: %d", s)
	}
	if l := ArenaSeasonLabel(202611); l != "novembro de 2026" {
		t.Fatal(l)
	}
	if !ArenaSolo.Valid() || !ArenaTrios.Valid() || ArenaMode(2).Valid() || ArenaTrios.Label() != "3x3" {
		t.Fatal("modos")
	}
}
