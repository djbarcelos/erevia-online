package zone

import (
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// Achado pelo teste de caos: a conferência da economia (banco × game_events) não fechava
// porque item gasto não virava item_out: poção bebida, pergaminho de encantar (sempre gasto,
// dê certo ou não), pergaminho de retorno e o que um PK deixa cair ao morrer. Itens de
// recompensa que não cabiam na bolsa contavam duas vezes (na recompensa e ao serem pegos).
func TestTelemetryCountsConsumedItems(t *testing.T) {
	z := newCombatZone()
	z.SetEventSink(nopSink{})
	p, _ := joinAt(z, 1, "knight", 30, 0, 0) // fora da cidade: o pergaminho de retorno funciona
	inv := &p.pl.inv
	inv.bag = [game.BagSize]stack{}
	for _, id := range []string{"potion_hp_s", "scroll_ench_weapon", "scroll_return", "sword_common"} {
		inv.add(game.Items[id], 1)
	}
	p.hp = 1
	drainEvents(z)
	z.handle(itemEvent{id: 1, msg: &pb.UseItem{Slot: 0}})
	z.handle(itemEvent{id: 1, msg: &pb.Enchant{ScrollSlot: 1, Place: 0, Slot: 3}})
	z.handle(itemEvent{id: 1, msg: &pb.UseItem{Slot: 2}})
	z.tickFor(game.ReturnChannel + 1)
	evs := drainEvents(z)
	for _, want := range []struct{ id, how string }{{"potion_hp_s", "use"}, {"scroll_ench_weapon", "enchant"}, {"scroll_return", "return"}} {
		if e := findEvent(evs, "item_out", want.id); e == nil || e.Amount != 1 || e.Extra["how"] != want.how {
			t.Errorf("%s gasto: item_out = %+v, quer how=%s", want.id, e, want.how)
		}
	}
	if inv.countOf("potion_hp_s")+inv.countOf("scroll_ench_weapon")+inv.countOf("scroll_return") != 0 {
		t.Fatal("os três deveriam ter sido gastos")
	}
}
