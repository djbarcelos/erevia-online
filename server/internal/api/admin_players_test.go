package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/zone"
)

// liveEnv: o portal com uma zona de verdade rodando e a persistência gravando no banco
// (como o cmd/zone faz), para as ações online, "recente" e offline.
type liveEnv struct {
	*adminEnv
	z *zone.Zone
}

func newLiveEnv(t *testing.T) *liveEnv {
	t.Helper()
	cfg := zone.DefaultConfig()
	cfg.Spawns, cfg.InvasionChance, cfg.XP2Days = nil, 0, nil
	saves := make(chan []zone.SavedState, 256)
	z := zone.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), saves)
	e := newAdminEnv(t, AdminConfig{Zone: z, WriteBurst: 1000, WritePerSec: 1000, ReadBurst: 1000, ReadPerSec: 1000})
	z.SetEventSink(eventSink{e.st})
	persisted := make(chan struct{})
	go func() {
		defer close(persisted)
		for batch := range saves {
			states := make([]store.CharacterState, len(batch))
			for i, p := range batch {
				states[i] = store.CharacterState{
					CharacterID: int64(p.ID), X: p.X, Z: p.Z, Level: p.Level, XP: p.XP, HP: p.HP, MP: p.MP,
					Gold: p.Gold, Items: p.Items, Attrs: p.Attrs, Ranks: p.Ranks, Hotbar: p.Hotbar, Seq: p.Seq,
					Karma: p.Karma, PKs: p.PKs, PvPKills: p.PvPKills, Appearance: p.Look, Quests: p.Quests,
					Achievements: p.Achievements, Title: p.Title, Spec: p.Spec,
				}
				if s := p.Storage; s != nil {
					states[i].Storage = &store.StorageState{AccountID: s.AccountID, Gold: s.Gold, Items: s.Items, Seq: s.Seq}
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := e.st.SaveStates(ctx, states); err != nil {
				t.Errorf("save states: %v", err)
			}
			cancel()
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { z.Run(ctx); close(done) }()
	// Antes do store fechar (o Cleanup do storetest roda depois deste).
	t.Cleanup(func() { cancel(); <-done; <-persisted })
	return &liveEnv{adminEnv: e, z: z}
}

// eventSink grava a telemetria da zona no banco do teste (sessões da ficha).
type eventSink struct{ st *store.Store }

func (s eventSink) InsertEvents(ctx context.Context, events []zone.GameEvent) error {
	rows := make([]store.GameEvent, len(events))
	for i, ev := range events {
		rows[i] = store.GameEvent(ev)
	}
	return s.st.InsertEvents(ctx, rows)
}

type testPeer struct {
	mu     sync.Mutex
	closed string
}

func (p *testPeer) Send([]byte) bool { return true }
func (p *testPeer) Close(reason string) {
	p.mu.Lock()
	p.closed = reason
	p.mu.Unlock()
}

func (p *testPeer) closedReason() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}

// newChar cria conta e personagem (offline).
func (e *liveEnv) newChar(user, name string) (int64, int64) {
	e.t.Helper()
	ctx := context.Background()
	acc, err := e.st.LoginIdentity(ctx, "dev", strings.ToLower(user), user)
	if err != nil {
		e.t.Fatal(err)
	}
	c, err := e.st.CreateCharacter(ctx, acc, name, "knight", game.DefaultLook("knight"))
	if err != nil {
		e.t.Fatal(err)
	}
	return acc, c.ID
}

// sync espera a zona processar tudo o que já está na fila.
func (e *liveEnv) sync() {
	e.t.Helper()
	if _, err := e.z.Status(context.Background()); err != nil {
		e.t.Fatal(err)
	}
}

// join faz o caminho do WebSocket: anota a hora, carrega do banco e entra na zona.
// Devolve o peer e se entrou (a zona pode recusar um estado lido antes de uma ação).
func (e *liveEnv) join(acc, id int64) (*testPeer, bool) {
	e.t.Helper()
	loadedAt := time.Now()
	c, err := e.st.CharacterForAccount(context.Background(), acc, id)
	if err != nil {
		e.t.Fatal(err)
	}
	return e.enter(c, loadedAt)
}

// enter entra na zona com um personagem já carregado (lido do banco em loadedAt).
func (e *liveEnv) enter(c store.Character, loadedAt time.Time) (*testPeer, bool) {
	e.t.Helper()
	p := &testPeer{}
	e.z.Join(context.Background(), zone.JoinInfo{
		ID: uint32(c.ID), Name: c.Name, Class: c.Class, X: c.X, Z: c.Z, XP: c.XP, HP: c.HP, MP: c.MP, Gold: c.Gold, Items: c.Items,
		Attrs: c.Attrs, Ranks: c.Ranks, Hotbar: c.Hotbar, Emblem: c.Emblem, Look: c.Appearance, AccountID: c.AccountID,
		Karma: c.Karma, PKs: c.PKs, PvPKills: c.PvPKills,
		Storage: &zone.StorageData{Gold: c.Storage.Gold, Items: c.Storage.Items, Seq: c.Storage.Seq},
		GM:      c.GM, Quests: c.Quests, Achievements: c.Achievements, Title: c.Title, Spec: c.Spec,
		LoadedAt: loadedAt, Peer: p,
	})
	e.sync()
	return p, p.closedReason() == ""
}

func (e *liveEnv) leave(id int64, p *testPeer) {
	e.z.Leave(context.Background(), uint32(id), p)
	e.sync()
}

// dbCount: unidades do item no banco (bolsa e equipamento).
func (e *liveEnv) dbCount(acc, id int64, item string) (int, int64) {
	e.t.Helper()
	c, err := e.st.CharacterForAccount(context.Background(), acc, id)
	if err != nil {
		e.t.Fatal(err)
	}
	n := 0
	for _, it := range c.Items {
		if it.ItemID == item {
			n += it.Count
		}
	}
	return n, c.Gold
}

// eventually espera o banco chegar ao valor (a persistência é assíncrona) e confere que
// ele fica lá (nada chega depois e desfaz).
func eventually(t *testing.T, what string, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !f() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout: %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(150 * time.Millisecond)
	if !f() {
		t.Fatalf("changed back: %s", what)
	}
}

type actionResp struct {
	OK      bool
	Where   string
	Message string
	Error   string
}

func (c *adminClient) post(path, body string) (int, actionResp) {
	c.env.t.Helper()
	resp := c.req("POST", path, body, adminHdr)
	var out actionResp
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (c *adminClient) getJSON(path string, out any) int {
	c.env.t.Helper()
	resp := c.req("GET", path, "", nil)
	json.NewDecoder(resp.Body).Decode(out)
	return resp.StatusCode
}

const potion = "potion_hp_s"

func TestAdminItemsOnlineRecentAndOfflineNeverDuplicate(t *testing.T) {
	e := newLiveEnv(t)
	gm := e.login("Mestre", true)
	acc, id := e.newChar("Jogador", "Heroi")
	base, gold0 := e.dbCount(acc, id, potion)
	path := fmt.Sprintf("/api/admin/players/%d", id)

	// Offline: vai pelo banco, na transação.
	code, r := gm.post(path+"/item", `{"action":"give","item":"potion_hp_s","count":3,"reason":"teste"}`)
	if code != http.StatusOK || r.Where != zone.WhereOffline {
		t.Fatalf("give offline = %d %+v", code, r)
	}
	if n, _ := e.dbCount(acc, id, potion); n != base+3 {
		t.Fatalf("db after offline give = %d, want %d", n, base+3)
	}
	code, r = gm.post(path+"/gold", `{"amount":500}`)
	if code != http.StatusOK || r.Where != zone.WhereOffline {
		t.Fatalf("gold offline = %d %+v", code, r)
	}
	// Telemetria na mesma transação.
	var in int64
	if err := e.st.QueryRowForTest(context.Background(), fmt.Sprintf(
		`SELECT coalesce(sum(amount), 0) FROM game_events WHERE character_id = %d AND kind = 'item_in' AND ref = 'potion_hp_s'`, id), &in); err != nil || in != 3 {
		t.Fatalf("item_in telemetry = %d, %v", in, err)
	}

	// Entrar: o login carrega do banco já com a ação.
	p, ok := e.join(acc, id)
	if !ok {
		t.Fatalf("join refused: %q", p.closedReason())
	}
	var sheet struct {
		Online bool
		Source string
		Gold   int64
		Bag    []struct {
			ID    string
			Count int
		}
	}
	gm.getJSON(path, &sheet)
	if !sheet.Online || sheet.Source != "zona" || sheet.Gold != gold0+500 {
		t.Fatalf("sheet online = %+v", sheet)
	}

	// Online: na memória da zona, e salvo na hora (o próximo salvamento não desfaz).
	code, r = gm.post(path+"/item", `{"action":"give","item":"potion_hp_s","count":2}`)
	if code != http.StatusOK || r.Where != zone.WhereOnline {
		t.Fatalf("give online = %d %+v", code, r)
	}
	eventually(t, "online give saved", func() bool { n, _ := e.dbCount(acc, id, potion); return n == base+5 })
	code, r = gm.post(path+"/item", `{"action":"take","item":"potion_hp_s","count":1}`)
	if code != http.StatusOK || r.Where != zone.WhereOnline {
		t.Fatalf("take online = %d %+v", code, r)
	}
	e.leave(id, p)
	eventually(t, "leave keeps the online changes", func() bool { n, _ := e.dbCount(acc, id, potion); return n == base+4 })

	// Saiu há pouco: o estado recente vale mais que o banco; a ação vai nele.
	code, r = gm.post(path+"/item", `{"action":"take","item":"potion_hp_s","count":4}`)
	if code != http.StatusOK || r.Where != zone.WhereRecent {
		t.Fatalf("take recent = %d %+v", code, r)
	}
	p, ok = e.join(acc, id)
	if !ok {
		t.Fatalf("join after recent: %q", p.closedReason())
	}
	e.leave(id, p)
	eventually(t, "recent take survives relog", func() bool { n, _ := e.dbCount(acc, id, potion); return n == base })

	// Pedido inválido: 400 e nada muda.
	if code, r := gm.post(path+"/item", `{"action":"take","item":"potion_hp_s","count":999}`); code != http.StatusBadRequest || r.Error == "" {
		t.Fatalf("take too many = %d %+v", code, r)
	}
	if code, _ := gm.post(path+"/item", `{"action":"give","item":"nada","count":1}`); code != http.StatusBadRequest {
		t.Fatalf("unknown item = %d", code)
	}

	// Tudo no gm_log, com o alvo.
	hist, _ := e.st.ModerationHistory(context.Background(), acc)
	var cmds []string
	for _, h := range hist {
		cmds = append(cmds, h.Command)
		if h.Target != "Heroi" {
			t.Fatalf("history target = %+v", h)
		}
	}
	got := strings.Join(cmds, ",")
	for _, want := range []string{"portal:give", "portal:take", "portal:take:falhou"} {
		if !strings.Contains(got, want) {
			t.Fatalf("history %s lacks %s", got, want)
		}
	}
}

// O login leu o banco antes da ação offline e só chega à zona depois dela: a zona recusa
// (o cliente reconecta e lê de novo). Sem isso, ele entraria sem o item e o salvaria por
// cima (dar = perdido; tirar = duplicado).
func TestAdminOfflineActionRefusesALoginReadBeforeIt(t *testing.T) {
	e := newLiveEnv(t)
	gm := e.login("Mestre", true)
	acc, id := e.newChar("Lento", "Lento")
	base, _ := e.dbCount(acc, id, potion)

	loadedAt := time.Now()
	stale, err := e.st.CharacterForAccount(context.Background(), acc, id)
	if err != nil {
		t.Fatal(err)
	}
	code, r := gm.post(fmt.Sprintf("/api/admin/players/%d/item", id), `{"action":"take","item":"potion_hp_s","count":1}`)
	if code != http.StatusOK || r.Where != zone.WhereOffline {
		t.Fatalf("take offline = %d %+v", code, r)
	}
	if p, ok := e.enter(stale, loadedAt); ok || p.closedReason() == "" {
		t.Fatal("a login read before the offline action must be refused")
	}
	p, ok := e.join(acc, id) // a reconexão lê de novo
	if !ok {
		t.Fatalf("fresh join refused: %q", p.closedReason())
	}
	e.leave(id, p)
	eventually(t, "the take stays taken", func() bool { n, _ := e.dbCount(acc, id, potion); return n == base-1 })
}

// Um login que corre junto com as ações nunca entra com o estado velho: ao fim,
// o banco tem exatamente o que as ações confirmadas deram e tiraram.
func TestAdminOfflineActionsRacingLoginsKeepEveryItem(t *testing.T) {
	e := newLiveEnv(t)
	gm := e.login("Mestre", true)
	acc, id := e.newChar("Corredor", "Corredor")
	base, _ := e.dbCount(acc, id, potion)
	path := fmt.Sprintf("/api/admin/players/%d/item", id)

	const pairs = 12
	var wg sync.WaitGroup
	stop := make(chan struct{})
	refused := 0
	wg.Add(1)
	go func() { // entra e sai sem parar
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			p, ok := e.join(acc, id)
			if !ok {
				refused++
				continue
			}
			time.Sleep(3 * time.Millisecond)
			e.leave(id, p)
		}
	}()
	where := map[string]int{}
	do := func(body string) {
		for attempt := 0; attempt < 50; attempt++ {
			code, r := gm.post(path, body)
			if code == http.StatusOK {
				where[r.Where]++
				return
			}
			if code != http.StatusConflict { // 409: outra ação terminando, nada feito
				t.Errorf("%s = %d %+v", body, code, r)
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Errorf("%s: always busy", body)
	}
	for i := 0; i < pairs; i++ {
		do(`{"action":"give","item":"potion_hp_s","count":2}`)
		do(`{"action":"take","item":"potion_hp_s","count":1}`)
	}
	close(stop)
	wg.Wait()
	e.sync()
	eventually(t, "every confirmed action is in the db exactly once", func() bool {
		n, _ := e.dbCount(acc, id, potion)
		return n == base+pairs
	})
	t.Logf("actions by place: %v; joins refused: %d", where, refused)
}

func TestAdminSearchSheetAndModeration(t *testing.T) {
	e := newLiveEnv(t)
	gm := e.login("Mestre", true)
	gmAcc, _ := e.st.LoginIdentity(context.Background(), "dev", "mestre", "Mestre")
	acc, id := e.newChar("Alvo", "Brigao")
	_, id2 := e.newChar("Quieto", "Calmo")
	path := fmt.Sprintf("/api/admin/players/%d", id)
	p, _ := e.join(acc, id)

	var list struct {
		Players []struct {
			ID     int64
			Name   string
			Online bool
		}
		Online int
	}
	gm.getJSON("/api/admin/players", &list)
	if len(list.Players) != 1 || list.Players[0].Name != "Brigao" || !list.Players[0].Online {
		t.Fatalf("online list = %+v", list)
	}
	for _, q := range []string{"brig", fmt.Sprint(acc), "dev:alvo", "Alvo"} {
		gm.getJSON("/api/admin/players?q="+q, &list)
		if len(list.Players) == 0 || list.Players[0].ID != id || !list.Players[0].Online {
			t.Fatalf("search %q = %+v", q, list)
		}
	}
	gm.getJSON("/api/admin/players?q=calm", &list)
	if len(list.Players) != 1 || list.Players[0].ID != id2 || list.Players[0].Online {
		t.Fatalf("offline search = %+v", list)
	}

	// Silenciar: banco e zona.
	if code, r := gm.post(path+"/mute", `{"minutes":30,"reason":"flood"}`); code != http.StatusOK {
		t.Fatalf("mute = %d %+v", code, r)
	}
	var sheet struct {
		Name    string
		Online  bool
		Region  string
		Equipped []struct{ ID, Where string }
		Account struct {
			ID         int64
			GM, Banned bool
			BanReason  string
			MutedUntil *time.Time
			Identities []store.Identity
			Characters []store.AltChar
		}
		Sessions []store.Session
		History  []store.GMLogRow
	}
	gm.getJSON(path, &sheet)
	if sheet.Account.MutedUntil == nil || time.Until(*sheet.Account.MutedUntil) < 29*time.Minute || !sheet.Online || sheet.Region != "Cidade" {
		t.Fatalf("sheet after mute = %+v", sheet)
	}
	if len(sheet.Equipped) == 0 || len(sheet.Account.Identities) != 1 || len(sheet.Account.Characters) != 1 {
		t.Fatalf("sheet details = %+v", sheet)
	}
	// A sessão aberta chega pela telemetria (gravada em lote, a cada 2 s).
	deadline := time.Now().Add(6 * time.Second)
	for gm.getJSON(path, &sheet); len(sheet.Sessions) == 0 || sheet.Sessions[0].End != nil; gm.getJSON(path, &sheet) {
		if time.Now().After(deadline) {
			t.Fatalf("sessions = %+v", sheet.Sessions)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if code, _ := gm.post(path+"/unmute", `{}`); code != http.StatusOK {
		t.Fatal("unmute")
	}
	if code, _ := gm.post(path+"/mute", `{"minutes":0}`); code != http.StatusBadRequest {
		t.Fatal("mute 0 min is invalid")
	}

	// Teleporte online.
	if code, r := gm.post(path+"/teleport", `{"x":60,"z":10}`); code != http.StatusOK || r.Where != zone.WhereOnline {
		t.Fatalf("teleport = %d %+v", code, r)
	}
	gm.getJSON(path, &sheet)
	if sheet.Region != "Arredores" {
		t.Fatalf("region after teleport = %q", sheet.Region)
	}

	// GM liga e desliga; não dá para tirar o próprio GM nem se banir.
	if code, _ := gm.post(path+"/gm", `{"gm":true}`); code != http.StatusOK {
		t.Fatal("gm on")
	}
	gm.getJSON(path, &sheet)
	if !sheet.Account.GM {
		t.Fatal("gm on not shown")
	}
	gm.post(path+"/gm", `{"gm":false}`)
	_, gmChar := e.newChar("Mestre", "Mestrao")
	if code, _ := gm.post(fmt.Sprintf("/api/admin/players/%d/gm", gmChar), `{"gm":false}`); code != http.StatusBadRequest {
		t.Fatal("removing your own GM must be refused")
	}
	if code, _ := gm.post(fmt.Sprintf("/api/admin/players/%d/ban", gmChar), `{"reason":"x"}`); code != http.StatusBadRequest {
		t.Fatal("banning yourself must be refused")
	}
	_ = gmAcc

	// Expulsar: só online.
	if code, _ := gm.post(path+"/kick", `{"reason":"teste"}`); code != http.StatusOK {
		t.Fatal("kick")
	}
	if p.closedReason() == "" {
		t.Fatal("the connection is closed")
	}
	if code, _ := gm.post(path+"/kick", `{}`); code != http.StatusConflict {
		t.Fatal("kick offline = 409")
	}

	// Banir pede motivo; banido não autentica; desbanir.
	if code, _ := gm.post(path+"/ban", `{"reason":""}`); code != http.StatusBadRequest {
		t.Fatal("ban without reason")
	}
	if code, _ := gm.post(path+"/ban", `{"reason":"duplicação"}`); code != http.StatusOK {
		t.Fatal("ban")
	}
	gm.getJSON(path, &sheet)
	if !sheet.Account.Banned || sheet.Account.BanReason != "duplicação" {
		t.Fatalf("ban not in db: %+v", sheet.Account)
	}
	if code, _ := gm.post(path+"/unban", `{"reason":"recurso aceito"}`); code != http.StatusOK {
		t.Fatal("unban")
	}

	// Histórico de moderação: as ações do portal e as do jogo (/kick no chat) contra a conta.
	e.st.LogGM(context.Background(), store.GMEntry{GMAccountID: 1, GMName: "Outro", Command: "kick", Target: "brigao"})
	gm.getJSON(path, &sheet)
	var cmds []string
	for _, h := range sheet.History {
		cmds = append(cmds, h.Command)
	}
	got := strings.Join(cmds, ",")
	for _, want := range []string{"kick", "portal:ban", "portal:unban", "portal:kick", "portal:mute", "portal:unmute", "portal:tp", "portal:gm:on"} {
		if !strings.Contains(","+got+",", ","+want+",") {
			t.Fatalf("history %q lacks %s", got, want)
		}
	}
	if strings.Contains(got, "Mestrao") {
		t.Fatal("history of another account leaked")
	}
}

func TestAdminPlayerRoutesHiddenAndNeedHeader(t *testing.T) {
	e := newLiveEnv(t)
	player := e.login("Jogador", false)
	gm := e.login("Mestre", true)
	_, id := e.newChar("Alvo", "Alvinho")
	want := readBody(player.req("GET", "/api/nada-aqui", "", nil))
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/admin/players"},
		{"GET", fmt.Sprintf("/api/admin/players/%d", id)},
		{"GET", "/api/admin/items"},
		{"GET", "/api/admin/ops"},
		{"POST", fmt.Sprintf("/api/admin/players/%d/item", id)},
		{"POST", "/api/admin/ops/announce"},
	} {
		resp := player.req(tc.method, tc.path, `{}`, adminHdr)
		if body := readBody(resp); resp.StatusCode != http.StatusNotFound || body != want {
			t.Errorf("non-GM %s %s = %d", tc.method, tc.path, resp.StatusCode)
		}
	}
	if code := gm.req("POST", fmt.Sprintf("/api/admin/players/%d/gold", id), `{"amount":5}`, nil).StatusCode; code != http.StatusForbidden {
		t.Fatalf("write without header = %d", code)
	}
	var items struct{ Items []struct{ ID, Name string } }
	if gm.getJSON("/api/admin/items", &items); len(items.Items) < 10 {
		t.Fatalf("items = %d", len(items.Items))
	}
}
