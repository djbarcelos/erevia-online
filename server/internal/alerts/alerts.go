// Package alerts manda avisos ao Discord (webhook em ALERT_WEBHOOK_URL) sobre o que
// acontece no servidor: erro novo, pico de erros, invariante quebrada, tick lento,
// servidor no ar (deploy ou queda) e relato de /bug.
//
// Um worker próprio, fora da zona, consulta o banco a cada minuto (Config.Every). Ele só
// lê as tabelas que a zona e a API já gravam (game_events, client_errors, bug_reports) e
// guarda o que já avisou em alert_state, então nada aqui espera pela zona nem a trava.
//
// Limite: no máximo 1 aviso por tipo e chave a cada 15 min (Config.Limit). Os avisos
// segurados pelo limite são contados e o próximo diz quantos foram. O "já visto" (erros,
// invariantes do dia, relatos) e o limite ficam no banco e valem depois de um reinício.
//
// Sem ALERT_WEBHOOK_URL (desenvolvimento), o aviso só vai para o log. A URL nunca vai para
// o log: tem o token do webhook.
package alerts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"erevia/server/internal/store"
)

// Config do worker. Os campos zerados usam o padrão (ver New).
type Config struct {
	WebhookURL string // ALERT_WEBHOOK_URL; vazio = só log
	PublicURL  string // base dos links do portal (PUBLIC_URL)
	Version    string // commit no ar (RAILWAY_GIT_COMMIT_SHA)

	SpikeN      int64         // pico: mais de SpikeN ocorrências de um erro em SpikeWindow (ALERT_SPIKE_N, padrão 50)
	SpikeWindow time.Duration // padrão 5 min
	Every       time.Duration // intervalo entre as verificações (padrão 1 min)
	Limit       time.Duration // 1 aviso por tipo e chave nesse intervalo (padrão 15 min)
	// BootDelay: espera antes do aviso de servidor no ar, para o servidor anterior terminar
	// de desligar (no deploy da Railway os dois ficam no ar juntos por um tempo) e o aviso
	// saber se ele desligou normalmente (padrão 90 s).
	BootDelay time.Duration

	Now  func() time.Time // relógio (testes)
	HTTP *http.Client     // padrão: timeout de 10 s
}

// FromEnv lê ALERT_WEBHOOK_URL e ALERT_SPIKE_N.
func FromEnv(publicURL, version string, log *slog.Logger) Config {
	cfg := Config{WebhookURL: strings.TrimSpace(os.Getenv("ALERT_WEBHOOK_URL")), PublicURL: publicURL, Version: version}
	if v := os.Getenv("ALERT_SPIKE_N"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			cfg.SpikeN = n
		} else {
			log.Error("ALERT_SPIKE_N inválida (inteiro > 0), usando o padrão", "value", v)
		}
	}
	return cfg
}

// Store é o que o worker lê e grava no banco (*store.Store).
type Store interface {
	AlertBootstrap(ctx context.Context, at time.Time) (bool, error)
	AlertMark(ctx context.Context, kind, key string) (store.AlertMark, bool, error)
	LatestAlertMark(ctx context.Context, kind, except string) (store.AlertMark, bool, error)
	SaveAlertMark(ctx context.Context, m store.AlertMark) error
	SuppressAlert(ctx context.Context, kind, key string) error
	SetAlertPos(ctx context.Context, kind, key string, pos int64) error
	MarkAlertSeen(ctx context.Context, keys []string, at time.Time) error
	PruneAlertMarks(ctx context.Context, before time.Time) error
	UnseenClientErrors(ctx context.Context, since time.Time, limit int) ([]store.AlertClientError, error)
	ClientErrorTotals(ctx context.Context, since time.Time) ([]store.ClientErrorTotal, error)
	UnseenServerErrors(ctx context.Context, since time.Time, limit int) ([]store.AlertEvent, error)
	ErrorSpikes(ctx context.Context, since time.Time, over int64) ([]store.AlertEvent, error)
	InvariantsSince(ctx context.Context, since time.Time) ([]store.AlertEvent, error)
	SlowTicks(ctx context.Context, since time.Time) ([]store.AlertEvent, error)
	PendingBugReports(ctx context.Context, since time.Time, limit int) ([]store.AlertBug, error)
}

