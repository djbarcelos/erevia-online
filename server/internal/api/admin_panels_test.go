package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
	"erevia/server/internal/zone"
)

// liveZone é uma zona falsa que também responde o painel ao vivo.
type liveZone struct {
	fakeZone
	live zone.LiveStats
}

func (l liveZone) LiveStats(context.Context) (zone.LiveStats, error) { return l.live, l.err }

// newPanelsEnv é o newAdminEnv com uma conexão direta ao schema do teste (para forjar dados).
func newPanelsEnv(t *testing.T, cfg AdminConfig) (*adminEnv, *pgx.Conn) {
	t.Helper()
	st, url := storetest.NewWithURL(t)
	a := New(st, Config{PublicURL: "http://example.test", DevLogin: true}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	a.SetAdmin(cfg)
	mux := http.NewServeMux()
	a.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	db, err := pgx.Connect(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(context.Background()) })
	return &adminEnv{t: t, st: st, api: a, srv: srv}, db
}

func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	var v T
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s = %d %s", resp.Request.URL.Path, resp.StatusCode, readBody(resp))
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestAdminPanelsHiddenFromNonGM(t *testing.T) {
	e := newAdminEnv(t, AdminConfig{})
	anon := e.login("", false)
	player := e.login("Jogador", false)
	want := readBody(anon.req("GET", "/api/nada-aqui", "", nil))
	for _, c := range []*adminClient{anon, player} {
		for _, tc := range []struct{ method, path, body string }{
			{"GET", "/api/admin/bugs", ""},
			{"GET", "/api/admin/bugs/1", ""},
			{"POST", "/api/admin/bugs/1/resolve", `{"note":"x"}`},
			{"GET", "/api/admin/client-errors", ""},
			{"GET", "/api/admin/metrics", ""},
			{"GET", "/api/admin/metrics/live", ""},
			{"GET", "/api/admin/economy", ""},
			{"GET", "/api/admin/economy/character?name=x", ""},
			{"GET", "/api/admin/economy/gm-log", ""},
		} {
			resp := c.req(tc.method, tc.path, tc.body, adminHdr)
			if body := readBody(resp); resp.StatusCode != http.StatusNotFound || body != want {
				t.Errorf("%s %s = %d %q, want the plain 404", tc.method, tc.path, resp.StatusCode, body)
			}
		}
	}
}

