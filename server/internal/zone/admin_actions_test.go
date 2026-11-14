package zone

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

const potion = "potion_hp_s"

// countItem soma as unidades de um item numa lista salva (bolsa e equipamento).
func countItem(items []game.ItemRecord, id string) int {
	n := 0
	for _, r := range items {
		if r.ItemID == id {
			n += r.Count
		}
	}
	return n
}

// lastSaved: o último estado do personagem que foi para o canal de persistência.
func lastSaved(t *testing.T, saves chan []SavedState, id uint32) SavedState {
	t.Helper()
	var st SavedState
	found := false
	for _, s := range drainSaves(saves) {
		if s.ID == id {
			st, found = s, true
		}
	}
	if !found {
		t.Fatalf("no save for %d", id)
	}
	return st
}

func joinInfo(z *Zone, info JoinInfo) *fakePeer {
	p := &fakePeer{}
	info.Peer = p
	if info.Class == "" {
		info.Class = "knight"
	}
	z.handle(joinEvent{info: info})
	return p
}

func TestAdminEditOnlineAppliesInMemoryAndSavesNow(t *testing.T) {
	saves := make(chan []SavedState, 64)
	z := newTestZone(saves)
	p, peer := joinKit(z, 1, "knight", 0, 0, 100)
	drainSaves(saves)

	r := z.adminEdit(1, AdminEdit{Kind: EditGiveItem, Item: potion, Count: 3})
	if r.Err != nil || r.Where != WhereOnline || r.Result.Count != 3 {
		t.Fatalf("give online = %+v", r)
	}
	if got := p.pl.inv.countOf(potion); got != countItem(game.StarterKit("knight"), potion)+3 {
		t.Fatalf("bag has %d potions", got)
	}
	// Salvo na hora (com save_seq novo), não só no próximo saveDue.
	st := lastSaved(t, saves, 1)
	if countItem(st.Items, potion) != p.pl.inv.countOf(potion) || st.Seq == 0 {
		t.Fatalf("saved state = %+v", st)
	}
	if !peer.has(func(m *pb.ServerMessage) bool { return m.GetInventory() != nil }) {
		t.Fatal("the player sees the new inventory")
	}

	// Tirar mais do que tem não mexe em nada.
	before := p.pl.inv.records()
	if r := z.adminEdit(1, AdminEdit{Kind: EditTakeItem, Item: potion, Count: 999}); r.Err == nil {
		t.Fatal("taking more than the player has must fail")
	}
	if !equalRecords(before, p.pl.inv.records()) {
		t.Fatal("a failed take changed the inventory")
	}

	// Moedas: tirar mais do que tem falha; dar e tirar mexem no ouro.
	if r := z.adminEdit(1, AdminEdit{Kind: EditGold, Gold: -1000}); r.Err == nil {
		t.Fatal("taking more gold than the player has must fail")
	}
	if r := z.adminEdit(1, AdminEdit{Kind: EditGold, Gold: -40}); r.Err != nil || p.pl.inv.gold != 60 {
		t.Fatalf("take gold = %+v, gold %d", r, p.pl.inv.gold)
	}

	// Tirar a arma equipada (sem nenhuma na bolsa) tira do equipamento.
	r = z.adminEdit(1, AdminEdit{Kind: EditTakeItem, Item: "sword_common", Count: 1})
	if r.Err != nil || !p.pl.inv.equip[game.SlotWeapon].empty() {
		t.Fatalf("take equipped = %+v", r)
	}

	// Teleporte (dentro do mapa).
	r = z.adminEdit(1, AdminEdit{Kind: EditTeleport, X: 40, Z: -20})
	if r.Err != nil || dist(p.pos, Vec2{X: 40, Z: -20}) > 2 {
		t.Fatalf("teleport = %+v, pos %+v", r, p.pos)
	}
}

