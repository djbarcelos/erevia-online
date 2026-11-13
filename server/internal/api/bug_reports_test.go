package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"erevia/server/internal/store/storetest"
	"erevia/server/internal/zone"
)

// fakeBugZone faz o papel da zona: o personagem 7 de qualquer conta está online.
type fakeBugZone struct {
	mu    sync.Mutex
	err   error
	calls int
}

func (f *fakeBugZone) PrepareBug(_ context.Context, accountID, characterID int64, text string) (zone.BugReport, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return zone.BugReport{}, f.err
	}
	if characterID != 7 {
		return zone.BugReport{}, zone.ErrBugNotOnline
	}
	if len([]rune(text)) < 8 {
		return zone.BugReport{}, zone.ErrBugTooShort
	}
	return zone.BugReport{AccountID: accountID, CharacterID: 7, Name: "Ana", Level: 4, X: 1.5, Z: -2, Version: "abc", UserAgent: "teste", Text: text}, nil
}

func (f *fakeBugZone) set(err error) {
	f.mu.Lock()
	f.err = err
	f.mu.Unlock()
}

type bugEnv struct {
	*adminEnv
	db   *pgx.Conn
	zone *fakeBugZone
}

func newBugEnv(t *testing.T) *bugEnv {
	t.Helper()
	st, url := storetest.NewWithURL(t)
	a := New(st, Config{PublicURL: "http://example.test", DevLogin: true}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	a.SetAdmin(AdminConfig{})
	fz := &fakeBugZone{}
	a.SetBugZone(fz)
	mux := http.NewServeMux()
	a.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	db, err := pgx.Connect(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(context.Background()) })
	return &bugEnv{adminEnv: &adminEnv{t: t, st: st, api: a, srv: srv}, db: db, zone: fz}
}

func (e *bugEnv) scalar(query string, args ...any) any {
	e.t.Helper()
	var v any
	if err := e.db.QueryRow(context.Background(), query, args...).Scan(&v); err != nil {
		e.t.Fatalf("%s: %v", query, err)
	}
	return v
}

// report manda um relato e devolve o status e o id.
func (c *adminClient) report(body string) (int, int64) {
	c.env.t.Helper()
	resp := c.req("POST", "/api/bug-reports", body, nil)
	var out struct{ ID int64 }
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out.ID
}

func (c *adminClient) shot(id int64, data []byte, contentType string) int {
	c.env.t.Helper()
	return c.req("POST", fmt.Sprintf("/api/bug-reports/%d/shot", id), string(data), map[string]string{"Content-Type": contentType}).StatusCode
}

func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 60}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const bugCtxJSON = `{"v":1,"fps":{"avg":58.5,"min":31,"samples":60},"ping":42,"region":"Cidade","errors":[{"kind":"client_invariant","message":"hud_hp"}],"messages":[{"t":-1.5,"type":"spawn","id":12}]}`

