package zone

import (
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// joinKit entra com o kit inicial da classe e moedas.
func joinKit(z *Zone, id uint32, class string, x, zz float64, gold int64) (*entity, *fakePeer) {
	peer := &fakePeer{}
	z.handle(joinEvent{info: JoinInfo{ID: id, Name: "K" + class, Class: class, X: x, Z: zz, Gold: gold,
		Items: game.StarterKit(class), Peer: peer}})
	return z.players[id], peer
}

func (f *fakePeer) lastInventory() *pb.Inventory {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if inv := f.msgs[i].GetInventory(); inv != nil {
			return inv
		}
		if w := f.msgs[i].GetWelcome(); w != nil {
			return w.Inventory
		}
	}
	return nil
}

func item(id string) *game.ItemDef { return game.Items[id] }

func TestInventoryAddStacksAndFills(t *testing.T) {
	var inv inventory
	if left := inv.add(item("potion_hp_s"), 1500); left != 0 {
		t.Fatalf("left = %d", left)
	}
	if inv.bag[0].count != 999 || inv.bag[1].count != 501 {
		t.Fatalf("stacks = %d, %d", inv.bag[0].count, inv.bag[1].count)
	}
	// Equipamentos não empilham: um por espaço até a bolsa encher.
	if left := inv.add(item("sword_good"), game.BagSize); left != 2 {
		t.Fatalf("left = %d, want 2 (38 free slots)", left)
	}
	if inv.canAdd(item("bone_old"), 1) {
		t.Fatal("full bag must refuse new item")
	}
	if !inv.canAdd(item("potion_hp_s"), 498) || inv.canAdd(item("potion_hp_s"), 499) {
		t.Fatal("room should count partial stacks")
	}
}

func TestInventoryMoveMergesAndSwaps(t *testing.T) {
	var inv inventory
	inv.bag[0] = stack{item("bone_old"), 990}
	inv.bag[1] = stack{item("bone_old"), 20}
	inv.moveInBag(1, 0)
	if inv.bag[0].count != 999 || inv.bag[1].count != 11 {
		t.Fatalf("merge = %d, %d", inv.bag[0].count, inv.bag[1].count)
	}
	inv.bag[2] = stack{item("skull_cracked"), 1}
	inv.moveInBag(2, 1)
	if inv.bag[1].def.ID != "skull_cracked" || inv.bag[2].def.ID != "bone_old" {
		t.Fatal("different items should swap")
	}
}

func TestStarterKitKeepsOldBalance(t *testing.T) {
	// A arma (e o escudo) iniciais devolvem o ataque/defesa que a classe tinha antes dos itens.
	want := map[string][2]float64{"knight": {14, 30}, "barbarian": {18, 20}, "mage": {20, 12}, "rogue": {12, 16}}
	z := newTestZone(nil)
	for i, class := range []string{"knight", "barbarian", "mage", "rogue"} {
		p, _ := joinKit(z, uint32(i+1), class, 0, 0, 0)
		atk := p.stats.PAtk
		if class == "mage" {
			atk = p.stats.MAtk
		}
		if atk != want[class][0] || p.stats.PDef != want[class][1] {
			t.Errorf("%s: atk %.0f def %.0f, want %v", class, atk, p.stats.PDef, want[class])
		}
	}
}