func TestAdminBugsListResolveAudited(t *testing.T) {
	e := newAdminEnv(t, AdminConfig{})
	gm := e.login("Mestre", true)
	ctx := context.Background()
	acc, _ := e.st.LoginIdentity(ctx, "dev", "ana", "Ana")
	c, err := e.st.CreateCharacter(ctx, acc, "Ana", "knight", game.DefaultLook("knight"))
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"porta travada", "monstro na parede"} {
		if err := e.st.SaveBugReport(ctx, store.BugReport{AccountID: acc, CharacterID: c.ID, CharacterName: "Ana", Level: 3,
			X: 60, Z: 5, Version: "abc1234def", UserAgent: "Mozilla/5.0 Chrome/131", Text: text}); err != nil {
			t.Fatal(err)
		}
	}
	type list struct {
		Bugs   []store.BugRow
		Facets *store.BugFacets
	}
	l := decodeJSON[list](t, gm.req("GET", "/api/admin/bugs?region=Arredores&q=porta", "", nil))
	if len(l.Bugs) != 1 || l.Bugs[0].Text != "porta travada" || l.Facets == nil || l.Facets.Open != 2 {
		t.Fatalf("list = %+v", l)
	}
	id := strconv.FormatInt(l.Bugs[0].ID, 10)
	d := decodeJSON[struct {
		Bug   store.BugRow
		Extra map[string]json.RawMessage
	}](t, gm.req("GET", "/api/admin/bugs/"+id, "", nil))
	if d.Bug.Region != "Arredores" || d.Bug.UserAgent == "" || d.Extra == nil {
		t.Fatalf("detail = %+v", d)
	}
	if code := gm.req("GET", "/api/admin/bugs/999999", "", nil).StatusCode; code != http.StatusNotFound {
		t.Fatalf("missing = %d", code)
	}

	// Escrita: sem o cabeçalho, recusado e nada muda.
	if code := gm.req("POST", "/api/admin/bugs/"+id+"/resolve", `{"note":"ok"}`, nil).StatusCode; code != http.StatusForbidden {
		t.Fatalf("no header = %d", code)
	}
	resp := gm.req("POST", "/api/admin/bugs/"+id+"/resolve", `{"resolved":true,"note":"corrigido no abc1234"}`, adminHdr)
	got := decodeJSON[struct{ Bug store.BugRow }](t, resp)
	if !got.Bug.Resolved || got.Bug.ResolvedByName != "Mestre" || got.Bug.ResolvedNote != "corrigido no abc1234" || got.Bug.ResolvedAt == nil {
		t.Fatalf("resolved = %+v", got.Bug)
	}
	log, _ := e.st.RecentGMLog(ctx, 5)
	if len(log) != 1 || log[0].Command != "portal:bug.resolve" || log[0].Target != "bug #"+id || log[0].Detail != "corrigido no abc1234" {
		t.Fatalf("gm_log = %+v", log)
	}
	if l := decodeJSON[list](t, gm.req("GET", "/api/admin/bugs", "", nil)); len(l.Bugs) != 1 || l.Facets.Open != 1 || l.Facets.Resolved != 1 {
		t.Fatalf("open after resolve = %+v", l)
	}
	// Reabrir também fica no gm_log.
	gm.req("POST", "/api/admin/bugs/"+id+"/resolve", `{"resolved":false}`, adminHdr)
	if log, _ := e.st.RecentGMLog(ctx, 5); len(log) != 2 || log[0].Command != "portal:bug.reopen" {
		t.Fatalf("reopen gm_log = %+v", log)
	}
	if code := gm.req("POST", "/api/admin/bugs/999999/resolve", `{}`, adminHdr).StatusCode; code != http.StatusNotFound {
		t.Fatalf("resolve missing = %d", code)
	}

	errs := decodeJSON[struct {
		Errors   []store.ClientErrorRow
		Versions []store.ClientErrorVersion
	}](t, gm.req("GET", "/api/admin/client-errors?days=30", "", nil))
	if errs.Errors == nil || errs.Versions == nil {
		t.Fatalf("client errors = %+v", errs)
	}
}

func TestAdminMetricsLive(t *testing.T) {
	live := zone.LiveStats{Online: 3, ByRegion: map[string]int{"Cidade": 2, "Arredores": 1}, Monsters: 40, Ground: 7,
		LastTick: &zone.TickSnapshot{Ticks: 600, AvgMs: 1.5, WorstMs: 9, BudgetMs: 50}}
	e := newAdminEnv(t, AdminConfig{Version: "abc1234", Zone: liveZone{live: live}})
	gm := e.login("Mestre", true)
	got := decodeJSON[struct {
		Version string
		Regions []string
		Zone    *zone.LiveStats
	}](t, gm.req("GET", "/api/admin/metrics/live", "", nil))
	if got.Version != "abc1234" || len(got.Regions) != 5 || got.Zone == nil || got.Zone.Online != 3 || got.Zone.LastTick.WorstMs != 9 {
		t.Fatalf("live = %+v", got)
	}
	// Zona que não responde (ou sem o painel ao vivo): zone = null, mas a versão continua.
	for _, z := range []AdminZone{liveZone{fakeZone: fakeZone{err: zone.ErrZoneBusy}}, fakeZone{online: 1}} {
		e2 := newAdminEnv(t, AdminConfig{Version: "v2", Zone: z})
		got := decodeJSON[struct {
			Version string
			Zone    *zone.LiveStats
		}](t, e2.login("Mestre", true).req("GET", "/api/admin/metrics/live", "", nil))
		if got.Version != "v2" || got.Zone != nil {
			t.Fatalf("zone down: %+v", got)
		}
	}
}