func TestBugReportWithContextAndShot(t *testing.T) {
	e := newBugEnv(t)
	anon := e.login("", false)
	if code, _ := anon.report(`{"text":"o golem atravessou a parede","characterId":7}`); code != http.StatusUnauthorized {
		t.Fatalf("anonymous = %d, want 401", code)
	}

	ana := e.login("Ana", false)
	code, id := ana.report(`{"text":"  o golem\u0007 atravessou a parede  ","characterId":7,"context":` + bugCtxJSON + `}`)
	if code != http.StatusCreated || id <= 0 {
		t.Fatalf("report = %d id %d, want 201", code, id)
	}
	if got := e.scalar(`SELECT text FROM bug_reports WHERE id = $1`, id); got != "o golem atravessou a parede" {
		t.Fatalf("text = %q", got)
	}
	if got := e.scalar(`SELECT context->>'region' FROM bug_reports WHERE id = $1`, id); got != "Cidade" {
		t.Fatalf("context region = %v", got)
	}
	if got := e.scalar(`SELECT (context->'fps'->>'min')::int FROM bug_reports WHERE id = $1`, id); got != int32(31) {
		t.Fatalf("context fps.min = %v", got)
	}
	if got := e.scalar(`SELECT character_name || ' ' || level || ' ' || version FROM bug_reports WHERE id = $1`, id); got != "Ana 4 abc" {
		t.Fatalf("stamped = %v", got)
	}

	// Print: JPEG de verdade, só uma vez e só do dono.
	img := testJPEG(t, 64, 36)
	if c := ana.shot(id, img, "image/jpeg"); c != http.StatusNoContent {
		t.Fatalf("shot = %d, want 204", c)
	}
	if c := ana.shot(id, img, "image/jpeg"); c != http.StatusConflict {
		t.Fatalf("second shot = %d, want 409", c)
	}
	if got := e.scalar(`SELECT width || 'x' || height || ' ' || length(jpeg) FROM bug_report_shots WHERE report_id = $1`, id); got != fmt.Sprintf("64x36 %d", len(img)) {
		t.Fatalf("shot row = %v", got)
	}

	// O portal lê o print; quem não é GM recebe o 404 de sempre.
	gm := e.login("Mestre", true)
	resp := gm.req("GET", fmt.Sprintf("/api/admin/bug-reports/%d/shot", id), "", nil)
	if body := readBody(resp); resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/jpeg" || body != string(img) {
		t.Fatalf("admin shot = %d %q (%d bytes)", resp.StatusCode, resp.Header.Get("Content-Type"), len(body))
	}
	if resp.Header.Get("Cache-Control") != "no-store" || resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("admin shot headers = %v", resp.Header)
	}
	if resp := ana.req("GET", fmt.Sprintf("/api/admin/bug-reports/%d/shot", id), "", nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("non-GM admin shot = %d, want 404", resp.StatusCode)
	}
	if resp := gm.req("GET", "/api/admin/bug-reports/999999/shot", "", nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing shot = %d, want 404", resp.StatusCode)
	}

	// Relato sem contexto (null) também vale.
	code, id2 := ana.report(`{"text":"a ponte some de noite","characterId":7,"context":null}`)
	if code != http.StatusCreated {
		t.Fatalf("no context = %d", code)
	}
	if got := e.scalar(`SELECT context IS NULL FROM bug_reports WHERE id = $1`, id2); got != true {
		t.Fatalf("context should be NULL")
	}
}

func TestBugShotOnlyOwnerValidJPEGAndInTime(t *testing.T) {
	e := newBugEnv(t)
	e.api.bug.shots = newLimiter(100, 100) // muitas tentativas da mesma conta neste teste
	ana := e.login("Ana", false)
	beto := e.login("Beto", false)
	_, id := ana.report(`{"text":"monstro andando morto","characterId":7}`)
	img := testJPEG(t, 32, 32)

	if c := beto.shot(id, img, "image/jpeg"); c != http.StatusNotFound {
		t.Fatalf("other account = %d, want 404", c)
	}
	if c := e.login("", false).shot(id, img, "image/jpeg"); c != http.StatusUnauthorized {
		t.Fatalf("anonymous = %d, want 401", c)
	}
	if c := ana.shot(id, img, "text/plain"); c != http.StatusUnsupportedMediaType {
		t.Fatalf("text/plain = %d, want 415", c)
	}
	if c := ana.shot(id, []byte("\xff\xd8\xffnão é jpeg"), "image/jpeg"); c != http.StatusBadRequest {
		t.Fatalf("fake jpeg = %d, want 400", c)
	}
	if c := ana.shot(id, testJPEG(t, 1300, 10), "image/jpeg"); c != http.StatusBadRequest {
		t.Fatalf("too wide = %d, want 400", c)
	}
	if c := ana.shot(id, bytes.Repeat([]byte{0xFF}, BugShotMaxBytes+1), "image/jpeg"); c != http.StatusRequestEntityTooLarge {
		t.Fatalf("too big = %d, want 413", c)
	}
	if c := ana.shot(999999, img, "image/jpeg"); c != http.StatusNotFound {
		t.Fatalf("missing report = %d, want 404", c)
	}
	// Outra origem: recusado.
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/bug-reports/%d/shot", e.srv.URL, id), bytes.NewReader(img))
	req.Header.Set("Content-Type", "image/jpeg")
	req.Header.Set("Origin", "https://site-malicioso.example")
	resp, err := ana.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin = %d, want 403", resp.StatusCode)
	}

	// Passou do prazo: o mesmo 404.
	_, old := ana.report(`{"text":"relato antigo demais","characterId":7}`)
	if _, err := e.db.Exec(context.Background(), `UPDATE bug_reports SET at = now() - interval '11 minutes' WHERE id = $1`, old); err != nil {
		t.Fatal(err)
	}
	if c := ana.shot(old, img, "image/jpeg"); c != http.StatusNotFound {
		t.Fatalf("expired = %d, want 404", c)
	}
	if got := e.scalar(`SELECT count(*) FROM bug_report_shots`); got != int64(0) {
		t.Fatalf("shots = %v, want 0", got)
	}
}