// PanicReporter registra um panic recuperado (a zona: server_error na telemetria).
type PanicReporter interface {
	ReportPanic(where string, playerID uint32, msg string, r any, stack []byte)
}

// Worker é o verificador dos alertas.
type Worker struct {
	cfg Config
	st  Store
	rep PanicReporter
	log *slog.Logger
	out *discord // nil = só log

	bootID     string
	bootAt     time.Time
	prevBoot   string // chave da subida anterior (alert_state boot)
	registered bool // a subida já está em alert_state
	bootDone   bool // o aviso de servidor no ar já foi tratado
	ready      bool // AlertBootstrap feito
	lastPrune  time.Time
	clients    spikeTracker
}

// New monta o worker (sem começar).
func New(cfg Config, st Store, rep PanicReporter, log *slog.Logger) *Worker {
	def := func(v *time.Duration, d time.Duration) {
		if *v <= 0 {
			*v = d
		}
	}
	def(&cfg.Every, time.Minute)
	def(&cfg.Limit, 15*time.Minute)
	def(&cfg.SpikeWindow, 5*time.Minute)
	if cfg.BootDelay == 0 {
		cfg.BootDelay = 90 * time.Second
	}
	if cfg.SpikeN <= 0 {
		cfg.SpikeN = 50
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Version == "" {
		cfg.Version = "dev"
	}
	cfg.PublicURL = strings.TrimRight(cfg.PublicURL, "/")
	w := &Worker{cfg: cfg, st: st, rep: rep, log: log, bootAt: cfg.Now()}
	if cfg.WebhookURL != "" {
		w.out = newDiscord(cfg.WebhookURL, cfg.HTTP)
	}
	var b [6]byte
	_, _ = rand.Read(b[:])
	w.bootID = strconv.FormatInt(w.bootAt.Unix(), 10) + "-" + hex.EncodeToString(b[:])
	return w
}

// Start roda o worker numa goroutine até ctx acabar. O canal fecha quando ele terminou
// (já com o desligamento normal anotado).
func Start(ctx context.Context, cfg Config, st Store, rep PanicReporter, log *slog.Logger) <-chan struct{} {
	return New(cfg, st, rep, log).Start(ctx)
}

// Start: ver a função Start. Um panic é registrado (rep) e o worker recomeça depois de 1 s,
// como os workers da zona (goWorker).
func (w *Worker) Start(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	if w.out == nil {
		w.log.Info("alertas só no log (ALERT_WEBHOOK_URL vazia)")
	} else {
		w.log.Info("alertas no Discord ligados", "spike_n", w.cfg.SpikeN)
	}
	go func() {
		defer close(done)
		for w.runSafe(ctx) && sleepCtx(ctx, time.Second) == nil {
		}
		w.stopped()
	}()
	return done
}

// runSafe devolve true se o worker terminou por panic.
func (w *Worker) runSafe(ctx context.Context) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			w.log.Error("panic no worker de alertas", "panic", r)
			if w.rep != nil {
				w.rep.ReportPanic("worker:alerts", 0, "", r, stack)
			}
			panicked = true
		}
	}()
	w.run(ctx)
	return false
}