// O painel de economia aponta o item duplicado, os gráficos ficam 1 minuto em cache, e o
// histórico do personagem e o gm_log com filtro respondem.
func TestAdminEconomyDetectsDuplicateAndCaches(t *testing.T) {
	e, db := newPanelsEnv(t, AdminConfig{})
	gm := e.login("Mestre", true)
	ctx := context.Background()
	acc, _ := e.st.LoginIdentity(ctx, "dev", "ana", "Ana")
	ana, err := e.st.CreateCharacter(ctx, acc, "Ana", "knight", game.DefaultLook("knight"))
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := e.st.InsertEvents(ctx, []store.GameEvent{
		{At: past, Kind: "item_in", CharacterID: ana.ID, AccountID: acc, Ref: "ring_good", Amount: 1, Extra: map[string]any{"how": "loot"}},
		{At: past, Kind: "gold", CharacterID: ana.ID, AccountID: acc, Ref: "loot", Level: 1, Amount: 25},
	}); err != nil {
		t.Fatal(err)
	}
	// Existe 1 anel explicado pela telemetria... e mais 2 forjados no armazém.
	if _, err := db.Exec(ctx, `INSERT INTO character_items (character_id, place, slot, item_id, count) VALUES ($1, 0, 9, 'ring_good', 1)`, ana.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO account_items (account_id, slot, item_id, count) VALUES ($1, 0, 'ring_good', 2)`, acc); err != nil {
		t.Fatal(err)
	}
	type econ struct {
		Supply      store.GoldSupply
		Items       []store.ItemBalance
		Suspicious  int
		GoldDaily   []store.GoldNetDay
		TimingsMs   map[string]float64
		GeneratedAt time.Time
	}
	r := decodeJSON[econ](t, gm.req("GET", "/api/admin/economy?days=7", "", nil))
	if r.Suspicious != 1 || r.Items[0].Item != "ring_good" || r.Items[0].Surplus != 2 || r.Items[0].Storage != 2 {
		t.Fatalf("economy = %+v", r)
	}
	if len(r.GoldDaily) != 1 || r.GoldDaily[0].In != 25 || r.Supply.Total != game.StarterGold || len(r.TimingsMs) != 4 {
		t.Fatalf("gold = %+v %+v %v", r.GoldDaily, r.Supply, r.TimingsMs)
	}
	// Em cache: a segunda leitura é a mesma rodada, mesmo com o banco mudando.
	db.Exec(ctx, `DELETE FROM account_items`)
	if r2 := decodeJSON[econ](t, gm.req("GET", "/api/admin/economy?days=7", "", nil)); !r2.GeneratedAt.Equal(r.GeneratedAt) || r2.Suspicious != 1 {
		t.Fatalf("not cached: %v vs %v", r2.GeneratedAt, r.GeneratedAt)
	}
	// Outro período é outra chave.
	if r3 := decodeJSON[econ](t, gm.req("GET", "/api/admin/economy?days=30", "", nil)); r3.Suspicious != 0 {
		t.Fatalf("30 days = %+v", r3.Items[0])
	}

	ch := decodeJSON[struct {
		Character store.CharacterRef
		History   []store.CharacterDay
		Flows     []store.CharacterFlow
		Spikes    []store.Spike
	}](t, gm.req("GET", "/api/admin/economy/character?name=ana&days=7", "", nil))
	if ch.Character.ID != ana.ID || len(ch.History) != 1 || ch.History[0].GoldIn != 25 || ch.History[0].ItemsIn != 1 || len(ch.Flows) != 2 || ch.Spikes == nil {
		t.Fatalf("character = %+v", ch)
	}
	if code := gm.req("GET", "/api/admin/economy/character?name=ninguem", "", nil).StatusCode; code != http.StatusNotFound {
		t.Fatalf("unknown character = %d", code)
	}

	m := decodeJSON[struct {
		Days      int
		Gold      []store.GoldDayRow
		TimingsMs map[string]float64
	}](t, gm.req("GET", "/api/admin/metrics?days=1", "", nil))
	if m.Days != 1 || len(m.TimingsMs) != 8 {
		t.Fatalf("metrics = %+v", m)
	}

	e.api.audit(withAdmin(gm, e), "teste", "Ana", "x")
	gl := decodeJSON[struct {
		Entries  []store.GMLogRow
		Commands []store.FacetCount
	}](t, gm.req("GET", "/api/admin/economy/gm-log?command=portal:&target=ana", "", nil))
	if len(gl.Entries) != 1 || gl.Entries[0].Command != "portal:teste" || len(gl.Commands) != 1 {
		t.Fatalf("gm-log = %+v", gl)
	}
}

// withAdmin monta um pedido "de dentro do portal" para chamar o audit direto no teste.
func withAdmin(c *adminClient, e *adminEnv) *http.Request {
	id, _ := e.st.LoginIdentity(context.Background(), "dev", "mestre", "Mestre")
	r := httptest.NewRequest("POST", "/api/admin/x", nil)
	return r.WithContext(context.WithValue(r.Context(), adminCtxKey{}, &adminRequest{accountID: id}))
}

func TestTTLCacheSharesAndExpires(t *testing.T) {
	c := newTTLCache(time.Minute)
	now := time.Now()
	var mu sync.Mutex
	c.now = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	var calls atomic.Int32
	release := make(chan struct{})
	fn := func(context.Context) (any, error) {
		calls.Add(1)
		<-release
		return int(calls.Load()), nil
	}
	// Dez pedidos ao mesmo tempo: uma consulta só.
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if v, _, err := c.get(context.Background(), "k", fn); err != nil || v.(int) != 1 {
				t.Errorf("get = %v %v", v, err)
			}
		}()
	}
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", calls.Load())
	}
	// Quem desiste não estraga a consulta dos outros.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := c.get(ctx, "outra", func(context.Context) (any, error) { return 1, nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled = %v", err)
	}
	// Vence depois do TTL.
	mu.Lock()
	now = now.Add(61 * time.Second)
	mu.Unlock()
	if v, _, _ := c.get(context.Background(), "k", fn); v.(int) != 2 {
		t.Fatalf("after ttl = %v", v)
	}
	// Erro não fica guardado.
	boom := errors.New("boom")
	if _, _, err := c.get(context.Background(), "e", func(context.Context) (any, error) { return nil, boom }); err != boom {
		t.Fatalf("err = %v", err)
	}
	if v, _, err := c.get(context.Background(), "e", func(context.Context) (any, error) { return 3, nil }); err != nil || v.(int) != 3 {
		t.Fatalf("after error = %v %v", v, err)
	}
}

// Os dias que já acabaram vão ao banco uma vez (uma consulta por dia); hoje, toda vez.
func TestCachedDaysQueriesClosedDaysOnce(t *testing.T) {
	p := &panels{days: &dayCache{m: map[string]any{}}}
	today := store.DayStart(time.Now())
	if time.Since(today) < lateEvents {
		t.Skip("logo depois da meia-noite: ontem ainda conta como aberto")
	}
	from, to := today.AddDate(0, 0, -2), today.Add(24*time.Hour)
	var calls []string
	fetch := func(_ context.Context, a, b time.Time) ([]string, error) {
		calls = append(calls, a.Format("01-02")+"→"+b.Format("01-02"))
		return []string{a.Format("2006-01-02")}, nil
	}
	q := newQueryStats()
	rows, err := cachedDays(context.Background(), p, q, "x", from, to, fetch)
	if err != nil || len(rows) != 3 || len(calls) != 3 || q.count["x"] != 3 {
		t.Fatalf("first: rows=%v calls=%v", rows, calls)
	}
	calls = nil
	rows, _ = cachedDays(context.Background(), p, newQueryStats(), "x", from, to, fetch)
	if len(rows) != 3 || len(calls) != 1 || calls[0] != today.Format("01-02")+"→"+to.Format("01-02") {
		t.Fatalf("second: rows=%v calls=%v (only today should hit the database)", rows, calls)
	}
	// Outra consulta tem a sua própria chave.
	calls = nil
	cachedDays(context.Background(), p, newQueryStats(), "y", from, to, fetch)
	if len(calls) != 3 {
		t.Fatalf("other query reused the cache: %v", calls)
	}
}
