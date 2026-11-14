package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

// ---------- Webhook falso e relógio ----------

// hook é um webhook do Discord falso: grava cada mensagem e responde o que o teste mandar.
type hook struct {
	srv *httptest.Server
	mu  sync.Mutex
	got []message
	// replies: respostas programadas, em ordem (depois delas, 204).
	replies []reply
	calls   int
}

type reply struct {
	status int
	body   string
	header map[string]string
}

func newHook(t *testing.T) *hook {
	h := &hook{}
	h.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.calls++
		if r.Method != http.MethodPost || r.URL.Path != "/api/webhooks/123/token-secreto" || r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if len(h.replies) > 0 {
			rp := h.replies[0]
			h.replies = h.replies[1:]
			for k, v := range rp.header {
				w.Header().Set(k, v)
			}
			w.WriteHeader(rp.status)
			io.WriteString(w, rp.body)
			return
		}
		var m message
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.got = append(h.got, m)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(h.srv.Close)
	return h
}

func (h *hook) url() string { return h.srv.URL + "/api/webhooks/123/token-secreto" }

func (h *hook) program(r ...reply) {
	h.mu.Lock()
	h.replies = append(h.replies, r...)
	h.mu.Unlock()
}

// take devolve e limpa as mensagens recebidas.
func (h *hook) take() []message {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := h.got
	h.got = nil
	return out
}

// embeds: todos os embeds recebidos (e limpa).
func (h *hook) embeds() []Embed {
	var out []Embed
	for _, m := range h.take() {
		out = append(out, m.Embeds...)
	}
	return out
}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// env é um teste com banco, webhook falso e relógio.
type env struct {
	t   *testing.T
	ctx context.Context
	st  *store.Store
	h   *hook
	clk *fakeClock
	log *bytes.Buffer
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return &env{t: t, ctx: ctx, st: storetest.New(t), h: newHook(t), clk: &fakeClock{t: time.Now()}, log: &bytes.Buffer{}}
}

// worker cria um worker (um "processo"); o primeiro Check prepara o banco (bootstrap).
func (e *env) worker(mut ...func(*Config)) *Worker {
	cfg := Config{
		WebhookURL: e.h.url(), PublicURL: "https://erevia.test/", Version: "abc1234def",
		SpikeN: 20, BootDelay: 24 * time.Hour, Now: e.clk.Now,
	}
	for _, m := range mut {
		m(&cfg)
	}
	w := New(cfg, e.st, nil, slog.New(slog.NewTextHandler(e.log, nil)))
	if w.out != nil {
		w.out.sleep = func(ctx context.Context, d time.Duration) error { return ctx.Err() }
	}
	return w
}

func (e *env) serverError(ref, panicText string, amount int64, at time.Time) {
	e.t.Helper()
	err := e.st.InsertEvents(e.ctx, []store.GameEvent{{
		At: at, Kind: "server_error", Ref: ref, Amount: amount, X: 10, Z: 20,
		Extra: map[string]any{
			"where": "msg", "player": 7, "msg": "UseSkill", "panic": panicText,
			"frames": []string{"zone.(*Zone).useSkill skills.go", "zone.(*Zone).handle zone.go"},
			"version": "abc1234def", "total": amount,
		},
	}})
	if err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) event(ev store.GameEvent) {
	e.t.Helper()
	if err := e.st.InsertEvents(e.ctx, []store.GameEvent{ev}); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) clientError(message, version string, count int64) string {
	e.t.Helper()
	stack := "at boom (https://erevia.test/assets/index.js:1:200)\nat tick (https://erevia.test/assets/index.js:1:900)"
	hash := store.ClientErrorHash(message, stack)
	x, z := float32(5), float32(-3)
	if err := e.st.RecordClientErrors(e.ctx, []store.ClientError{{
		Hash: hash, Version: version, Kind: "error", Message: message, Stack: stack, Count: count,
		UserAgent: "Mozilla/5.0 (Teste)", Region: "Cidade", X: &x, Z: &z,
	}}); err != nil {
		e.t.Fatal(err)
	}
	return hash
}

