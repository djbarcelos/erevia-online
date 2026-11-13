package zone

import (
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

func TestRingsAndEarringsUseBothSlots(t *testing.T) {
	z := newTestZone(nil)
	p, peer := joinKit(z, 1, "rogue", 0, 0, 0)
	inv := &p.pl.inv
	inv.bag[10] = stack{item("ring_common"), 1}
	inv.bag[11] = stack{item("ring_common"), 1}
	inv.bag[12] = stack{item("ear_common"), 1}
	mdef := p.stats.MDef

	// Duplo clique: primeiro anel no encaixe 1, o segundo no 2.
	z.useItem(p, placeBag, 10)
	z.useItem(p, placeBag, 11)
	if inv.equip[game.SlotRing1].empty() || inv.equip[game.SlotRing2].empty() || !inv.bag[10].empty() || !inv.bag[11].empty() {
		t.Fatalf("rings: %+v / %+v", inv.equip[game.SlotRing1], inv.equip[game.SlotRing2])
	}
	if p.stats.MDef != mdef+4 {
		t.Fatalf("two rings must add their bonus twice: %.0f -> %.0f", mdef, p.stats.MDef)
	}

	// Com os dois ocupados, um terceiro troca com o primeiro.
	p.level = 6
	inv.bag[13] = stack{item("ring_good"), 1}
	z.useItem(p, placeBag, 13)
	if inv.equip[game.SlotRing1].def.ID != "ring_good" || inv.bag[13].def.ID != "ring_common" {
		t.Fatal("third ring should swap with ring 1")
	}

	// Arrastar para um encaixe errado é recusado e desfeito no cliente.
	before := len(peer.msgs)
	z.moveItem(p, placeBag, 12, placeEquip, game.SlotRing2)
	if inv.bag[12].def.ID != "ear_common" || !peer.systemContains("lugar") || len(peer.msgs) == before {
		t.Fatal("earring must not go into a ring slot")
	}
	// Arrastar para o encaixe certo escolhido (brinco 2).
	z.moveItem(p, placeBag, 12, placeEquip, game.SlotEar2)
	if inv.equip[game.SlotEar2].def.ID != "ear_common" || !inv.equip[game.SlotEar1].empty() {
		t.Fatal("earring should go into the chosen slot")
	}

	// Trocar anel 1 ↔ anel 2; brinco não entra no anel.
	z.moveItem(p, placeEquip, game.SlotRing1, placeEquip, game.SlotRing2)
	if inv.equip[game.SlotRing2].def.ID != "ring_good" || inv.equip[game.SlotRing1].def.ID != "ring_common" {
		t.Fatal("ring slots should swap")
	}
	z.moveItem(p, placeEquip, game.SlotEar2, placeEquip, game.SlotRing1)
	if inv.equip[game.SlotEar2].def.ID != "ear_common" || inv.equip[game.SlotRing1].def.ID != "ring_common" {
		t.Fatal("earring must not move into a ring slot")
	}
	z.moveItem(p, placeEquip, game.SlotEar2, placeEquip, game.SlotEar1)
	if !inv.equip[game.SlotEar2].empty() || inv.equip[game.SlotEar1].def.ID != "ear_common" {
		t.Fatal("earring should move to the empty earring slot")
	}
}

func TestEverySlotEquipsAndSaves(t *testing.T) {
	z := newTestZone(nil)
	p, _ := joinKit(z, 1, "knight", 0, 0, 0)
	p.level = 20
	inv := &p.pl.inv
	epics := []string{"helm_epic", "armor_epic", "legs_epic", "gloves_epic", "boots_epic", "cape_epic", "neck_epic",
		"ear_epic", "ear_epic", "ring_epic", "ring_epic", "sword_epic", "shield_epic"}
	for i, id := range epics {
		inv.bag[10+i] = stack{item(id), 1}
		z.useItem(p, placeBag, 10+i)
	}
	for sl := 0; sl < game.EquipSlots; sl++ {
		if inv.equip[sl].empty() || inv.equip[sl].def.Grade != game.GradeEpic {
			t.Fatalf("slot %d (%s) = %+v", sl, game.SlotNames[sl], inv.equip[sl])
		}
	}
	// As botas épicas aceleram, as luvas atacam mais rápido.
	base := game.ClassStats("knight", 20)
	if p.stats.Speed != base.Speed+0.5 || p.stats.AtkInterval >= base.AtkInterval {
		t.Fatalf("speed %.2f interval %.3f", p.stats.Speed, p.stats.AtkInterval)
	}

	// Ida e volta pelo que é salvo: tudo volta para o mesmo encaixe.
	back, skipped := loadInventory(inv.gold, inv.records())
	if skipped != 0 || back.equip != inv.equip {
		t.Fatalf("reload: skipped %d", skipped)
	}
	// Um item salvo num encaixe errado não some: vai para a bolsa.
	odd, skipped := loadInventory(0, []game.ItemRecord{{Equipped: true, Slot: game.SlotRing1, ItemID: "cape_common", Count: 1}})
	if skipped != 0 || !odd.equip[game.SlotRing1].empty() || odd.bag[0].def.ID != "cape_common" {
		t.Fatal("misplaced item should go to the bag")
	}
}

func TestChosenStarterWeaponsWork(t *testing.T) {
	z := newTestZone(nil)
	// Lâmina curva conta como espada para a Esgrima.
	peer := &fakePeer{}
	look := game.Look{Weapon: "blade_common", Offhand: "shield_tower", Emblem: "#4f9dff"}
	z.handle(joinEvent{info: JoinInfo{ID: 1, Name: "Lamina", Class: "knight", Items: game.StarterKitFor("knight", look),
		Emblem: look.Emblem, Ranks: map[uint32]int{game.SkillFencing: 1}, XP: game.XPLevelStart(5), Peer: peer}})
	p := z.players[1]
	if p.pl.inv.weaponType() != game.WeaponSword || p.pl.rank(game.SkillFencing) != 1 {
		t.Fatalf("weapon %q rank %d", p.pl.inv.weaponType(), p.pl.rank(game.SkillFencing))
	}
	withFencing := p.stats.PAtk
	p.pl.ranks[game.SkillFencing] = 0
	z.refreshStats(p)
	if withFencing <= p.stats.PAtk {
		t.Fatal("fencing must count the curved blade as a sword")
	}

	// Os outros veem a arma escolhida e a cor do emblema.
	var me *pb.Entity
	for _, m := range peer.msgs {
		for _, e := range m.GetWelcome().GetEntities() {
			if e.Id == 1 {
				me = e
			}
		}
	}
	if me == nil || me.Weapon != "skeleton_blade" || me.Offhand != "skeleton_shield_large_a" || me.Emblem != "#4f9dff" {
		t.Fatalf("entity = %+v", me)
	}
	// Personagem antigo (sem cor salva) ganha a cor da classe.
	other := &fakePeer{}
	z.handle(joinEvent{info: JoinInfo{ID: 2, Name: "Velho", Class: "mage", Items: game.StarterKit("mage"), Peer: other}})
	if z.entityPB(z.players[2]).Emblem != "#8c6fd6" {
		t.Fatal("old character should get the class emblem")
	}
}
