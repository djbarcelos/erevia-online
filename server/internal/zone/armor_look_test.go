package zone

import (
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// O visual da armadura (tunnel#96) leva a raridade de cada peça equipada para todos.
func TestArmorLookFollowsEquippedGrades(t *testing.T) {
	z := newTestZone(nil)
	p, peer := joinKit(z, 1, "knight", 0, 0, 0)
	inv := &p.pl.inv
	p.level = 18
	if e := z.entityPB(p).GetArmor(); e != nil && e.Chest != 0 {
		t.Fatalf("o kit inicial não leva peito: %v", e)
	}
	inv.bag[5] = stack{def: item("armor_epic"), count: 1}
	inv.bag[6] = stack{def: item("helm_rare"), count: 1}
	inv.bag[7] = stack{def: item("cape_good"), count: 1}
	for _, s := range []int{5, 6, 7} {
		z.useItem(p, placeBag, s)
	}
	a := z.entityPB(p).GetArmor()
	if a == nil || a.Chest != uint32(game.GradeEpic) || a.Helm != uint32(game.GradeRare) || a.Cape != uint32(game.GradeGood) || a.Legs != 0 {
		t.Fatalf("armor look = %v", a)
	}
	if !peer.has(func(m *pb.ServerMessage) bool {
		x := m.GetAppearance().GetArmor()
		return x != nil && x.Chest == uint32(game.GradeEpic) && x.Helm == uint32(game.GradeRare)
	}) {
		t.Fatal("a troca de peça tem de chegar a todos pelo Appearance")
	}
	z.moveItem(p, placeEquip, game.SlotArmor, placeBag, 30)
	if a := z.entityPB(p).GetArmor(); a == nil || a.Chest != 0 || a.Helm != uint32(game.GradeRare) {
		t.Fatalf("tirar o peito zera só o peito: %v", a)
	}
}