func (e *env) bug(acc int64, text string) {
	e.t.Helper()
	if err := e.st.SaveBugReport(e.ctx, store.BugReport{
		AccountID: acc, CharacterName: "Aldric", Level: 12, X: 55, Z: 10,
		Version: "abc1234def", UserAgent: "Mozilla/5.0 (Teste)", Text: text,
	}); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) account(name string) int64 {
	e.t.Helper()
	id, err := e.st.LoginIdentity(e.ctx, "dev", name, name)
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

func titles(list []Embed) []string {
	out := make([]string, len(list))
	for i, e := range list {
		out[i] = e.Title
	}
	return out
}

func field(e Embed, name string) string {
	for _, f := range e.Fields {
		if f.Name == name {
			return f.Value
		}
	}
	return ""
}

func one(t *testing.T, list []Embed, title string) Embed {
	t.Helper()
	if len(list) != 1 || !strings.HasPrefix(list[0].Title, title) {
		t.Fatalf("esperava um aviso %q, veio %q", title, titles(list))
	}
	return list[0]
}

func none(t *testing.T, list []Embed) {
	t.Helper()
	if len(list) != 0 {
		t.Fatalf("não esperava aviso, veio %q", titles(list))
	}
}

// ---------- Um teste por tipo de aviso ----------

func TestNewServerError(t *testing.T) {
	e := newEnv(t)
	w := e.worker()
	w.Check(e.ctx)
	none(t, e.h.embeds())

	e.serverError("00aa11bb22cc33dd", "runtime error: index out of range [3] with length 3", 1, e.clk.Now())
	w.Check(e.ctx)
	got := one(t, e.h.embeds(), "Erro novo no servidor")
	if got.Color != colorCritical || got.URL != "https://erevia.test/admin#operacao" {
		t.Fatalf("cor ou link: %+v", got)
	}
	if !strings.Contains(got.Description, "index out of range") || !strings.Contains(got.Description, "useSkill skills.go") {
		t.Fatalf("descrição: %q", got.Description)
	}
	if field(got, "Onde") != "`msg`" || field(got, "Versão") != "`abc1234`" || !strings.Contains(field(got, "Jogador"), "UseSkill") {
		t.Fatalf("campos: %+v", got.Fields)
	}
	if got.Timestamp == "" || got.Footer == nil || !strings.Contains(got.Footer.Text, "abc1234") {
		t.Fatalf("rodapé e hora: %+v", got)
	}

	// O mesmo hash de novo não é novo.
	e.clk.Add(time.Minute)
	e.serverError("00aa11bb22cc33dd", "runtime error: index out of range [3] with length 3", 3, e.clk.Now())
	w.Check(e.ctx)
	none(t, e.h.embeds())
}

func TestNewClientError(t *testing.T) {
	e := newEnv(t)
	w := e.worker()
	w.Check(e.ctx)

	hash := e.clientError("TypeError: x is undefined", "v1", 1)
	w.Check(e.ctx)
	got := one(t, e.h.embeds(), "Erro novo no navegador")
	if got.Color != colorHigh || field(got, "Hash") != "`"+hash+"`" || field(got, "Região") != "Cidade" {
		t.Fatalf("aviso: %+v", got)
	}
	if !strings.Contains(got.Description, "TypeError: x is undefined") || !strings.Contains(got.Description, "```") {
		t.Fatalf("descrição: %q", got.Description)
	}

	// Mesmo hash noutra versão: já visto.
	e.clientError("TypeError: x is undefined", "v2", 1)
	w.Check(e.ctx)
	none(t, e.h.embeds())
}

func TestNewErrorsBurstIsSummarized(t *testing.T) {
	e := newEnv(t)
	w := e.worker()
	w.Check(e.ctx)
	for i := range 8 {
		e.clientError("Erro número "+string(rune('A'+i)), "v1", 1)
	}
	w.Check(e.ctx)
	got := e.h.embeds()
	if len(got) != newErrorsPerCheck+1 || !strings.HasPrefix(got[len(got)-1].Title, "Mais 3 erros novos") {
		t.Fatalf("esperava %d avisos e o resumo: %q", newErrorsPerCheck, titles(got))
	}
	w.Check(e.ctx)
	none(t, e.h.embeds()) // os do resumo também ficaram vistos
}

func TestSpikeServer(t *testing.T) {
	e := newEnv(t)
	w := e.worker()
	w.Check(e.ctx)
	ref := "ffee000011112222"
	e.serverError(ref, "nil map", 1, e.clk.Now().Add(-4*time.Minute))
	e.serverError(ref, "nil map", 15, e.clk.Now().Add(-2*time.Minute))
	e.serverError(ref, "nil map", 10, e.clk.Now())
	w.Check(e.ctx)
	got := e.h.embeds()
	if len(got) != 2 || got[0].Title != "Erro novo no servidor" || got[1].Title != "Pico de erros no servidor" {
		t.Fatalf("esperava erro novo e pico: %q", titles(got))
	}
	if !strings.Contains(got[1].Description, "**26 ocorrências** em 5 min") || got[1].Color != colorCritical {
		t.Fatalf("pico: %q", got[1].Description)
	}

	// Falha no banco também tem pico.
	for range 3 {
		e.event(store.GameEvent{At: e.clk.Now(), Kind: "db_error", Ref: "save_characters", Amount: 9, Extra: map[string]any{"err": "conn refused"}})
	}
	w.Check(e.ctx)
	got = e.h.embeds()
	if len(got) != 1 || got[0].Title != "Pico de falhas no banco" || field(got[0], "Operação") != "`save_characters`" {
		t.Fatalf("pico do banco: %q", titles(got))
	}
}

func TestSpikeClient(t *testing.T) {
	e := newEnv(t)
	w := e.worker()
	w.Check(e.ctx)

	hash := e.clientError("Falha no WebGL", "v1", 5)
	w.Check(e.ctx)
	one(t, e.h.embeds(), "Erro novo no navegador") // 5: ainda não é pico

	e.clk.Add(time.Minute)
	e.clientError("Falha no WebGL", "v1", 10)
	e.clientError("Falha no WebGL", "v2", 6) // outra versão soma no mesmo hash
	w.Check(e.ctx)
	got := one(t, e.h.embeds(), "Pico de erros no navegador")
	if !strings.Contains(got.Description, "**21 ocorrências**") || field(got, "Hash") != "`"+hash+"`" {
		t.Fatalf("pico: %q %+v", got.Description, got.Fields)
	}
}

func TestSpikeTracker(t *testing.T) {
	var tr spikeTracker
	t0 := time.Now()
	read, win := 6*time.Minute, 5*time.Minute
	row := func(total int64, first time.Time) []store.ClientErrorTotal {
		return []store.ClientErrorTotal{{Hash: "h", Total: total, FirstSeen: first}}
	}
	// Erro antigo na primeira leitura (reinício): só a base.
	if n := tr.observe(t0, read, win, row(1000, t0.Add(-time.Hour)))["h"]; n != 0 {
		t.Fatalf("base: %d", n)
	}
	for i := 1; i <= 5; i++ {
		tr.observe(t0.Add(time.Duration(i)*time.Minute), read, win, row(1000+int64(i)*10, t0.Add(-time.Hour)))
	}
	if n := tr.observe(t0.Add(6*time.Minute), read, win, row(1060, t0.Add(-time.Hour)))["h"]; n != 50 {
		t.Fatalf("5 min de 10 por minuto: %d", n)
	}
	// Sumiu da leitura (sem atividade) e voltou: a diferença é toda da verificação atual.
	for i := 7; i <= 15; i++ {
		tr.observe(t0.Add(time.Duration(i)*time.Minute), read, win, nil)
	}
	if n := tr.observe(t0.Add(16*time.Minute), read, win, row(1100, t0.Add(-time.Hour)))["h"]; n != 40 {
		t.Fatalf("voltou: %d", n)
	}
	// Verificações perdidas por mais que a janela de leitura: o total antigo não serve de base.
	if n := tr.observe(t0.Add(30*time.Minute), read, win, row(5000, t0.Add(-time.Hour)))["h"]; n != 0 {
		t.Fatalf("depois de um buraco: %d", n)
	}
}

func TestInvariantFirstPerRulePerDay(t *testing.T) {
	e := newEnv(t)
	w := e.worker()
	w.Check(e.ctx)
	inv := func(rule string, at time.Time) {
		e.event(store.GameEvent{
			At: at, Kind: "invariant", Ref: rule, CharacterID: 0, Level: 9, X: 12.5, Z: -40,
			Extra: map[string]any{"value": "hp 120 > max 100", "fixed": true, "version": "abc1234def", "name": "Aldric"},
		})
	}
	inv("hp", e.clk.Now())
	inv("hp", e.clk.Now())
	w.Check(e.ctx)
	got := one(t, e.h.embeds(), "Invariante quebrada: hp")
	if got.Color != colorWarn || field(got, "Registros hoje") != "2" || !strings.Contains(field(got, "Valor"), "hp 120") ||
		!strings.Contains(got.Description, "corrigiu") || field(got, "Região") == "" {
		t.Fatalf("aviso: %+v", got)
	}

	inv("hp", e.clk.Now())
	inv("gold", e.clk.Now())
	w.Check(e.ctx)
	one(t, e.h.embeds(), "Invariante quebrada: gold") // hp já foi avisada hoje

	e.clk.Add(24 * time.Hour) // outro dia: de novo
	inv("hp", e.clk.Now())
	w.Check(e.ctx)
	got2 := e.h.embeds()
	var inv2 []Embed
	for _, x := range got2 {
		if strings.HasPrefix(x.Title, "Invariante") {
			inv2 = append(inv2, x)
		}
	}
	one(t, inv2, "Invariante quebrada: hp")
}

func TestSlowTick(t *testing.T) {
	e := newEnv(t)
	w := e.worker()
	w.Check(e.ctx)
	now := e.clk.Now()
	slow := func(at time.Time, ms int64, n int) {
		e.event(store.GameEvent{At: at, Kind: "slow_tick", Ref: "monsters", Amount: ms, Extra: map[string]any{
			"systems": map[string]any{"monsters": 61.5, "players": 12.0}, "slow_ticks": n, "budget_ms": 50.0,
			"players": 40, "monsters": 900, "version": "abc1234def",
		}})
	}
	// Um tick lento solto não é p95 acima de 50 ms.
	slow(now.Add(-3*time.Minute), 80, 1)
	slow(now.Add(-2*time.Minute), 70, 2)
	slow(now.Add(-time.Minute), 90, 3)
	w.Check(e.ctx)
	none(t, e.h.embeds())

	// 2 min com ~25% dos ticks acima de 50 ms (20 ticks/s: 1200 por minuto).
	e.clk.Add(15 * time.Minute) // os de antes saem da janela de leitura
	now = e.clk.Now()
	slow(now.Add(-2*time.Minute-10*time.Second), 120, 300)
	slow(now.Add(-70*time.Second), 140, 300)
	slow(now.Add(-10*time.Second), 95, 300)
	w.Check(e.ctx)
	got := one(t, e.h.embeds(), "Tick lento na zona")
	if !strings.Contains(got.Description, "25% dos ticks") || !strings.Contains(got.Description, "**140 ms**") ||
		!strings.HasPrefix(field(got, "Sistemas (último tick lento)"), "`monsters 61.5 ms, players 12.0 ms") || field(got, "Jogadores") != "40" {
		t.Fatalf("aviso: %q %+v", got.Description, got.Fields)
	}
	// Continua lento um minuto depois: o limite segura.
	e.clk.Add(time.Minute)
	slow(e.clk.Now(), 100, 300)
	w.Check(e.ctx)
	none(t, e.h.embeds())
}

func TestFindSlowSpell(t *testing.T) {
	now := time.Now()
	ev := func(ago time.Duration, n float64) store.AlertEvent {
		return store.AlertEvent{At: now.Add(-ago), Amount: 60, Extra: map[string]any{"slow_ticks": n, "budget_ms": 50.0}}
	}
	// Lento há 2 min, mas o último slow_tick é velho: já passou.
	if _, ok := findSlowSpell([]store.AlertEvent{ev(6*time.Minute, 1), ev(5*time.Minute, 200), ev(4*time.Minute, 200)}, now); ok {
		t.Fatal("slow_tick velho não avisa")
	}
	// Só 1 min coberto.
	if _, ok := findSlowSpell([]store.AlertEvent{ev(70*time.Second, 1), ev(10*time.Second, 200)}, now); ok {
		t.Fatal("1 min não basta")
	}
	// 5% exatos não passa (p95 = 50 ms).
	if _, ok := findSlowSpell([]store.AlertEvent{ev(3*time.Minute, 1), ev(2*time.Minute, 60), ev(time.Minute, 60), ev(0, 60)}, now); ok {
		t.Fatal("5% não é p95 acima de 50 ms")
	}
	s, ok := findSlowSpell([]store.AlertEvent{ev(3*time.Minute, 1), ev(2*time.Minute, 61), ev(time.Minute, 61), ev(0, 61)}, now)
	if !ok || s.covered != 3*time.Minute {
		t.Fatalf("acima de 5%% por 3 min: %v %v", ok, s.covered)
	}
}

func TestBootDeployAndCrash(t *testing.T) {
	e := newEnv(t)
	boot := func(version string) *Worker {
		w := e.worker(func(c *Config) { c.Version = version; c.BootDelay = time.Minute })
		w.Check(e.ctx) // registra a subida; o aviso espera o BootDelay
		none(t, e.h.embeds())
		e.clk.Add(time.Minute)
		w.Check(e.ctx)
		return w
	}
	a := boot("aaaaaaa111")
	got := one(t, e.h.embeds(), "Servidor no ar")
	if field(got, "Versão") != "`aaaaaaa111`" || !strings.Contains(got.Description, "Primeira subida") || got.URL != "https://erevia.test/admin#inicio" {
		t.Fatalf("primeira subida: %+v", got)
	}
	a.stopped() // SIGTERM: desligou normalmente

	e.clk.Add(time.Minute)
	b := boot("bbbbbbb222")
	got = one(t, e.h.embeds(), "Deploy: servidor no ar")
	if got.Color != colorOK || !strings.Contains(got.Description, "`aaaaaaa`") || !strings.Contains(got.Description, "`bbbbbbb`") ||
		strings.Contains(got.Description, "não registrou") {
		t.Fatalf("deploy: %q", got.Description)
	}
	_ = b // b cai sem stopped()

	e.clk.Add(20 * time.Minute)
	boot("bbbbbbb222")
	got = one(t, e.h.embeds(), "Servidor caiu e voltou")
	if got.Color != colorCritical || !strings.Contains(got.Description, "não desligou normalmente") {
		t.Fatalf("queda: %q", got.Description)
	}

	// Caindo de novo em menos de 15 min: o limite segura; depois diz quantas subidas foram.
	e.clk.Add(2 * time.Minute)
	boot("bbbbbbb222")
	none(t, e.h.embeds())
	e.clk.Add(16 * time.Minute)
	boot("bbbbbbb222")
	got = one(t, e.h.embeds(), "Servidor caiu e voltou")
	if !strings.Contains(got.Description, "Mais 1 subida(s)") {
		t.Fatalf("contagem do limite: %q", got.Description)
	}
}

func TestBugReport(t *testing.T) {
	e := newEnv(t)
	w := e.worker()
	w.Check(e.ctx)
	acc := e.account("aldric")
	e.bug(acc, "O baú da praça some quando eu clico @everyone **rápido** [aqui](https://phish.test)")
	w.Check(e.ctx)
	got := one(t, e.h.embeds(), "Relato de bug #")
	if got.Color != colorInfo || got.URL != "https://erevia.test/admin#relatos" {
		t.Fatalf("cor ou link: %+v", got)
	}
	if strings.Contains(got.Description, "@everyone") || strings.Contains(got.Description, "**rápido**") ||
		strings.Contains(got.Description, "[aqui](") || !strings.Contains(got.Description, "O baú da praça some") {
		t.Fatalf("texto do jogador não pode formatar nem mencionar: %q", got.Description)
	}
	if field(got, "Personagem") != "Aldric (nível 12)" || field(got, "Posição") != "55.0, 10.0" ||
		field(got, "Região") != "Arredores" || field(got, "Versão") != "`abc1234`" {
		t.Fatalf("campos: %+v", got.Fields)
	}

	// Outro relato da mesma conta em menos de 15 min espera; outra conta não.
	e.bug(acc, "Segundo relato: a porta não abre")
	other := e.account("brena")
	e.bug(other, "Relato de outra conta, sem limite")
	w.Check(e.ctx)
	got = one(t, e.h.embeds(), "Relato de bug #")
	if !strings.Contains(got.Description, "outra conta") {
		t.Fatalf("a outra conta tinha de passar: %q", got.Description)
	}
	e.bug(acc, "Terceiro relato: caí no chão")
	e.clk.Add(16 * time.Minute)
	w.Check(e.ctx)
	list := e.h.take()
	if len(list) != 1 || len(list[0].Embeds) != 2 || !strings.Contains(list[0].Embeds[0].Description, "Segundo") ||
		!strings.Contains(list[0].Embeds[1].Description, "Terceiro") {
		t.Fatalf("os relatos segurados saem juntos numa mensagem: %+v", list)
	}
	if list[0].AllowedMentions.Parse == nil || len(list[0].AllowedMentions.Parse) != 0 {
		t.Fatalf("allowed_mentions tem de ser vazio: %+v", list[0].AllowedMentions)
	}
}

// ---------- Limite, "já visto" e reinício ----------

func TestLimit15Minutes(t *testing.T) {
	e := newEnv(t)
	w := e.worker()
	w.Check(e.ctx)
	ref := "aaaa0000bbbb1111"
	spike := func() {
		e.serverError(ref, "boom", 30, e.clk.Now())
		w.Check(e.ctx)
	}
	spike()
	if got := e.h.embeds(); len(got) != 2 { // erro novo + pico
		t.Fatalf("primeiro pico: %q", titles(got))
	}
	for range 3 { // 3 verificações seguidas em pico, dentro dos 15 min
		e.clk.Add(4 * time.Minute)
		spike()
		none(t, e.h.embeds())
	}
	e.clk.Add(3 * time.Minute) // 15 min depois do primeiro
	spike()
	got := one(t, e.h.embeds(), "Pico de erros no servidor")
	if !strings.Contains(got.Description, "3 verificação(ões) sem aviso") {
		t.Fatalf("o aviso seguinte diz quantos o limite segurou: %q", got.Description)
	}
	m, _, _ := e.st.AlertMark(e.ctx, "spike", "server:"+ref)
	if m.Suppressed != 0 || m.SentAt.Sub(e.clk.Now()).Abs() > time.Millisecond {
		t.Fatalf("limite recomeça no último envio: %+v", m)
	}
}

func TestSeenSurvivesRestart(t *testing.T) {
	e := newEnv(t)
	w1 := e.worker()
	w1.Check(e.ctx)
	acc := e.account("aldric")
	e.serverError("1234123412341234", "boom", 30, e.clk.Now())
	e.clientError("ReferenceError: y", "v1", 1)
	e.bug(acc, "Relato antes do reinício")
	e.event(store.GameEvent{At: e.clk.Now(), Kind: "invariant", Ref: "gold", Extra: map[string]any{"value": "-5"}})
	w1.Check(e.ctx)
	if got := e.h.embeds(); len(got) != 5 { // 2 erros novos, pico, invariante, relato
		t.Fatalf("antes do reinício: %q", titles(got))
	}

	// Outro processo (reinício): nada se repete, nem o pico (o limite está no banco).
	e.clk.Add(time.Minute)
	w2 := e.worker()
	w2.Check(e.ctx)
	none(t, e.h.embeds())
	m, ok, _ := e.st.AlertMark(e.ctx, "spike", "server:1234123412341234")
	if !ok || m.Suppressed != 1 {
		t.Fatalf("o pico segurado foi contado no banco: %+v", m)
	}
}

func TestBootstrapIgnoresWhatExisted(t *testing.T) {
	e := newEnv(t)
	acc := e.account("aldric")
	e.serverError("9999888877776666", "antigo", 1, e.clk.Now())
	e.clientError("Erro antigo", "v1", 1)
	e.bug(acc, "Relato de antes dos alertas")
	w := e.worker()
	w.Check(e.ctx)
	none(t, e.h.embeds())
	e.bug(acc, "Relato depois dos alertas")
	w.Check(e.ctx)
	got := one(t, e.h.embeds(), "Relato de bug #")
	if !strings.Contains(got.Description, "depois") {
		t.Fatalf("relato: %q", got.Description)
	}
}

// ---------- Envio: 429, erros e segredo ----------

func Test429RespectsRetryAfter(t *testing.T) {
	h := newHook(t)
	h.program(
		reply{status: 429, body: `{"message":"You are being rate limited.","retry_after":1.5,"global":false}`},
		reply{status: 429, body: `oops`, header: map[string]string{"Retry-After": "2"}},
	)
	d := newDiscord(h.url(), nil)
	var waits []time.Duration
	d.sleep = func(ctx context.Context, w time.Duration) error {
		if w > 0 {
			waits = append(waits, w)
		}
		return nil
	}
	if err := d.send(context.Background(), newMessage([]Embed{{Title: "oi"}})); err != nil {
		t.Fatal(err)
	}
	if len(waits) != 2 || waits[0] != 1500*time.Millisecond || waits[1] != 2*time.Second {
		t.Fatalf("esperas: %v", waits)
	}
	if got := h.take(); len(got) != 1 || got[0].Embeds[0].Title != "oi" {
		t.Fatalf("entregue uma vez: %+v", got)
	}
}

func Test429TooLongRetriesNextCheck(t *testing.T) {
	e := newEnv(t)
	w := e.worker()
	w.Check(e.ctx)
	e.serverError("5555666677778888", "boom", 1, e.clk.Now())
	e.h.program(reply{status: 429, body: `{"retry_after": 120}`})
	w.Check(e.ctx)
	none(t, e.h.embeds())
	if !strings.Contains(e.log.String(), "429") {
		t.Fatalf("o 429 vai para o log: %s", e.log.String())
	}
	e.clk.Add(time.Minute)
	w.Check(e.ctx)
	one(t, e.h.embeds(), "Erro novo no servidor") // não foi marcado como visto: sai na próxima
}

func TestRealSleepOn429(t *testing.T) {
	h := newHook(t)
	h.program(reply{status: 429, body: `{"retry_after": 0.2}`})
	d := newDiscord(h.url(), nil)
	start := time.Now()
	if err := d.send(context.Background(), newMessage([]Embed{{Title: "oi"}})); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took < 200*time.Millisecond {
		t.Fatalf("não esperou o retry_after: %s", took)
	}
}

func TestBucketEmptyWaits(t *testing.T) {
	h := newHook(t)
	h.program(reply{status: 204, header: map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset-After": "0.75"}})
	d := newDiscord(h.url(), nil)
	var waits []time.Duration
	d.sleep = func(ctx context.Context, w time.Duration) error {
		if w > 0 {
			waits = append(waits, w)
		}
		return nil
	}
	now := time.Now()
	d.now = func() time.Time { return now }
	d.send(context.Background(), newMessage([]Embed{{Title: "1"}}))
	d.send(context.Background(), newMessage([]Embed{{Title: "2"}}))
	if len(waits) != 1 || waits[0] != 750*time.Millisecond {
		t.Fatalf("esperar o balde: %v", waits)
	}
}

func TestServerErrorRetryAndPermanent(t *testing.T) {
	h := newHook(t)
	h.program(reply{status: 502}, reply{status: 503})
	d := newDiscord(h.url(), nil)
	d.sleep = func(ctx context.Context, w time.Duration) error { return nil }
	if err := d.send(context.Background(), newMessage([]Embed{{Title: "oi"}})); err != nil {
		t.Fatalf("5xx tenta de novo: %v", err)
	}
	h.program(reply{status: 400, body: `{"embeds": ["0"]}`})
	err := d.send(context.Background(), newMessage([]Embed{{Title: "oi"}}))
	if _, ok := err.(*permanentError); !ok {
		t.Fatalf("400 é definitivo: %v", err)
	}
}

func TestTimeoutAndSecretNotLogged(t *testing.T) {
	block := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-block }))
	defer slow.Close()
	defer close(block)
	e := newEnv(t)
	w := e.worker(func(c *Config) {
		c.WebhookURL = slow.URL + "/api/webhooks/123/token-secreto"
		c.HTTP = &http.Client{Timeout: 100 * time.Millisecond}
	})
	w.Check(e.ctx)
	e.serverError("abababababababab", "boom", 1, e.clk.Now())
	start := time.Now()
	w.Check(e.ctx)
	if took := time.Since(start); took > 10*time.Second {
		t.Fatalf("o timeout não segurou: %s", took)
	}
	logs := e.log.String()
	if !strings.Contains(logs, "Timeout") && !strings.Contains(logs, "deadline") {
		t.Fatalf("o erro vai para o log: %s", logs)
	}
	if strings.Contains(logs, "token-secreto") || strings.Contains(logs, "/api/webhooks") {
		t.Fatalf("a URL do webhook vazou no log: %s", logs)
	}
}