func TestEquipRules(t *testing.T) {
	z := newTestZone(nil)
	p, peer := joinKit(z, 1, "mage", 0, 0, 0)
	inv := &p.pl.inv
	inv.bag[5] = stack{item("sword_good"), 1}
	inv.bag[6] = stack{item("staff_good"), 1}

	z.useItem(p, placeBag, 5)
	if !peer.systemContains("classe") || inv.equip[game.SlotWeapon].def.ID != "staff_common" {
		t.Fatal("mage must not equip a sword")
	}
	z.useItem(p, placeBag, 6)
	if !peer.systemContains("nível") {
		t.Fatal("level 1 must not equip a level 6 item")
	}

	p.level = 6
	matk := p.stats.MAtk
	z.useItem(p, placeBag, 6)
	if inv.equip[game.SlotWeapon].def.ID != "staff_good" || inv.bag[6].def.ID != "staff_common" {
		t.Fatal("equip should swap with the bag slot")
	}
	if p.stats.MAtk <= matk {
		t.Fatalf("matk %v -> %v", matk, p.stats.MAtk)
	}
	armed := p.stats.MAtk
	if !peer.has(func(m *pb.ServerMessage) bool { return m.GetAppearance().GetWeapon() == "staff" }) {
		t.Fatal("weapon change must broadcast appearance")
	}

	// Desequipar pela bolsa: vai para o espaço pedido.
	z.moveItem(p, placeEquip, game.SlotWeapon, placeBag, 30)
	if !inv.equip[game.SlotWeapon].empty() || inv.bag[30].def.ID != "staff_good" {
		t.Fatal("unequip to slot 30")
	}
	if p.stats.MAtk != armed-17 {
		t.Fatal("unarmed should lose the staff's bonus")
	}
}

func TestPotionHealsWithCooldown(t *testing.T) {
	z := newTestZone(nil)
	p, peer := joinKit(z, 1, "knight", 0, 0, 0)
	z.useItem(p, placeBag, 0)
	if !peer.systemContains("cheia") || p.pl.inv.bag[0].count != 5 {
		t.Fatal("full HP must not waste a potion")
	}
	p.hp = 10
	z.useItem(p, placeBag, 0)
	if p.hp != 90 || p.pl.inv.bag[0].count != 4 {
		t.Fatalf("hp %v count %d", p.hp, p.pl.inv.bag[0].count)
	}
	z.useItem(p, placeBag, 0)
	if p.hp != 90 || p.pl.inv.bag[0].count != 4 {
		t.Fatal("potion on cooldown must be refused")
	}
	z.tickFor(game.PotionCooldown + 0.1)
	p.hp = 10
	z.useItem(p, placeBag, 0)
	if p.pl.inv.bag[0].count != 3 {
		t.Fatal("potion should work after cooldown")
	}
}

func TestLootReservedForTopDamageThenFree(t *testing.T) {
	z := newCombatZone(fixed("bone_lord", 22, 40, 0)) // o chefe sempre deixa itens
	m := z.onlyMonster()
	a, _ := joinKit(z, 1, "knight", 38, 0, 0)
	b, peerB := joinKit(z, 2, "rogue", 42, 0, 0)
	m.mon.hate[b.id] = 500
	m.mon.hate[a.id] = 10
	z.kill(m, a) // A deu o último golpe, mas B causou mais dano
	m.mon.respawnAt = 1e9 // o chefe não volta durante o teste

	if len(z.ground) == 0 {
		t.Fatal("boss must drop loot")
	}
	var crown *groundItem
	for _, g := range z.ground {
		if g.owner != b.id {
			t.Fatalf("owner = %d, want %d", g.owner, b.id)
		}
		if g.def.ID == "broken_crown" {
			crown = g
		}
	}
	z.requestPickup(a, crown.id)
	z.tickFor(1)
	if z.ground[crown.id] == nil || a.pl.inv.countOf("broken_crown") != 0 {
		t.Fatal("A must not take B's reserved loot")
	}
	z.requestPickup(b, crown.id)
	z.tickFor(1)
	if b.pl.inv.countOf("broken_crown") != 1 || !peerB.systemContains("Coroa Partida") {
		t.Fatal("B should pick its crown")
	}

	// Depois da reserva, qualquer um pega; e o que sobrar some em 60 s.
	z.tickFor(reservedFor)
	var other *groundItem
	for _, g := range z.ground {
		if g.def.Kind != game.KindGold {
			other = g
		}
	}
	z.requestPickup(a, other.id)
	z.tickFor(2)
	if a.pl.inv.countOf(other.def.ID) == 0 {
		t.Fatal("A should pick free loot")
	}
	z.tickFor(groundLife)
	if len(z.ground) != 0 {
		t.Fatalf("%d items should have expired", len(z.ground))
	}
}

