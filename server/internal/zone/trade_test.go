package zone

import (
	"strings"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

func (f *fakePeer) lastTrade() *pb.TradeState {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if t := f.msgs[i].GetTrade(); t != nil {
			return t
		}
	}
	return nil
}

func tradeCmd(z *Zone, p *entity, kind pb.TradeCmdKind, set func(*pb.TradeCmd)) {
	p.pl.moveTokens = z.cfg.MaxMovesPerSec
	m := &pb.TradeCmd{Kind: kind}
	if set != nil {
		set(m)
	}
	z.handle(tradeEvent{id: p.id, msg: m})
}

// openTestTrade põe Ana e Beto lado a lado e abre a troca entre eles.
func openTestTrade(t *testing.T, saves chan []SavedState) (*Zone, *entity, *entity, *fakePeer, *fakePeer) {
	t.Helper()
	z := newCombatZone()
	if saves != nil {
		z.saves = saves
	}
	a, pa := joinAt(z, 1, "knight", 0, 0, 0)
	b, pb_ := joinAt(z, 2, "mage", 2, 0, 0)
	tradeCmd(z, a, pb.TradeCmdKind_TRADE_CMD_REQUEST, func(m *pb.TradeCmd) { m.TargetId = b.id })
	inv := pb_.lastInvite()
	if inv == nil || inv.Kind != pb.InviteKind_INVITE_KIND_TRADE {
		t.Fatalf("expected trade invite, got %+v", inv)
	}
	b.pl.moveTokens = z.cfg.MaxMovesPerSec
	z.handle(socialEvent{id: b.id, msg: &pb.InviteReply{InviteId: inv.Id, Accept: true}})
	if a.pl.trade == nil || a.pl.trade != b.pl.trade || !pa.lastTrade().Open {
		t.Fatal("trade should be open for both")
	}
	return z, a, b, pa, pb_
}

func giveItem(p *entity, slot int, id string, count, enchant int) {
	p.pl.inv.bag[slot] = stack{def: game.Items[id], count: count, enchant: enchant}
}

func lockAndConfirm(z *Zone, a, b *entity) {
	for _, p := range []*entity{a, b} {
		tradeCmd(z, p, pb.TradeCmdKind_TRADE_CMD_LOCK, nil)
	}
	for _, p := range []*entity{a, b} {
		tradeCmd(z, p, pb.TradeCmdKind_TRADE_CMD_CONFIRM, nil)
	}
}

func TestTradeRequestNeedsProximity(t *testing.T) {
	z := newCombatZone()
	a, pa := joinAt(z, 1, "knight", 0, 0, 0)
	b, pb_ := joinAt(z, 2, "mage", 30, 0, 0)
	tradeCmd(z, a, pb.TradeCmdKind_TRADE_CMD_REQUEST, func(m *pb.TradeCmd) { m.TargetId = b.id })
	if pb_.lastInvite() != nil || !pa.systemContains("Chegue mais perto") {
		t.Fatal("far players must not get a trade invite")
	}
}

func TestTradeSwapsItemsAndGoldAndSavesBothTogether(t *testing.T) {
	saves := make(chan []SavedState, 16)
	z, a, b, pa, _ := openTestTrade(t, saves)
	giveItem(a, 10, "scroll_ench_weapon", 3, 0)
	giveItem(b, 11, game.StarterKit("knight")[0].ItemID, 1, 5) // a arma inicial (não empilha)
	weapon := b.pl.inv.bag[11].def
	a.pl.inv.gold, b.pl.inv.gold = 1000, 50
	for len(saves) > 0 {
		<-saves
	}

	tradeCmd(z, a, pb.TradeCmdKind_TRADE_CMD_ADD_ITEM, func(m *pb.TradeCmd) { m.Slot = 10; m.Count = 2 })
	tradeCmd(z, a, pb.TradeCmdKind_TRADE_CMD_SET_GOLD, func(m *pb.TradeCmd) { m.Gold = 300 })
	tradeCmd(z, b, pb.TradeCmdKind_TRADE_CMD_ADD_ITEM, func(m *pb.TradeCmd) { m.Slot = 11 })
	lockAndConfirm(z, a, b)

	if a.pl.trade != nil || b.pl.trade != nil || pa.lastTrade().Open {
		t.Fatal("trade should be closed after it runs")
	}
	if a.pl.inv.countOf("scroll_ench_weapon") != 1 || b.pl.inv.countOf("scroll_ench_weapon") != 2 {
		t.Fatalf("scrolls: a=%d b=%d", a.pl.inv.countOf("scroll_ench_weapon"), b.pl.inv.countOf("scroll_ench_weapon"))
	}
	if a.pl.inv.gold != 700 || b.pl.inv.gold != 350 {
		t.Fatalf("gold: a=%d b=%d", a.pl.inv.gold, b.pl.inv.gold)
	}
	if b.pl.inv.countOf(weapon.ID) != 0 {
		t.Fatal("the weapon should have left Beto's bag")
	}
	got := a.pl.inv.bag[a.pl.inv.slotOf(weapon.ID)]
	if got.enchant != 5 {
		t.Fatalf("enchant must travel with the item, got +%d", got.enchant)
	}
	select {
	case batch := <-saves:
		ids := map[uint32]bool{}
		for _, s := range batch {
			ids[s.ID] = true
		}
		if !ids[a.id] || !ids[b.id] {
			t.Fatalf("both players must be saved in the same batch, got %v", ids)
		}
	default:
		t.Fatal("trade must be saved right away")
	}
}