func TestNoWebhookOnlyLogs(t *testing.T) {
	e := newEnv(t)
	w := e.worker(func(c *Config) { c.WebhookURL = "" })
	w.Check(e.ctx)
	e.serverError("cdcdcdcdcdcdcdcd", "boom sem webhook", 1, e.clk.Now())
	w.Check(e.ctx)
	if !strings.Contains(e.log.String(), "Erro novo no servidor") || !strings.Contains(e.log.String(), "boom sem webhook") {
		t.Fatalf("sem webhook, o aviso vai para o log: %s", e.log.String())
	}
	e.h.mu.Lock()
	calls := e.h.calls
	e.h.mu.Unlock()
	if calls != 0 {
		t.Fatal("sem webhook não manda nada")
	}
}

// ---------- Panic no worker ----------

type panicky struct {
	Store
	n atomic.Int32
}

func (p *panicky) UnseenServerErrors(ctx context.Context, since time.Time, limit int) ([]store.AlertEvent, error) {
	if p.n.Add(1) == 1 {
		var m map[string]int
		m["x"] = 1 // panic de verdade
	}
	return p.Store.UnseenServerErrors(ctx, since, limit)
}

type reporter struct {
	mu    sync.Mutex
	where []string
}

func (r *reporter) ReportPanic(where string, playerID uint32, msg string, v any, stack []byte) {
	r.mu.Lock()
	r.where = append(r.where, where)
	r.mu.Unlock()
}