func TestPickupGoldAddsToBalance(t *testing.T) {
	z := newCombatZone()
	p, _ := joinKit(z, 1, "knight", 30, 0, 10)
	g := &groundItem{id: z.nextGroundID(), def: item(game.GoldID), count: 25, pos: Vec2{X: 33, Z: 0}, expires: 999}
	z.ground[g.id] = g
	z.requestPickup(p, g.id)
	z.tickFor(2)
	if p.pl.inv.gold != 35 {
		t.Fatalf("gold = %d", p.pl.inv.gold)
	}
}

func TestShopBuyAndSell(t *testing.T) {
	z := newTestZone(nil)
	merchant, smith := npcIDBase, npcIDBase+1
	p, peer := joinKit(z, 1, "knight", 6, -4, 100)

	z.buy(p, merchant, "potion_hp_s", 3)
	inv := &p.pl.inv
	if inv.gold != 40 || inv.countOf("potion_hp_s") != 8 {
		t.Fatalf("gold %d potions %d", inv.gold, inv.countOf("potion_hp_s"))
	}
	z.buy(p, merchant, "potion_hp_l", 1)
	if !peer.systemContains("insuficientes") || inv.gold != 40 {
		t.Fatal("buy without gold must fail")
	}
	z.buy(p, merchant, "sword_good", 1)
	if inv.countOf("sword_good") != 0 {
		t.Fatal("merchant does not sell swords")
	}
	z.buy(p, smith, "armor_common", 1)
	if !peer.systemContains("Chegue mais perto") {
		t.Fatal("smith is far away")
	}

	// Vende 2 poções: 40% de 20 = 8 cada.
	z.sell(p, merchant, 0, 2)
	if inv.gold != 56 || inv.countOf("potion_hp_s") != 6 {
		t.Fatalf("gold %d potions %d", inv.gold, inv.countOf("potion_hp_s"))
	}
	if p.pl.saveAt == 0 {
		t.Fatal("trade should schedule a save")
	}
	if last := peer.lastInventory(); last.Gold != 56 {
		t.Fatalf("inventory message gold = %d", last.Gold)
	}
}

func TestReloginUsesInMemoryState(t *testing.T) {
	saves := make(chan []SavedState, 10)
	z := newTestZone(saves)
	p, _ := joinKit(z, 1, "knight", 0, 0, 50)
	p.pl.inv.gold = 999
	p.pl.inv.add(item("broken_crown"), 1)
	z.handle(leaveEvent{id: 1, peer: p.pl.peer})

	// O banco ainda tem o estado antigo (salvamento em andamento): o relogin traz o da memória.
	q, peer := joinKit(z, 1, "knight", 0, 0, 50)
	if q.pl.inv.gold != 999 || q.pl.inv.countOf("broken_crown") != 1 {
		t.Fatalf("relogin lost state: gold %d", q.pl.inv.gold)
	}
	if w := peer.lastInventory(); w.Gold != 999 {
		t.Fatal("welcome should carry the inventory")
	}
	if s := (<-saves)[0]; s.Gold != 999 || s.Seq == 0 {
		t.Fatalf("saved %+v", s)
	}
}

func TestSaveQueueFullKeepsStatePending(t *testing.T) {
	saves := make(chan []SavedState) // sem espaço: todo envio sem bloqueio falha
	z := newTestZone(saves)
	p, _ := joinKit(z, 1, "knight", 0, 0, 0)
	p.pl.inv.gold = 7
	z.saveAll(false)
	if len(z.pending) != 1 || z.pending[1].Gold != 7 {
		t.Fatal("state should stay pending")
	}
	got := make(chan []SavedState, 1)
	go func() { got <- <-saves }()
	z.saveAll(true)
	if b := <-got; len(b) != 1 || b[0].Gold != 7 {
		t.Fatalf("batch = %+v", b)
	}
}
