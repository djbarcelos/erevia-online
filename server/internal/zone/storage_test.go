package zone

import (
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

func (z *Zone) keeper() *entity {
	for _, n := range z.npcs {
		if n.npc.role == pb.NpcRole_NPC_ROLE_STORAGE {
			return n
		}
	}
	return nil
}

// account entra com um personagem da conta acc, perto do Guarda do Armazém.
func account(z *Zone, id uint32, name string, acc int64, d *StorageData) (*entity, *fakePeer) {
	k := z.keeper()
	peer := &fakePeer{}
	z.handle(joinEvent{info: JoinInfo{ID: id, Name: name, Class: "knight", X: k.pos.X + 1, Z: k.pos.Z,
		Items: game.StarterKit("knight"), Gold: 500, AccountID: acc, Storage: d, Peer: peer}})
	return z.players[id], peer
}

func store(z *Zone, p *entity, m *pb.StorageMove) {
	m.NpcId = z.keeper().id
	z.handle(itemEvent{id: p.id, msg: m})
	z.tickFor(0.2)
}

func (f *fakePeer) lastStorage() *pb.Storage {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if s := f.msgs[i].GetStorage(); s != nil {
			return s
		}
	}
	return nil
}

func TestStorageDepositWithdrawAndGold(t *testing.T) {
	z := newCombatZone()
	p, peer := account(z, 1, "Ana", 7, nil)
	slot := p.pl.inv.slotOf("potion_hp_s")
	n := p.pl.inv.bag[slot].count

	store(z, p, &pb.StorageMove{FromPlace: placeBag, FromSlot: uint32(slot), ToPlace: placeStorage, ToSlot: 3, Count: 2})
	st := z.storages[7]
	if st.slots[3].def == nil || st.slots[3].count != 2 || p.pl.inv.countOf("potion_hp_s") != n-2 {
		t.Fatal("deposit moves the chosen amount")
	}
	if s := peer.lastStorage(); s == nil || len(s.Items) != 1 || s.Items[0].Slot != 3 {
		t.Fatalf("storage state = %+v", s)
	}
	store(z, p, &pb.StorageMove{FromPlace: placeStorage, FromSlot: 3, ToPlace: placeBag, ToSlot: 99})
	if !st.slots[3].empty() || p.pl.inv.countOf("potion_hp_s") != n {
		t.Fatal("withdraw brings everything back")
	}

	k := z.keeper().id
	z.handle(itemEvent{id: p.id, msg: &pb.StorageGold{NpcId: k, Amount: 300}})
	z.tickFor(0.2)
	if st.gold != 300 || p.pl.inv.gold != 200 {
		t.Fatalf("deposit gold: storage %d bag %d", st.gold, p.pl.inv.gold)
	}
	z.handle(itemEvent{id: p.id, msg: &pb.StorageGold{NpcId: k, Amount: -1000}})
	z.tickFor(0.2)
	if st.gold != 0 || p.pl.inv.gold != 500 {
		t.Fatal("withdrawing more than stored takes only what there is")
	}
}

func TestStorageKeepsEnchantAndRefusesWhenFull(t *testing.T) {
	z := newCombatZone()
	p, peer := account(z, 1, "Ana", 7, nil)
	p.pl.inv.put(item("sword_good"), 1, 6)
	slot := -1
	for i, s := range p.pl.inv.bag {
		if !s.empty() && s.enchant == 6 {
			slot = i
		}
	}
	store(z, p, &pb.StorageMove{FromPlace: placeBag, FromSlot: uint32(slot), ToPlace: placeStorage})
	st := z.storages[7]
	found := false
	for _, s := range st.slots {
		if s.def == item("sword_good") && s.enchant == 6 {
			found = true
		}
	}
	if !found {
		t.Fatal("enchanted items keep the +N in storage")
	}
	for i := range st.slots {
		if st.slots[i].empty() {
			st.slots[i] = stack{def: item("ring_common"), count: 1}
		}
	}
	slot = p.pl.inv.slotOf("potion_hp_s")
	store(z, p, &pb.StorageMove{FromPlace: placeBag, FromSlot: uint32(slot), ToPlace: placeStorage})
	if p.pl.inv.slotOf("potion_hp_s") != slot || !peer.systemContains("armazém está cheio") {
		t.Fatal("a full storage refuses")
	}
}

func TestStorageSharedByAccountAndSavedOnce(t *testing.T) {
	z := newCombatZone()
	saved := &StorageData{Gold: 40, Items: []game.ItemRecord{{Slot: 0, ItemID: "bone_old", Count: 9}}}
	a, _ := account(z, 1, "Ana", 7, saved)
	b, pb_ := account(z, 2, "Alt", 7, &StorageData{}) // o banco devolve um estado velho
	if z.storageOf(a) != z.storageOf(b) || z.storageOf(b).gold != 40 {
		t.Fatal("two characters of the same account share one storage in memory")
	}
	store(z, b, &pb.StorageMove{FromPlace: placeStorage, FromSlot: 0, ToPlace: placeBag})
	if b.pl.inv.countOf("bone_old") != 9 || !z.storageOf(a).slots[0].empty() {
		t.Fatal("what one takes, the other no longer sees (no duplication)")
	}
	_ = pb_
	s1 := z.savedState(b)
	s2 := z.savedState(a)
	if s1.Storage == nil || s2.Storage != nil {
		t.Fatal("the changed storage goes with one save only")
	}
	if len(s1.Storage.Items) != 0 || s1.Storage.AccountID != 7 || s1.Storage.Seq == 0 {
		t.Fatalf("storage save = %+v", s1.Storage)
	}

	// Sai todo mundo e volta: vale a memória, não o banco velho.
	z.removePlayer(1)
	z.removePlayer(2)
	c, _ := account(z, 1, "Ana", 7, saved)
	if !z.storageOf(c).slots[0].empty() {
		t.Fatal("relogging uses the storage in memory, not the stale database copy")
	}
}

func TestStorageNeedsTheKeeper(t *testing.T) {
	z := newCombatZone()
	p, peer := account(z, 1, "Ana", 7, nil)
	p.teleportTo(Vec2{X: 30})
	slot := p.pl.inv.slotOf("potion_hp_s")
	store(z, p, &pb.StorageMove{FromPlace: placeBag, FromSlot: uint32(slot), ToPlace: placeStorage})
	if !z.storages[7].slots[0].empty() || !peer.systemContains("Chegue mais perto") {
		t.Fatal("storage only works next to the keeper")
	}
	// Equipado não entra (só da bolsa).
	p.teleportTo(z.keeper().pos)
	store(z, p, &pb.StorageMove{FromPlace: placeEquip, FromSlot: 0, ToPlace: placeStorage})
	for _, s := range z.storages[7].slots {
		if !s.empty() {
			t.Fatal("equipped items cannot be stored")
		}
	}
}