func TestChangingAnOfferUnlocksBothAndConfirmNeedsLocks(t *testing.T) {
	z, a, b, _, pb_ := openTestTrade(t, nil)
	giveItem(a, 0, "potion_hp_s", 5, 0)
	a.pl.inv.gold = 100
	tradeCmd(z, b, pb.TradeCmdKind_TRADE_CMD_CONFIRM, nil)
	if !pb_.systemContains("travadas") {
		t.Fatal("confirm without both locks must be refused")
	}
	tradeCmd(z, a, pb.TradeCmdKind_TRADE_CMD_LOCK, nil)
	tradeCmd(z, b, pb.TradeCmdKind_TRADE_CMD_LOCK, nil)
	tradeCmd(z, a, pb.TradeCmdKind_TRADE_CMD_SET_GOLD, func(m *pb.TradeCmd) { m.Gold = 10 })
	tr := a.pl.trade
	if tr.offer[0].locked || tr.offer[1].locked {
		t.Fatal("changing an offer must unlock both sides")
	}
	if got := pb_.lastTrade(); got.Theirs.Gold != 10 || got.Mine.Locked {
		t.Fatalf("partner view = %+v", got)
	}
	tradeCmd(z, a, pb.TradeCmdKind_TRADE_CMD_SET_GOLD, func(m *pb.TradeCmd) { m.Gold = 999999 })
	if tr.offer[0].gold != 100 {
		t.Fatalf("gold offer must be capped by what the player has, got %d", tr.offer[0].gold)
	}
}

func TestDisconnectMidTradeMovesNothing(t *testing.T) {
	z, a, b, _, pb_ := openTestTrade(t, nil)
	giveItem(a, 0, "potion_hp_s", 5, 0)
	tradeCmd(z, a, pb.TradeCmdKind_TRADE_CMD_ADD_ITEM, func(m *pb.TradeCmd) { m.Slot = 0 })
	tradeCmd(z, a, pb.TradeCmdKind_TRADE_CMD_LOCK, nil)
	tradeCmd(z, b, pb.TradeCmdKind_TRADE_CMD_LOCK, nil)
	tradeCmd(z, a, pb.TradeCmdKind_TRADE_CMD_CONFIRM, nil)
	before := b.pl.inv.countOf("potion_hp_s")

	z.removePlayer(a.id)
	if b.pl.trade != nil || pb_.lastTrade().Open || !pb_.systemContains("saiu do jogo") {
		t.Fatal("partner's trade must close when the other leaves")
	}
	if b.pl.inv.countOf("potion_hp_s") != before {
		t.Fatal("nothing may move when a trade is interrupted")
	}
	st := z.recent[a.id].state
	n := 0
	for _, it := range st.Items {
		if it.ItemID == "potion_hp_s" && !it.Equipped && it.Slot == 0 {
			n = it.Count
		}
	}
	if n != 5 {
		t.Fatalf("leaver keeps the offered potions, got %d", n)
	}
}

func TestUsingAnOfferedItemRemovesItFromTheOffer(t *testing.T) {
	z, a, b, _, _ := openTestTrade(t, nil)
	giveItem(a, 0, "potion_hp_s", 1, 0)
	tradeCmd(z, a, pb.TradeCmdKind_TRADE_CMD_ADD_ITEM, func(m *pb.TradeCmd) { m.Slot = 0 })
	tradeCmd(z, a, pb.TradeCmdKind_TRADE_CMD_LOCK, nil)
	tradeCmd(z, b, pb.TradeCmdKind_TRADE_CMD_LOCK, nil)

	a.hp = 1
	a.pl.moveTokens = z.cfg.MaxMovesPerSec
	z.handle(itemEvent{id: a.id, msg: &pb.UseItem{Place: placeBag, Slot: 0}})
	tr := a.pl.trade
	if len(tr.offer[0].items) != 0 || tr.offer[0].locked || tr.offer[1].locked {
		t.Fatalf("drunk potion must leave the offer and unlock, offer = %+v", tr.offer[0])
	}
}

func TestFullBagCancelsWithoutChanges(t *testing.T) {
	z, a, b, _, pb_ := openTestTrade(t, nil)
	giveItem(a, 0, "scroll_ench_weapon", 1, 0)
	filler := game.Items["scroll_ench_armor"]
	for i := range b.pl.inv.bag {
		b.pl.inv.bag[i] = stack{def: filler, count: filler.Stack}
	}
	bagA, bagB := a.pl.inv, b.pl.inv
	tradeCmd(z, a, pb.TradeCmdKind_TRADE_CMD_ADD_ITEM, func(m *pb.TradeCmd) { m.Slot = 0 })
	lockAndConfirm(z, a, b)
	if a.pl.inv != bagA || b.pl.inv != bagB {
		t.Fatal("a failed trade must not change any bag")
	}
	if !strings.Contains(strings.Join(pb_.systems(), "|"), "não tem espaço") {
		t.Fatal("expected a no-room message")
	}
}

func TestWalkingAwayCancelsTrade(t *testing.T) {
	z, a, b, pa, _ := openTestTrade(t, nil)
	b.teleportTo(Vec2{X: 40, Z: 0})
	z.tickFor(1.05)
	if a.pl.trade != nil || pa.lastTrade().Open {
		t.Fatal("trade must close when players walk apart")
	}
}