func (w *Worker) run(ctx context.Context) {
	tick := time.NewTicker(w.cfg.Every)
	defer tick.Stop()
	for {
		w.Check(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// Check faz uma verificação completa (o worker chama a cada Every; os testes, direto).
func (w *Worker) Check(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	// Uma verificação nunca passa do intervalo (o envio espera no máximo 30 s por um 429).
	ctx, cancel := context.WithTimeout(ctx, max(w.cfg.Every-5*time.Second, 50*time.Second))
	defer cancel()
	now := w.cfg.Now()
	if !w.ready {
		first, err := w.st.AlertBootstrap(ctx, now)
		if err != nil {
			w.log.Warn("alertas: preparar", "err", err)
			return
		}
		if first {
			w.log.Info("alertas: primeira vez; os erros e relatos que já existiam contam como vistos")
		}
		w.ready = true
	}
	checks := []struct {
		name string
		fn   func(context.Context, time.Time) error
	}{
		{"boot", w.checkBoot},
		{"new_error", w.checkNewErrors},
		{"spike", w.checkSpikes},
		{"invariant", w.checkInvariants},
		{"slow_tick", w.checkSlowTick},
		{"bug", w.checkBugs},
		{"prune", w.prune},
	}
	for _, c := range checks {
		if ctx.Err() != nil {
			return
		}
		if err := c.fn(ctx, now); err != nil {
			w.log.Warn("alertas: verificação", "check", c.name, "err", err)
		}
	}
}

// ---------- Limite e envio ----------

// gate: o aviso (kind, key) pode sair agora? Devolve também quantos iguais o limite já
// segurou (para dizer no próximo). Não conta nada.
func (w *Worker) gate(ctx context.Context, kind, key string, now time.Time) (store.AlertMark, bool, error) {
	m, found, err := w.st.AlertMark(ctx, kind, key)
	if err != nil {
		return m, false, err
	}
	return m, !found || now.Sub(m.SentAt) >= w.cfg.Limit, nil
}

// limited manda o aviso se o limite deixa; senão conta como segurado. Depois de mandar,
// grava a hora (o limite começa a contar).
func (w *Worker) limited(ctx context.Context, kind, key string, now time.Time, build func(held int) []Embed) error {
	m, ok, err := w.gate(ctx, kind, key, now)
	if err != nil {
		return err
	}
	if !ok {
		return w.st.SuppressAlert(ctx, kind, key)
	}
	if err := w.send(ctx, build(m.Suppressed)); err != nil {
		return err
	}
	return w.st.SaveAlertMark(ctx, store.AlertMark{Kind: kind, Key: key, SentAt: now, Pos: m.Pos, Info: m.Info})
}

// send manda uma mensagem (até 10 embeds). Recusa definitiva do Discord (400, 404...) vai
// para o log e conta como enviada (tentar de novo a cada minuto não resolve); erro de
// rede ou limite volta como erro e o aviso fica para a próxima verificação.
func (w *Worker) send(ctx context.Context, embeds []Embed) error {
	if len(embeds) == 0 {
		return nil
	}
	ts := w.cfg.Now().UTC().Format(time.RFC3339)
	for i := range embeds {
		if embeds[i].Timestamp == "" {
			embeds[i].Timestamp = ts
		}
		if embeds[i].Footer == nil {
			embeds[i].Footer = &Footer{Text: "Erevia · versão " + short(w.cfg.Version)}
		}
	}
	msg := newMessage(embeds)
	if w.out == nil {
		for _, e := range msg.Embeds {
			w.log.Info("alerta (sem ALERT_WEBHOOK_URL)", "title", e.Title, "text", oneLine(e.Description, 300), "url", e.URL)
		}
		return nil
	}
	err := w.out.send(ctx, msg)
	var perm *permanentError
	if errors.As(err, &perm) {
		w.log.Error("alertas: o Discord recusou o aviso", "status", perm.status, "body", perm.body, "title", msg.Embeds[0].Title)
		return nil
	}
	return err
}

// link: endereço da seção do portal.
func (w *Worker) link(section string) string { return w.cfg.PublicURL + "/admin#" + section }

func short(v string) string {
	if len(v) > 7 {
		return v[:7]
	}
	if v == "" {
		return "dev"
	}
	return v
}

func oneLine(s string, n int) string {
	return cut(strings.Join(strings.Fields(s), " "), n)
}
