package api_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"erevia/server/internal/api"
	"erevia/server/internal/store/storetest"
)

type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func newEnv(t *testing.T, devLogin bool) (*httptest.Server, func() *client) {
	t.Helper()
	st := storetest.New(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	a := api.New(st, api.Config{PublicURL: "http://example.test", DevLogin: devLogin}, log)
	mux := http.NewServeMux()
	a.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, func() *client {
		jar, _ := cookiejar.New(nil)
		return &client{t: t, base: srv.URL, http: &http.Client{Jar: jar}}
	}
}

func (c *client) do(method, path, body string, out any) int {
	c.t.Helper()
	req, _ := http.NewRequest(method, c.base+path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	if out != nil {
		json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

func TestDevLoginDisabledByDefault(t *testing.T) {
	_, newClient := newEnv(t, false)
	c := newClient()
	if code := c.do("POST", "/api/auth/dev", `{"username":"ana"}`, nil); code != http.StatusNotFound {
		t.Fatalf("dev login without flag = %d, want 404", code)
	}
	var providers map[string]bool
	c.do("GET", "/api/auth/providers", "", &providers)
	if providers["dev"] || providers["google"] || providers["discord"] {
		t.Fatalf("providers = %v", providers)
	}
}

func TestLoginAndCharacters(t *testing.T) {
	_, newClient := newEnv(t, true)
	c := newClient()

	if code := c.do("GET", "/api/me", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("/me without login = %d", code)
	}
	if code := c.do("POST", "/api/auth/dev", `{"username":"Ana"}`, nil); code != http.StatusOK {
		t.Fatalf("dev login = %d", code)
	}
	var me struct{ DisplayName string }
	if code := c.do("GET", "/api/me", "", &me); code != http.StatusOK || me.DisplayName != "Ana" {
		t.Fatalf("/me = %d %+v", code, me)
	}

	var created struct {
		ID    int64
		Name  string
		Class string
	}
	if code := c.do("POST", "/api/characters", `{"name":"Aria","class":"mage"}`, &created); code != http.StatusCreated {
		t.Fatalf("create = %d", code)
	}
	if created.ID == 0 || created.Class != "mage" {
		t.Fatalf("created = %+v", created)
	}

	var errBody struct{ Error string }
	cases := []struct {
		body string
		code int
	}{
		{`{"name":"aria","class":"knight"}`, http.StatusConflict}, // nome repetido
		{`{"name":"x","class":"knight"}`, http.StatusBadRequest},  // nome curto
		{`{"name":"Valido","class":"dragao"}`, http.StatusBadRequest},
		{`{"name":"Valido","class":"knight","extra":1}`, http.StatusBadRequest},
		// Aparência fora das opções da classe.
		{`{"name":"Valido","class":"knight","look":{"weapon":"axe_common","offhand":"","emblem":"#d6b46e"}}`, http.StatusBadRequest},
		{`{"name":"Valido","class":"mage","look":{"weapon":"staff_common","offhand":"shield_badge","emblem":"#8c6fd6"}}`, http.StatusBadRequest},
		{`{"name":"Valido","class":"rogue","look":{"weapon":"daggers_common","offhand":"","emblem":"#123456"}}`, http.StatusBadRequest},
		{`{"name":"Valido","class":"rogue","look":{"weapon":"daggers_common","offhand":"","emblem":"#d04050","x":1}}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		if code := c.do("POST", "/api/characters", tc.body, &errBody); code != tc.code {
			t.Errorf("%s → %d, want %d (%s)", tc.body, code, tc.code, errBody.Error)
		}
	}

	var beto struct {
		Emblem string
		Look   struct{ Right, Left string }
	}
	if code := c.do("POST", "/api/characters", `{"name":"Beto","class":"rogue","look":{"weapon":"daggers_mixed_common","offhand":"","emblem":"#d04050"}}`, &beto); code != http.StatusCreated {
		t.Fatalf("create with look = %d", code)
	}
	if beto.Emblem != "#d04050" || beto.Look.Right != "dagger" || beto.Look.Left != "skeleton_blade" {
		t.Fatalf("created with look = %+v", beto)
	}
	c.do("POST", "/api/characters", `{"name":"Caio","class":"barbarian"}`, nil)
	if code := c.do("POST", "/api/characters", `{"name":"Dani","class":"knight"}`, nil); code != http.StatusConflict {
		t.Fatalf("4th character = %d, want 409", code)
	}

	var list struct {
		Characters []struct {
			Name   string
			Emblem string
			Look   struct{ Right, Left string }
		}
		Max int
	}
	c.do("GET", "/api/characters", "", &list)
	if len(list.Characters) != 3 || list.Max != 3 {
		t.Fatalf("list = %+v", list)
	}
	// A lista mostra o que cada um tem nas mãos e a cor (padrão da classe quando não escolhida).
	if a := list.Characters[0]; a.Name != "Aria" || a.Emblem != "#8c6fd6" || a.Look.Right != "staff" {
		t.Fatalf("list[0] = %+v", a)
	}
	if b := list.Characters[1]; b.Look.Left != "skeleton_blade" || b.Emblem != "#d04050" {
		t.Fatalf("list[1] = %+v", b)
	}

	// Outro jogador não vê os personagens da Ana.
	other := newClient()
	other.do("POST", "/api/auth/dev", `{"username":"Bob"}`, nil)
	other.do("GET", "/api/characters", "", &list)
	if len(list.Characters) != 0 {
		t.Fatalf("Bob sees %d characters", len(list.Characters))
	}

	// Logout invalida a sessão.
	if code := c.do("POST", "/api/auth/logout", `{}`, nil); code != http.StatusOK {
		t.Fatalf("logout = %d", code)
	}
	if code := c.do("GET", "/api/me", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("/me after logout = %d", code)
	}
}

func TestCreationOptions(t *testing.T) {
	_, newClient := newEnv(t, false)
	var opts struct {
		Classes map[string]struct {
			Weapons  []struct{ ID, Name, Model, OffModel string }
			Offhands []struct{ ID, Name, Model string }
			Emblem   string
		}
		Emblems []string
	}
	if code := newClient().do("GET", "/api/creation", "", &opts); code != http.StatusOK {
		t.Fatalf("creation = %d", code)
	}
	k := opts.Classes["knight"]
	if len(opts.Classes) != 4 || len(opts.Emblems) != 8 || k.Weapons[0].ID != "sword_common" || k.Weapons[0].Model != "sword_1handed" {
		t.Fatalf("options = %+v", opts)
	}
	if last := k.Offhands[len(k.Offhands)-1]; last.ID != "" || last.Name != "Nada" {
		t.Fatalf("knight offhands = %+v", k.Offhands)
	}
	if r := opts.Classes["rogue"].Weapons[0]; r.OffModel != "dagger" {
		t.Fatalf("rogue pair = %+v", r)
	}
}

func TestPostRequiresJSONAndSameOrigin(t *testing.T) {
	srv, newClient := newEnv(t, true)
	c := newClient()

	// Formulário (o que um site malicioso conseguiria enviar sem preflight) é recusado.
	req, _ := http.NewRequest("POST", srv.URL+"/api/auth/dev", strings.NewReader("username=ana"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, _ := c.http.Do(req)
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("form post = %d", resp.StatusCode)
	}

	req, _ = http.NewRequest("POST", srv.URL+"/api/auth/dev", strings.NewReader(`{"username":"ana"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://site-malicioso.example")
	resp, _ = c.http.Do(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin post = %d", resp.StatusCode)
	}
}

func TestOAuthLoginUnknownProvider(t *testing.T) {
	_, newClient := newEnv(t, false)
	c := newClient()
	c.http.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if code := c.do("GET", "/api/auth/google/login", "", nil); code != http.StatusNotFound {
		t.Fatalf("google login without config = %d", code)
	}
}

func TestOAuthLoginRedirectsWithPKCE(t *testing.T) {
	st := storetest.New(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	a := api.New(st, api.Config{
		PublicURL: "https://erevia.example",
		Google:    api.NewGoogle("client-id", "secret", "https://erevia.example"),
	}, log)
	mux := http.NewServeMux()
	a.Register(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/auth/google/login", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	for _, want := range []string{"accounts.google.com", "code_challenge=", "code_challenge_method=S256", "state=",
		"redirect_uri=https%3A%2F%2Ferevia.example%2Fapi%2Fauth%2Fgoogle%2Fcallback"} {
		if !strings.Contains(loc, want) {
			t.Errorf("redirect missing %q: %s", want, loc)
		}
	}
	cookie := rec.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("state cookie flags: %+v", cookie)
	}

	// Callback com state errado é recusado e não cria sessão.
	req := httptest.NewRequest("GET", "/api/auth/google/callback?state=errado&code=x", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "login_error=state_invalido") {
		t.Fatalf("bad state redirect = %s", loc)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == api.SessionCookie && c.Value != "" {
			t.Fatal("session must not be created on bad state")
		}
	}
}
