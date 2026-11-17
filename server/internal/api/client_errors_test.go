package api_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"erevia/server/internal/api"
	"erevia/server/internal/store/storetest"
)

// errEnv: servidor da API e uma conexão direta ao schema do teste.
func errEnv(t *testing.T) (*httptest.Server, *pgx.Conn, func() *client) {
	t.Helper()
	st, url := storetest.NewWithURL(t)
	a := api.New(st, api.Config{PublicURL: "http://example.test", DevLogin: true}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	mux := http.NewServeMux()
	a.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	db, err := pgx.Connect(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(context.Background()) })
	return srv, db, func() *client {
		jar, _ := cookiejar.New(nil)
		return &client{t: t, base: srv.URL, http: &http.Client{Jar: jar}}
	}
}

// postErrors manda um lote como o navegador (fetch: JSON; sendBeacon: text/plain) a partir
// do IP "ip" (o X-Forwarded-For, como atrás do proxy da Railway).
func (c *client) postErrors(body, contentType, ip string) int {
	c.t.Helper()
	req, _ := http.NewRequest("POST", c.base+"/api/client-errors", strings.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if ip != "" {
		req.Header.Set("X-Forwarded-For", "1.2.3.4, "+ip) // só o último vale
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("post errors: %v", err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func scalar[T any](t *testing.T, db *pgx.Conn, query string, args ...any) T {
	t.Helper()
	var v T
	if err := db.QueryRow(context.Background(), query, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return v
}

func TestClientErrorsAnonymousAndUpsert(t *testing.T) {
	_, db, newClient := errEnv(t)
	anon := newClient()
	batch := `{"version":"abc123","errors":[{"kind":"error","message":"Error: boom","stack":"Error: boom\n    at f (http://x/assets/index-1.js:1:2)","count":3,
		"ctx":{"characterId":1,"region":"Cidade","x":1.5,"z":-2,"fps":60,"ping":35},"extra":{"a":1},"futuro":true}],"outroCampo":1}`
	if code := anon.postErrors(batch, "application/json", "10.0.0.1"); code != http.StatusNoContent {
		t.Fatalf("anonymous = %d, want 204", code)
	}
	// sendBeacon manda text/plain: também vale, e soma na mesma linha.
	if code := anon.postErrors(strings.Replace(batch, `"count":3`, `"count":4`, 1), "text/plain;charset=UTF-8", "10.0.0.1"); code != http.StatusNoContent {
		t.Fatalf("beacon = %d, want 204", code)
	}
	if n := scalar[int64](t, db, `SELECT count(*) FROM client_errors`); n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
	if n := scalar[int64](t, db, `SELECT count FROM client_errors WHERE version = 'abc123'`); n != 7 {
		t.Fatalf("count = %d, want 7", n)
	}
	if acc := scalar[*int64](t, db, `SELECT account_id FROM client_errors`); acc != nil {
		t.Fatalf("anonymous account = %d", *acc)
	}
	if ua := scalar[string](t, db, `SELECT user_agent FROM client_errors`); !strings.HasPrefix(ua, "Go-http-client") {
		t.Fatalf("user agent = %q", ua)
	}

	// Com sessão: conta e personagem (o dela) entram.
	c := newClient()
	c.do("POST", "/api/auth/dev", `{"username":"Ana"}`, nil)
	var created struct{ ID int64 }
	c.do("POST", "/api/characters", `{"name":"Aria","class":"mage"}`, &created)
	body := fmt.Sprintf(`{"version":"abc123","errors":[{"kind":"webgl","message":"contexto perdido","count":1,"ctx":{"characterId":%d,"region":"Arredores"}}]}`, created.ID)
	if code := c.postErrors(body, "application/json", "10.0.0.2"); code != http.StatusNoContent {
		t.Fatalf("with session = %d", code)
	}
	if n := scalar[int64](t, db, `SELECT count(*) FROM client_errors WHERE kind = 'webgl' AND account_id IS NOT NULL AND character_id = $1 AND region = 'Arredores'`, created.ID); n != 1 {
		t.Fatalf("webgl row with account = %d", n)
	}

	// Formulário (sem JSON) e outra origem: recusados como nos outros pedidos de escrita.
	if code := anon.postErrors(batch, "application/x-www-form-urlencoded", "10.0.0.3"); code != http.StatusUnsupportedMediaType {
		t.Fatalf("form = %d, want 415", code)
	}
	req, _ := http.NewRequest("POST", anon.base+"/api/client-errors", strings.NewReader(batch))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Origin", "https://site-malicioso.example")
	if resp, _ := anon.http.Do(req); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin = %d, want 403", resp.StatusCode)
	}
	if code := anon.postErrors(`{"errors":[`, "application/json", "10.0.0.3"); code != http.StatusBadRequest {
		t.Fatalf("bad json = %d, want 400", code)
	}
}

func TestClientErrorsLimits(t *testing.T) {
	_, db, newClient := errEnv(t)
	c := newClient()

	// Corpo acima de 128 KB: 413, e nada gravado.
	huge := `{"errors":[{"message":"x","stack":"` + strings.Repeat("a", api.ClientErrorsMaxBody) + `"}]}`
	if code := c.postErrors(huge, "application/json", "10.1.0.1"); code != http.StatusRequestEntityTooLarge {
		t.Fatalf("huge = %d, want 413", code)
	}
	if n := scalar[int64](t, db, `SELECT count(*) FROM client_errors`); n != 0 {
		t.Fatalf("rows after 413 = %d", n)
	}

	// Mensagem e stack longas são cortadas; mais de 20 erros no lote, só os 20 primeiros.
	var list []string
	for i := range 25 {
		long := 1
		if i < 3 { // três cheios (o corpo inteiro tem de caber nos 128 KB)
			long = 600
		}
		list = append(list, fmt.Sprintf(`{"message":"erro %d %s","stack":"%s"}`, i, strings.Repeat("é", long), strings.Repeat("b", 16*long)))
	}
	if code := c.postErrors(`{"version":"v","errors":[`+strings.Join(list, ",")+`]}`, "application/json", "10.1.0.1"); code != http.StatusNoContent {
		t.Fatalf("long batch = %d", code)
	}
	if n := scalar[int64](t, db, `SELECT count(*) FROM client_errors`); n != api.ClientErrorsMaxBatch {
		t.Fatalf("rows = %d, want %d", n, api.ClientErrorsMaxBatch)
	}
	if n := scalar[int64](t, db, `SELECT max(char_length(message)) FROM client_errors`); n != api.ClientErrorMaxMessage {
		t.Fatalf("message length = %d, want %d", n, api.ClientErrorMaxMessage)
	}
	if n := scalar[int64](t, db, `SELECT max(octet_length(stack)) FROM client_errors`); n != api.ClientErrorMaxStack {
		t.Fatalf("stack length = %d, want %d", n, api.ClientErrorMaxStack)
	}
	// Sem mensagem não grava; byte nulo e extra inválido não derrubam o lote.
	if code := c.postErrors(`{"errors":[{"message":""},{"message":"nulo\u0000aqui","kind":"x","extra":{"z":"\u0000"}},{"message":"lista","extra":[1]}]}`, "application/json", "10.1.0.1"); code != http.StatusNoContent {
		t.Fatalf("odd batch = %d", code)
	}
	if n := scalar[int64](t, db, `SELECT count(*) FROM client_errors WHERE message IN ('nuloaqui', 'lista') AND extra IS NULL`); n != 2 {
		t.Fatalf("odd rows = %d, want 2", n)
	}
}

func TestClientErrorsRateLimit(t *testing.T) {
	_, _, newClient := errEnv(t)
	body := `{"errors":[{"message":"spam"}]}`

	// Por IP: 30 por minuto, sem sessão.
	anon := newClient()
	for i := range api.ClientErrorsPerMinute {
		if code := anon.postErrors(body, "application/json", "10.2.0.1"); code != http.StatusNoContent {
			t.Fatalf("request %d = %d", i+1, code)
		}
	}
	if code := anon.postErrors(body, "application/json", "10.2.0.1"); code != http.StatusTooManyRequests {
		t.Fatalf("over IP limit = %d, want 429", code)
	}
	if code := anon.postErrors(body, "application/json", "10.2.0.2"); code != http.StatusNoContent {
		t.Fatalf("other IP = %d, want 204", code)
	}

	// Por conta: trocar de IP não adianta.
	c := newClient()
	c.do("POST", "/api/auth/dev", `{"username":"Ana"}`, nil)
	for i := range api.ClientErrorsPerMinute {
		if code := c.postErrors(body, "application/json", fmt.Sprintf("10.3.0.%d", i)); code != http.StatusNoContent {
			t.Fatalf("account request %d = %d", i+1, code)
		}
	}
	if code := c.postErrors(body, "application/json", "10.3.1.1"); code != http.StatusTooManyRequests {
		t.Fatalf("over account limit = %d, want 429", code)
	}
}