func equalRecords(a, b []game.ItemRecord) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Quem saiu há pouco volta pelo z.recent, que vale mais que o banco: a ação tem que mudar
// o recent (e salvá-lo), senão o próximo login a perderia (dar) ou duplicaria (tirar).
func TestAdminEditRecentChangesTheStateTheNextLoginUses(t *testing.T) {
	saves := make(chan []SavedState, 64)
	z := newTestZone(saves)
	p, _ := joinKit(z, 1, "knight", 0, 0, 100)
	p.pl.inv.add(game.Items[potion], 5)
	potions := p.pl.inv.countOf(potion)
	z.removePlayer(1)
	leave := lastSaved(t, saves, 1)

	r := z.adminEdit(1, AdminEdit{Kind: EditGiveItem, Item: "scroll_return", Count: 2})
	if r.Err != nil || r.Where != WhereRecent {
		t.Fatalf("give to recent = %+v", r)
	}
	r = z.adminEdit(1, AdminEdit{Kind: EditTakeItem, Item: potion, Count: potions})
	if r.Err != nil || r.Where != WhereRecent {
		t.Fatalf("take from recent = %+v", r)
	}
	if r := z.adminEdit(1, AdminEdit{Kind: EditGold, Gold: 25}); r.Err != nil {
		t.Fatal(r.Err)
	}
	st := lastSaved(t, saves, 1)
	if st.Seq <= leave.Seq || countItem(st.Items, "scroll_return") != 2 || countItem(st.Items, potion) != 0 || st.Gold != 125 {
		t.Fatalf("saved recent = %+v (leave seq %d)", st, leave.Seq)
	}
	if st.Storage != nil {
		t.Fatal("the storage did not change and must not be saved again")
	}

	// O login carrega do banco o estado de antes (o salvamento pode nem ter terminado): vale
	// o recent, com a ação.
	joinInfo(z, JoinInfo{ID: 1, Name: "Kknight", Gold: 100, Items: leave.Items, LoadedAt: time.Now()})
	q := z.players[1]
	if q == nil {
		t.Fatal("the player must join")
	}
	if q.pl.inv.countOf("scroll_return") != 2 || q.pl.inv.countOf(potion) != 0 || q.pl.inv.gold != 125 {
		t.Fatalf("after relog: scrolls %d, potions %d, gold %d", q.pl.inv.countOf("scroll_return"), q.pl.inv.countOf(potion), q.pl.inv.gold)
	}
}

// Offline de verdade: a zona trava o personagem; o login é recusado durante a transação e
// também quando o estado foi lido do banco antes de ela terminar.
func TestAdminEditOfflineHoldsAndRefusesStaleJoins(t *testing.T) {
	z := newTestZone(make(chan []SavedState, 64))
	loadedBefore := time.Now()

	r := z.adminEdit(7, AdminEdit{Kind: EditGiveItem, Item: potion, Count: 1, AccountID: 70})
	if r.Err != nil || r.Where != WhereOffline || r.Hold == 0 {
		t.Fatalf("offline = %+v", r)
	}
	// Uma segunda ação no mesmo personagem espera a primeira.
	if r2 := z.adminEdit(7, AdminEdit{Kind: EditGold, Gold: 5}); !errors.Is(r2.Err, ErrCharacterBusy) {
		t.Fatalf("second action while held = %+v", r2)
	}

	// Durante a transação: recusado, mesmo carregado agora.
	peer := joinInfo(z, JoinInfo{ID: 7, Name: "Off", AccountID: 70, LoadedAt: time.Now()})
	if z.players[7] != nil || peer.closed != adminStaleReason {
		t.Fatalf("join while held: online=%v closed=%q", z.players[7] != nil, peer.closed)
	}

	// Depois: o Join carregado antes da mudança é recusado; o carregado depois entra.
	z.handle(socialDone{fn: func() {
		a := z.admin()
		delete(a.holds, 7)
		a.edits[7] = time.Now()
	}})
	peer = joinInfo(z, JoinInfo{ID: 7, Name: "Off", AccountID: 70, LoadedAt: loadedBefore})
	if z.players[7] != nil || peer.closed == "" {
		t.Fatal("a join loaded before the change must be refused")
	}
	if peer.has(func(m *pb.ServerMessage) bool { return m.GetKicked() != nil }) {
		t.Fatal("no Kicked: the client reconnects by itself")
	}
	joinInfo(z, JoinInfo{ID: 7, Name: "Off", AccountID: 70, LoadedAt: time.Now()})
	if z.players[7] == nil {
		t.Fatal("a join loaded after the change goes in")
	}
}