func TestBugReportValidationZoneAndLimits(t *testing.T) {
	e := newBugEnv(t)
	ana := e.login("Ana", false)
	e.api.bug.create = newLimiter(100, 100) // o limite por conta fica para o fim
	cases := []struct {
		name string
		body string
		want int
	}{
		{"contexto não-objeto", `{"text":"texto comprido o bastante","characterId":7,"context":[1,2]}`, http.StatusBadRequest},
		{"contexto grande", `{"text":"texto comprido o bastante","characterId":7,"context":{"a":"` + strings.Repeat("x", BugContextMaxBytes) + `"}}`, http.StatusBadRequest},
		{"corpo grande", `{"text":"` + strings.Repeat("x", bugBodyMax) + `"}`, http.StatusRequestEntityTooLarge},
		{"json quebrado", `{"text":`, http.StatusBadRequest},
	}
	for _, c := range cases {
		if code, _ := ana.report(c.body); code != c.want {
			t.Fatalf("%s = %d, want %d", c.name, code, c.want)
		}
	}
	if e.zone.calls != 0 {
		t.Fatalf("invalid bodies reached the zone (%d)", e.zone.calls)
	}
	// Sem JSON: o mesmo checkPost dos outros pedidos.
	if resp := ana.req("POST", "/api/bug-reports", `{"text":"texto comprido o bastante"}`, map[string]string{"Content-Type": "text/plain"}); resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("text/plain = %d, want 415", resp.StatusCode)
	}

	// O que a zona responde vira o status certo.
	if code, _ := ana.report(`{"text":"texto comprido o bastante","characterId":8}`); code != http.StatusConflict {
		t.Fatalf("not online = %d, want 409", code)
	}
	if code, _ := ana.report(`{"text":"curto","characterId":7}`); code != http.StatusBadRequest {
		t.Fatalf("too short = %d, want 400", code)
	}
	e.zone.set(&zone.BugCooldownError{Seconds: 12})
	resp := ana.req("POST", "/api/bug-reports", `{"text":"texto comprido o bastante","characterId":7}`, nil)
	if body := readBody(resp); resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get("Retry-After") != "12" || !strings.Contains(body, "Espere 12 s") {
		t.Fatalf("cooldown = %d %q %s", resp.StatusCode, resp.Header.Get("Retry-After"), body)
	}
	e.zone.set(zone.ErrZoneBusy)
	if code, _ := ana.report(`{"text":"texto comprido o bastante","characterId":7}`); code != http.StatusServiceUnavailable {
		t.Fatalf("zone busy = %d, want 503", code)
	}
	e.zone.set(nil)

	// Limite por conta: 3 de uma vez.
	e.api.bug.create = newLimiter(bugPerSec, bugBurst)
	for i := range bugBurst {
		if code, _ := ana.report(`{"text":"texto comprido o bastante","characterId":7}`); code != http.StatusCreated {
			t.Fatalf("report %d = %d", i, code)
		}
	}
	if code, _ := ana.report(`{"text":"texto comprido o bastante","characterId":7}`); code != http.StatusTooManyRequests {
		t.Fatalf("over the limit = %d, want 429", code)
	}
	// Outra conta tem o próprio balde.
	if code, _ := e.login("Beto", false).report(`{"text":"texto comprido o bastante","characterId":7}`); code != http.StatusCreated {
		t.Fatalf("other account = %d", code)
	}

	// Sem zona ligada: 503 (o cliente cai no /bug do chat).
	e.api.SetBugZone(nil)
	if code, _ := e.login("Caio", false).report(`{"text":"texto comprido o bastante","characterId":7}`); code != http.StatusServiceUnavailable {
		t.Fatalf("no zone = %d, want 503", code)
	}
}

func TestBugReportGoneWithAccount(t *testing.T) {
	e := newBugEnv(t)
	ana := e.login("Ana", false)
	_, id := ana.report(`{"text":"o golem atravessou a parede","characterId":7,"context":` + bugCtxJSON + `}`)
	if c := ana.shot(id, testJPEG(t, 16, 16), "image/jpeg"); c != http.StatusNoContent {
		t.Fatalf("shot = %d", c)
	}
	if resp := ana.req("DELETE", "/api/account", `{"confirm":"APAGAR"}`, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("delete account = %d", resp.StatusCode)
	}
	if got := e.scalar(`SELECT count(*) FROM bug_reports`); got != int64(0) {
		t.Fatalf("bug_reports after delete = %v", got)
	}
	if got := e.scalar(`SELECT count(*) FROM bug_report_shots`); got != int64(0) {
		t.Fatalf("bug_report_shots after delete = %v", got)
	}
}