func TestPanicIsReportedAndWorkerRestarts(t *testing.T) {
	e := newEnv(t)
	st := &panicky{Store: e.st}
	rep := &reporter{}
	w := New(Config{WebhookURL: e.h.url(), Every: 20 * time.Millisecond, BootDelay: time.Hour}, st, rep, slog.New(slog.NewTextHandler(e.log, nil)))
	ctx, cancel := context.WithCancel(e.ctx)
	done := w.Start(ctx)
	deadline := time.Now().Add(10 * time.Second)
	for st.n.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("o worker não terminou")
	}
	rep.mu.Lock()
	defer rep.mu.Unlock()
	if len(rep.where) != 1 || rep.where[0] != "worker:alerts" {
		t.Fatalf("o panic é registrado uma vez: %v", rep.where)
	}
	if st.n.Load() < 3 {
		t.Fatalf("o worker recomeçou e seguiu verificando: %d", st.n.Load())
	}
	// Desligou normalmente: a subida ficou anotada como limpa.
	m, ok, _ := e.st.AlertMark(context.Background(), "boot", w.bootID)
	if !ok || m.Info["clean"] != true {
		t.Fatalf("desligamento normal anotado: %+v", m)
	}
}

// ---------- Formato ----------

func TestEmbedLimits(t *testing.T) {
	long := strings.Repeat("a", 5000)
	fields := make([]Field, 30)
	for i := range fields {
		fields[i] = Field{Name: long, Value: long}
	}
	m := newMessage([]Embed{{Title: long, Description: "```\n" + long, Fields: fields, Footer: &Footer{Text: long}}, {Title: "b", Fields: []Field{{Name: "vazio", Value: " "}}}})
	total := 0
	for _, e := range m.Embeds {
		total += embedSize(e)
		if len([]rune(e.Title)) > maxTitle || len([]rune(e.Description)) > maxDesc || len(e.Fields) > maxFields {
			t.Fatalf("limites: %d %d %d", len(e.Title), len(e.Description), len(e.Fields))
		}
		for _, f := range e.Fields {
			if len([]rune(f.Value)) > maxFieldValue || strings.TrimSpace(f.Value) == "" {
				t.Fatalf("campo: %q", f.Value)
			}
		}
	}
	if total > maxEmbedTotal {
		t.Fatalf("total %d > %d", total, maxEmbedTotal)
	}
	if strings.Count(m.Embeds[0].Description, "```")%2 != 0 {
		t.Fatalf("bloco de código cortado tem de fechar: %q", m.Embeds[0].Description[len(m.Embeds[0].Description)-10:])
	}
	if m.Embeds[1].Fields[0].Value != "—" {
		t.Fatalf("campo vazio: %q", m.Embeds[1].Fields[0].Value)
	}
	if got := code("a```b"); strings.Count(got, "```") != 2 {
		t.Fatalf("crases dentro do bloco: %q", got)
	}
}