func TestAdminReleaseThroughTheZone(t *testing.T) {
	z := newTestZone(make(chan []SavedState, 64))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { z.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	r, err := z.AdminEdit(context.Background(), 9, AdminEdit{Kind: EditTeleport, X: 1e6, Z: 5})
	if err != nil || r.Where != WhereOffline {
		t.Fatalf("AdminEdit = %+v, %v", r, err)
	}
	if r.Edit.X > z.cfg.Size { // a zona põe a posição dentro do mapa antes do banco
		t.Fatalf("teleport not clamped: %v", r.Edit.X)
	}
	if err := z.AdminRelease(context.Background(), 9, r.Hold); err != nil {
		t.Fatal(err)
	}
	live, err := z.AdminCharacter(context.Background(), 9, 0)
	if err != nil || live.Held {
		t.Fatalf("still held after release: %+v %v", live, err)
	}
	// Depois de solto, uma ação nova trava de novo (não fica "ocupado" para sempre).
	r, err = z.AdminEdit(context.Background(), 9, AdminEdit{Kind: EditGold, Gold: 1})
	if err != nil || r.Where != WhereOffline {
		t.Fatalf("second AdminEdit = %+v, %v", r, err)
	}
}

func TestAdminHoldExpiresAndCountsAsAChange(t *testing.T) {
	z := newTestZone(nil)
	loaded := time.Now().Add(-time.Second) // lido do banco antes da trava
	z.adminEdit(3, AdminEdit{Kind: EditGold, Gold: 1})
	z.admin().holds[3] = adminHold{token: 1, expires: time.Now().Add(-time.Millisecond)} // a API sumiu
	peer := joinInfo(z, JoinInfo{ID: 3, Name: "Vence", LoadedAt: loaded})
	if z.players[3] != nil || peer.closed == "" {
		t.Fatal("a join loaded before the hold expired is refused")
	}
	joinInfo(z, JoinInfo{ID: 3, Name: "Vence", LoadedAt: time.Now()})
	if z.players[3] == nil {
		t.Fatal("after the expired hold, a fresh join goes in")
	}
}

func TestApplyOfflineEditUsesTheBagRules(t *testing.T) {
	kit := game.StarterKit("knight")
	st := OfflineState{Gold: 10, Items: kit}
	res, ev, err := ApplyOfflineEdit(&st, AdminEdit{Kind: EditGiveItem, Item: potion, Count: 4})
	if err != nil || res.Count != 4 || !st.ItemsChanged || countItem(st.Items, potion) != countItem(kit, potion)+4 {
		t.Fatalf("give = %+v %v %+v", res, err, st.Items)
	}
	if len(ev) != 1 || ev[0].Kind != "item_in" || ev[0].Ref != potion || ev[0].Amount != 4 {
		t.Fatalf("telemetry = %+v", ev)
	}
	// Não tem o bastante: nada muda.
	before := st
	if _, _, err := ApplyOfflineEdit(&st, AdminEdit{Kind: EditTakeItem, Item: "scroll_return", Count: 1}); err == nil {
		t.Fatal("take of an item the character does not have must fail")
	}
	if !equalRecords(before.Items, st.Items) {
		t.Fatal("a failed take changed the items")
	}
	// Tira do equipamento quando a bolsa não tem.
	st = OfflineState{Items: kit}
	if _, _, err := ApplyOfflineEdit(&st, AdminEdit{Kind: EditTakeItem, Item: "sword_common", Count: 1}); err != nil {
		t.Fatal(err)
	}
	if countItem(st.Items, "sword_common") != countItem(kit, "sword_common")-1 {
		t.Fatalf("equipped sword not taken: %+v", st.Items)
	}
	// Moedas e posição não mexem nos itens.
	st = OfflineState{Gold: 5, Items: kit}
	if _, _, err := ApplyOfflineEdit(&st, AdminEdit{Kind: EditGold, Gold: -6}); err == nil || st.Gold != 5 {
		t.Fatal("taking more gold than there is must fail and change nothing")
	}
	if _, ev, err := ApplyOfflineEdit(&st, AdminEdit{Kind: EditGold, Gold: 7}); err != nil || st.Gold != 12 || st.ItemsChanged || ev[0].Kind != "gold" {
		t.Fatalf("gold = %d %v %+v", st.Gold, err, ev)
	}
	if _, _, err := ApplyOfflineEdit(&st, AdminEdit{Kind: EditTeleport, X: 3, Z: 4}); err != nil || st.X != 3 || st.Z != 4 || st.ItemsChanged {
		t.Fatal("teleport offline")
	}
	// Bolsa cheia.
	var full []game.ItemRecord
	for i := range game.BagSize {
		full = append(full, game.ItemRecord{Slot: i, ItemID: "sword_common", Count: 1})
	}
	st = OfflineState{Items: full}
	if _, _, err := ApplyOfflineEdit(&st, AdminEdit{Kind: EditGiveItem, Item: "axe_common", Count: 1}); err == nil {
		t.Fatal("a full bag must refuse")
	}
	// Item que o jogo não reconhece: não regrava a bolsa (apagaria o item).
	st = OfflineState{Items: append(kit[:len(kit):len(kit)], game.ItemRecord{Slot: 30, ItemID: "item_que_saiu", Count: 1})}
	if _, _, err := ApplyOfflineEdit(&st, AdminEdit{Kind: EditGiveItem, Item: potion, Count: 1}); err == nil {
		t.Fatal("unknown items in the saved bag must block the item edit")
	}
}

func TestAdminAccountChangeAppliesOnlineAndRefusesStaleJoins(t *testing.T) {
	z := newTestZone(nil)
	loaded := time.Now()
	joinInfo(z, JoinInfo{ID: 1, Name: "Ana", AccountID: 10, LoadedAt: loaded})
	peerB := joinInfo(z, JoinInfo{ID: 2, Name: "Bia", AccountID: 10, LoadedAt: loaded})
	joinInfo(z, JoinInfo{ID: 3, Name: "Caio", AccountID: 11, LoadedAt: loaded})

	until := time.Now().Add(30 * time.Minute)
	names := z.accountChanged(AccountChange{AccountID: 10, Mute: true, MutedUntil: until, Reason: "spam"})
	if strings.Join(names, ",") != "Ana,Bia" {
		t.Fatalf("names = %v", names)
	}
	if !z.players[1].pl.mutedUntil.Equal(until) || !z.players[2].pl.mutedUntil.Equal(until) || !z.players[3].pl.mutedUntil.IsZero() {
		t.Fatal("mute applies to every character of the account, and only to them")
	}
	if !strings.Contains(lastChat(peerB), "spam") {
		t.Fatalf("the player is told why: %q", lastChat(peerB))
	}

	// GM liga na hora.
	z.accountChanged(AccountChange{AccountID: 11, SetGM: true, GM: true})
	if !z.players[3].pl.gm {
		t.Fatal("GM applies to who is online")
	}

	// Banir derruba todos da conta.
	z.accountChanged(AccountChange{AccountID: 10, Ban: true, Reason: "dup"})
	if z.players[1] != nil || z.players[2] != nil || z.players[3] == nil {
		t.Fatal("ban kicks every character of the account")
	}
	if !peerB.has(func(m *pb.ServerMessage) bool { k := m.GetKicked(); return k != nil && strings.Contains(k.Reason, "dup") }) {
		t.Fatal("the banned player sees the reason")
	}
	// Um Join da conta carregado antes da mudança é recusado (o banco já mudou).
	peer := joinInfo(z, JoinInfo{ID: 1, Name: "Ana", AccountID: 10, LoadedAt: loaded})
	if z.players[1] != nil || peer.closed == "" {
		t.Fatal("a join of the account loaded before the change must be refused")
	}
}

func lastChat(p *fakePeer) string {
	c := p.chats()
	if len(c) == 0 {
		return ""
	}
	return c[len(c)-1].Text
}

func TestAdminKickAndLiveStateThroughTheZone(t *testing.T) {
	saves := make(chan []SavedState, 64)
	z := newTestZone(saves)
	peer := joinInfo(z, JoinInfo{ID: 1, Name: "Kknight", X: 5, Z: 5, Gold: 42, AccountID: 9, Items: game.StarterKit("knight")})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { z.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	live, err := z.AdminCharacter(context.Background(), 1, 9)
	if err != nil || !live.Online || live.Gold != 42 || live.Storage == nil {
		t.Fatalf("live online = %+v, %v", live, err)
	}
	list, err := z.AdminOnline(context.Background())
	if err != nil || len(list) != 1 || list[0].Region != "Cidade" {
		t.Fatalf("online = %+v, %v", list, err)
	}
	name, err := z.AdminKick(context.Background(), 1, "teste")
	if err != nil || name != "Kknight" {
		t.Fatalf("kick = %q, %v", name, err)
	}
	if !peer.has(func(m *pb.ServerMessage) bool { k := m.GetKicked(); return k != nil && strings.Contains(k.Reason, "teste") }) {
		t.Fatal("kicked with the reason")
	}
	if _, err := z.AdminKick(context.Background(), 1, ""); !errors.Is(err, ErrNotOnline) {
		t.Fatalf("kick offline = %v", err)
	}
	// Saiu: a ficha vem do estado recente.
	live, err = z.AdminCharacter(context.Background(), 1, 9)
	if err != nil || live.Online || !live.Recent || live.Gold != 42 {
		t.Fatalf("live recent = %+v, %v", live, err)
	}
}

// A zona pegou o pedido mas não respondeu: "não confirmado", nunca "deu errado" (que
// levaria o GM a repetir e duplicar).
func TestAskZoneActionUnconfirmedWhenTheZoneHangs(t *testing.T) {
	old := adminActionWait
	adminActionWait = 50 * time.Millisecond
	defer func() { adminActionWait = old }()
	z := newTestZone(nil) // sem Run: o pedido entra na fila e ninguém responde
	if _, err := z.AdminEdit(context.Background(), 1, AdminEdit{Kind: EditGold, Gold: 1}); !errors.Is(err, ErrZoneUnconfirmed) {
		t.Fatalf("err = %v, want ErrZoneUnconfirmed", err)
	}
	for len(z.events) < cap(z.events) {
		z.events <- socialDone{fn: func() {}}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := z.AdminEdit(ctx, 1, AdminEdit{Kind: EditGold, Gold: 1}); !errors.Is(err, ErrZoneBusy) {
		t.Fatalf("full queue: err = %v, want ErrZoneBusy (nothing done)", err)
	}
}

func TestAdminEditValidates(t *testing.T) {
	z := newTestZone(nil)
	joinKit(z, 1, "knight", 0, 0, 0)
	for _, e := range []AdminEdit{
		{Kind: EditGiveItem, Item: "nao_existe", Count: 1},
		{Kind: EditGiveItem, Item: potion, Count: 0},
		{Kind: EditGiveItem, Item: "gold", Count: 1},
		{Kind: EditGold, Gold: 0},
		{Kind: EditGold, Gold: maxAdminGold + 1},
		{Kind: "nada"},
	} {
		var inv *AdminInvalid
		if r := z.adminEdit(1, e); !errors.As(r.Err, &inv) {
			t.Errorf("%+v: err = %v, want AdminInvalid", e, r.Err)
		}
	}
}
