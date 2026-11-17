package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
	"erevia/server/internal/zone"
)

type fakeZone struct {
	online int
	err    error
}

func (f fakeZone) Status(context.Context) (zone.AdminStatus, error) {
	return zone.AdminStatus{Online: f.online}, f.err
}

type adminEnv struct {
	t   *testing.T
	st  *store.Store
	api *API
	srv *httptest.Server
}

func newAdminEnv(t *testing.T, cfg AdminConfig) *adminEnv {
	t.Helper()
	st := storetest.New(t)
	a := New(st, Config{PublicURL: "http://example.test", DevLogin: true}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	a.SetAdmin(cfg)
	mux := http.NewServeMux()
	a.Register(mux)
	// Rota de escrita só do teste: mostra o caminho de uma ação de verdade (audit + resposta).
	a.admin.mux.HandleFunc("POST /api/admin/test/echo", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Target string }
		if !readJSON(w, r, &body) {
			return
		}
		if err := a.audit(r, "test.echo", body.Target, "detalhe"); err != nil {
			a.internalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	// Rota de escrita que "esquece" o audit: a rede de segurança do AdminOnly grava mesmo assim.
	a.admin.mux.HandleFunc("POST /api/admin/test/forgot", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &adminEnv{t: t, st: st, api: a, srv: srv}
}

type adminClient struct {
	env  *adminEnv
	http *http.Client
}

// login entra pelo login de dev; gm marca a conta como GM no banco.
func (e *adminEnv) login(user string, gm bool) *adminClient {
	e.t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &adminClient{env: e, http: &http.Client{Jar: jar}}
	if user == "" {
		return c
	}
	if resp := c.req("POST", "/api/auth/dev", `{"username":"`+user+`"}`, nil); resp.StatusCode != http.StatusOK {
		e.t.Fatalf("dev login %s = %d", user, resp.StatusCode)
	}
	if gm {
		id, err := e.st.LoginIdentity(context.Background(), "dev", strings.ToLower(user), user)
		if err != nil {
			e.t.Fatal(err)
		}
		if err := e.st.SetGM(context.Background(), id, true); err != nil {
			e.t.Fatal(err)
		}
	}
	return c
}

func (c *adminClient) req(method, path, body string, hdr map[string]string) *http.Response {
	c.env.t.Helper()
	req, _ := http.NewRequest(method, c.env.srv.URL+path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.env.t.Fatalf("%s %s: %v", method, path, err)
	}
	c.env.t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func readBody(resp *http.Response) string {
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

var adminHdr = map[string]string{AdminHeader: "1"}

func TestAdminHiddenFromNonGM(t *testing.T) {
	e := newAdminEnv(t, AdminConfig{})
	anon := e.login("", false)
	player := e.login("Jogador", false)

	// O 404 de um caminho qualquer que não existe.
	want := readBody(anon.req("GET", "/api/nada-aqui", "", nil))

	for _, c := range []*adminClient{anon, player} {
		for _, tc := range []struct{ method, path, body string }{
			{"GET", "/api/admin/me", ""},
			{"GET", "/api/admin/overview", ""},
			{"GET", "/api/admin/gm-log", ""},
			{"GET", "/api/admin/progression", ""},
			{"GET", "/api/admin/nao-existe", ""},
			{"POST", "/api/admin/overview", `{}`}, // método errado: 404, não 405
			{"POST", "/api/admin/test/echo", `{"target":"x"}`},
			{"DELETE", "/api/admin/test/echo", `{}`},
		} {
			resp := c.req(tc.method, tc.path, tc.body, adminHdr)
			body := readBody(resp)
			if resp.StatusCode != http.StatusNotFound || body != want {
				t.Errorf("%s %s = %d %q, want 404 %q", tc.method, tc.path, resp.StatusCode, body, want)
			}
			if resp.Header.Get("X-Frame-Options") != "" || resp.Header.Get("Content-Security-Policy") != "" {
				t.Errorf("%s %s: 404 has admin headers (reveals the portal)", tc.method, tc.path)
			}
		}
	}

	// Banido perde o portal mesmo sendo GM (a sessão nem vale mais).
	gm := e.login("Banido", true)
	if code := gm.req("GET", "/api/admin/me", "", nil).StatusCode; code != http.StatusOK {
		t.Fatalf("gm before ban = %d", code)
	}
	id, _ := e.st.LoginIdentity(context.Background(), "dev", "banido", "Banido")
	if _, err := e.st.CreateCharacter(context.Background(), id, "Banidao", "knight", game.DefaultLook("knight")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.st.SetBanned(context.Background(), "Banidao", true, "teste"); err != nil {
		t.Fatal(err)
	}
	if code := gm.req("GET", "/api/admin/me", "", nil).StatusCode; code != http.StatusNotFound {
		t.Fatalf("banned gm = %d, want 404", code)
	}

	// Tirar o GM vale na próxima requisição.
	gm2 := e.login("Antigo", true)
	id2, _ := e.st.LoginIdentity(context.Background(), "dev", "antigo", "Antigo")
	e.st.SetGM(context.Background(), id2, false)
	if code := gm2.req("GET", "/api/admin/me", "", nil).StatusCode; code != http.StatusNotFound {
		t.Fatalf("former gm = %d, want 404", code)
	}
}

func TestAdminOverviewForGM(t *testing.T) {
	started := time.Now().Add(-90 * time.Minute)
	e := newAdminEnv(t, AdminConfig{Version: "abc123", StartedAt: started, Zone: fakeZone{online: 7}})
	gm := e.login("Mestre", true)
	player := e.login("Jogador", false)
	player.req("POST", "/api/characters", `{"name":"Heroi","class":"mage"}`, nil)

	resp := gm.req("GET", "/api/admin/overview", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("overview = %d", resp.StatusCode)
	}
	for k, v := range map[string]string{
		"Cache-Control": "no-store", "X-Frame-Options": "DENY", "X-Content-Type-Options": "nosniff",
	} {
		if got := resp.Header.Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP = %q", csp)
	}
	var ov struct {
		Online    *int
		Version   string
		UptimeSec int64
		Summary   store.AdminSummary
	}
	json.NewDecoder(resp.Body).Decode(&ov)
	if ov.Online == nil || *ov.Online != 7 || ov.Version != "abc123" || ov.UptimeSec < 5390 || ov.UptimeSec > 5500 {
		t.Fatalf("overview = %+v", ov)
	}
	if s := ov.Summary; s.Accounts != 2 || s.Accounts24h != 2 || s.Characters != 1 || s.Characters24h != 1 || s.Bugs24h != 0 {
		t.Fatalf("summary = %+v", s)
	}

	var me struct {
		AccountID   int64
		DisplayName string
	}
	json.NewDecoder(gm.req("GET", "/api/admin/me", "", nil).Body).Decode(&me)
	if me.DisplayName != "Mestre" || me.AccountID == 0 {
		t.Fatalf("me = %+v", me)
	}
}

// A seção Progressão traz as cinco views da 00018, com as colunas na ordem do SQL.
func TestAdminProgressionForGM(t *testing.T) {
	e := newAdminEnv(t, AdminConfig{})
	gm := e.login("Mestre", true)
	player := e.login("Jogador", false)
	player.req("POST", "/api/characters", `{"name":"Heroi","class":"mage"}`, nil)

	resp := gm.req("GET", "/api/admin/progression", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("progression = %d %s", resp.StatusCode, readBody(resp))
	}
	var out struct{ Tables []store.ProgressionTable }
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	ids := map[string]store.ProgressionTable{}
	for _, tb := range out.Tables {
		ids[tb.ID] = tb
	}
	for _, id := range []string{"v_level_playtime", "v_xp_sources", "v_level_kills_deaths", "v_gold_per_hour", "v_level_reached"} {
		if len(ids[id].Columns) == 0 {
			t.Errorf("%s ausente ou sem colunas: %+v", id, ids[id])
		}
	}
	// O funil vem de characters: o personagem criado chegou ao nível 1.
	funnel := ids["v_level_reached"]
	if len(funnel.Rows) != 30 || funnel.Columns[0] != "nivel" || funnel.Columns[1] != "chegaram" || funnel.Rows[0][1] != float64(1) {
		t.Fatalf("funil = %v %v", funnel.Columns, funnel.Rows[:min(2, len(funnel.Rows))])
	}
}

func TestAdminOverviewZoneDown(t *testing.T) {
	e := newAdminEnv(t, AdminConfig{Zone: fakeZone{err: zone.ErrZoneBusy}})
	gm := e.login("Mestre", true)
	var ov struct{ Online *int }
	resp := gm.req("GET", "/api/admin/overview", "", nil)
	json.NewDecoder(resp.Body).Decode(&ov)
	if resp.StatusCode != http.StatusOK || ov.Online != nil {
		t.Fatalf("zone down: %d online=%v (want 200 and null)", resp.StatusCode, ov.Online)
	}
}

func TestAdminWritesNeedHeaderAndAreAudited(t *testing.T) {
	e := newAdminEnv(t, AdminConfig{})
	gm := e.login("Mestre", true)

	// Sem o cabeçalho, com outro valor, como formulário ou de outra origem: recusado e nada no gm_log.
	if code := gm.req("POST", "/api/admin/test/echo", `{"target":"Alvo"}`, nil).StatusCode; code != http.StatusForbidden {
		t.Fatalf("without header = %d, want 403", code)
	}
	if code := gm.req("POST", "/api/admin/test/echo", `{"target":"Alvo"}`, map[string]string{AdminHeader: "0"}).StatusCode; code != http.StatusForbidden {
		t.Fatalf("wrong header = %d, want 403", code)
	}
	resp := gm.req("POST", "/api/admin/test/echo", "", map[string]string{AdminHeader: "1", "Content-Type": "application/x-www-form-urlencoded"})
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("form = %d, want 415", resp.StatusCode)
	}
	resp = gm.req("POST", "/api/admin/test/echo", `{"target":"Alvo"}`, map[string]string{AdminHeader: "1", "Origin": "https://site-malicioso.example"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin = %d, want 403", resp.StatusCode)
	}
	if log, _ := e.st.RecentGMLog(context.Background(), 10); len(log) != 0 {
		t.Fatalf("rejected writes reached gm_log: %+v", log)
	}

	// Leitura não precisa do cabeçalho.
	if code := gm.req("GET", "/api/admin/me", "", nil).StatusCode; code != http.StatusOK {
		t.Fatalf("GET without header = %d", code)
	}

	// Com o cabeçalho: passa e fica no gm_log com quem, o quê e em quem.
	if code := gm.req("POST", "/api/admin/test/echo", `{"target":"Alvo"}`, adminHdr).StatusCode; code != http.StatusOK {
		t.Fatalf("with header = %d", code)
	}
	// Rota que esqueceu o audit: a rede de segurança registra.
	if code := gm.req("POST", "/api/admin/test/forgot", `{}`, adminHdr).StatusCode; code != http.StatusOK {
		t.Fatalf("forgot = %d", code)
	}
	log, err := e.st.RecentGMLog(context.Background(), 10)
	if err != nil || len(log) != 2 {
		t.Fatalf("gm_log = %+v, %v", log, err)
	}
	if g := log[1]; g.GMName != "Mestre" || g.GMAccountID == 0 || g.Command != "portal:test.echo" || g.Target != "Alvo" || g.Detail != "detalhe" {
		t.Fatalf("audited = %+v", g)
	}
	if g := log[0]; g.Command != "portal:unaudited POST /api/admin/test/forgot" {
		t.Fatalf("safety net = %+v", g)
	}

	// A lista do portal mostra as duas.
	var out struct{ Entries []store.GMLogRow }
	json.NewDecoder(gm.req("GET", "/api/admin/gm-log", "", nil).Body).Decode(&out)
	if len(out.Entries) != 2 || out.Entries[1].Target != "Alvo" {
		t.Fatalf("gm-log = %+v", out)
	}
}

func TestAdminRateLimitPerAccount(t *testing.T) {
	e := newAdminEnv(t, AdminConfig{ReadBurst: 3, ReadPerSec: 0.01, WriteBurst: 1, WritePerSec: 0.01})
	a, b := e.login("Mestre", true), e.login("Outro", true)
	for i := 0; i < 3; i++ {
		if code := a.req("GET", "/api/admin/me", "", nil).StatusCode; code != http.StatusOK {
			t.Fatalf("read %d = %d", i, code)
		}
	}
	resp := a.req("GET", "/api/admin/me", "", nil)
	if resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get("Retry-After") == "" {
		t.Fatalf("4th read = %d (Retry-After %q), want 429", resp.StatusCode, resp.Header.Get("Retry-After"))
	}
	// Cada conta tem o seu balde; escrita tem outro.
	if code := b.req("GET", "/api/admin/me", "", nil).StatusCode; code != http.StatusOK {
		t.Fatalf("other gm = %d", code)
	}
	if code := a.req("POST", "/api/admin/test/echo", `{"target":"x"}`, adminHdr).StatusCode; code != http.StatusOK {
		t.Fatalf("first write = %d", code)
	}
	if code := a.req("POST", "/api/admin/test/echo", `{"target":"x"}`, adminHdr).StatusCode; code != http.StatusTooManyRequests {
		t.Fatalf("second write = %d, want 429", code)
	}
	// Quem não é GM continua vendo 404 (o limite só existe depois da checagem).
	if code := e.login("", false).req("GET", "/api/admin/me", "", nil).StatusCode; code != http.StatusNotFound {
		t.Fatalf("anon = %d", code)
	}
}

func TestLimiterRefills(t *testing.T) {
	now := time.Unix(1000, 0)
	l := newLimiter(1, 2)
	l.now = func() time.Time { return now }
	for i := 0; i < 2; i++ {
		if ok, _ := l.allow(1); !ok {
			t.Fatalf("burst %d refused", i)
		}
	}
	ok, wait := l.allow(1)
	if ok || wait <= 0 || wait > time.Second {
		t.Fatalf("empty bucket: ok=%v wait=%v", ok, wait)
	}
	now = now.Add(1500 * time.Millisecond)
	if ok, _ := l.allow(1); !ok {
		t.Fatal("did not refill")
	}
}
