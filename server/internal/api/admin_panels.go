package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"erevia/server/internal/store"
)

// Portal de administração, parte de leitura: Relatos (admin_reports.go), Métricas
// (admin_metrics.go) e Economia (admin_economy.go). Tudo atrás do AdminOnly, como o resto
// do portal; a única escrita (marcar relato como resolvido) passa pelo audit.

// panelCacheTTL: por quanto tempo as consultas pesadas (gráficos e economia) valem.
const panelCacheTTL = time.Minute

// panelQueryTimeout: prazo de uma rodada de consultas de um painel (feita uma vez por
// minuto, sem depender de quem pediu continuar esperando).
const panelQueryTimeout = 15 * time.Second

type panels struct {
	a      *API
	cache  *ttlCache  // os painéis inteiros, por 1 minuto
	days   *dayCache  // os dias que já acabaram, sem prazo
	totals totalsCache // a telemetria de antes de hoje (economia)
}

// registerAdminPanels monta as rotas de Relatos, Métricas e Economia (chamado por
// registerAdmin, no mux que já está atrás do AdminOnly).
func (a *API) registerAdminPanels(m *http.ServeMux) {
	p := &panels{a: a, cache: newTTLCache(panelCacheTTL), days: &dayCache{m: map[string]any{}}}
	// Relatos (#107)
	m.HandleFunc("GET /api/admin/bugs", p.handleBugs)
	m.HandleFunc("GET /api/admin/bugs/{id}", p.handleBug)
	m.HandleFunc("POST /api/admin/bugs/{id}/resolve", p.handleResolveBug)
	m.HandleFunc("GET /api/admin/client-errors", p.handleClientErrors)
	// Métricas (#108)
	m.HandleFunc("GET /api/admin/metrics/live", p.handleMetricsLive)
	m.HandleFunc("GET /api/admin/metrics", p.handleMetrics)
	// Economia e auditoria (#109)
	m.HandleFunc("GET /api/admin/economy", p.handleEconomy)
	m.HandleFunc("GET /api/admin/economy/character", p.handleEconomyCharacter)
	m.HandleFunc("GET /api/admin/economy/gm-log", p.handleEconomyGMLog)
}

// periodDays lê ?days= (1, 7, 30 ou 90; o resto vira 7).
func periodDays(r *http.Request) int {
	switch d, _ := strconv.Atoi(r.URL.Query().Get("days")); d {
	case 1, 7, 30, 90:
		return d
	}
	return 7
}

// since é o começo do período: meia-noite (UTC) de days−1 dias atrás, para os dias dos
// gráficos saírem inteiros.
func since(now time.Time, days int) time.Time {
	d := now.UTC().Truncate(24 * time.Hour)
	return d.AddDate(0, 0, -(days - 1))
}

func queryInt64(r *http.Request, key string) int64 {
	v, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get(key)), 10, 64)
	return max(v, 0)
}

// timings mede cada consulta de um painel (o portal mostra, para ver o que pesa).
type timings map[string]float64

func (t timings) run(name string, fn func() error) error {
	start := time.Now()
	err := fn()
	t[name] = float64(time.Since(start).Microseconds()) / 1000
	return err
}

// ---------- Cache ----------

// ttlCache guarda o resultado de uma consulta por chave durante ttl. Pedidos iguais ao
// mesmo tempo esperam a mesma consulta (uma só vai ao banco). Erro não fica guardado.
type ttlCache struct {
	ttl     time.Duration
	now     func() time.Time
	mu      sync.Mutex
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	done chan struct{} // fechado quando val/err ficam prontos
	at   time.Time
	val  any
	err  error
}

func newTTLCache(ttl time.Duration) *ttlCache {
	return &ttlCache{ttl: ttl, now: time.Now, entries: map[string]*cacheEntry{}}
}

// get devolve o valor da chave (e quando foi calculado), calculando com fn se não houver
// um válido. fn roda com um contexto próprio: quem pediu pode desistir sem estragar o
// resultado para os outros.
func (c *ttlCache) get(ctx context.Context, key string, fn func(ctx context.Context) (any, error)) (any, time.Time, error) {
	c.mu.Lock()
	e := c.entries[key]
	if e != nil {
		select {
		case <-e.done:
			if e.err != nil || c.now().Sub(e.at) >= c.ttl {
				e = nil
			}
		default: // ainda calculando: espera o mesmo
		}
	}
	if e == nil {
		for k, old := range c.entries { // limpa os vencidos (poucas chaves: período × painel)
			select {
			case <-old.done:
				if c.now().Sub(old.at) >= c.ttl {
					delete(c.entries, k)
				}
			default:
			}
		}
		e = &cacheEntry{done: make(chan struct{})}
		c.entries[key] = e
		go func() {
			qctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), panelQueryTimeout)
			defer cancel()
			v, err := fn(qctx)
			c.mu.Lock()
			e.val, e.err, e.at = v, err, c.now()
			if err != nil && c.entries[key] == e {
				delete(c.entries, key)
			}
			c.mu.Unlock()
			close(e.done)
		}()
	}
	c.mu.Unlock()
	select {
	case <-e.done:
		return e.val, e.at, e.err
	case <-ctx.Done():
		return nil, time.Time{}, ctx.Err()
	}
}

// ---------- Dias fechados ----------

// lateEvents: a telemetria grava em lotes a cada ~2 s; um dia só é dado como fechado (e
// guardado de vez) alguns minutos depois da meia-noite, quando nada dele chega mais.
const lateEvents = 5 * time.Minute

// keepDays: dias guardados no dayCache (o maior período do portal é de 90 dias).
const keepDays = 100

// closedBefore: os dias antes deste instante (meia-noite UTC) já acabaram.
func closedBefore(now time.Time) time.Time { return store.DayStart(now.Add(-lateEvents)) }

// dayCache guarda o resultado de cada consulta por dia para os dias que já acabaram: o
// passado não muda, então só o dia de hoje volta ao banco a cada minuto.
type dayCache struct {
	mu sync.Mutex
	m  map[string]any // "consulta|AAAA-MM-DD" → []T
}

func (c *dayCache) get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[key]
	return v, ok
}

func (c *dayCache) put(key string, v any, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = v
	oldest := store.DayStart(now).AddDate(0, 0, -keepDays).Format("2006-01-02")
	for k := range c.m {
		if i := strings.LastIndexByte(k, '|'); i >= 0 && k[i+1:] < oldest {
			delete(c.m, k)
		}
	}
}

// queryStats conta as consultas de verdade (ao banco) de cada família e a mais lenta.
type queryStats struct {
	count   map[string]int
	slowest timings
}

func newQueryStats() *queryStats { return &queryStats{count: map[string]int{}, slowest: timings{}} }

func (q *queryStats) measure(name string, fn func() error) error {
	start := time.Now()
	err := fn()
	ms := float64(time.Since(start).Microseconds()) / 1000
	q.count[name]++
	q.slowest[name] = max(q.slowest[name], ms)
	return err
}

// cachedDays devolve as linhas de [from, to) (meias-noites UTC): cada dia fechado vem do
// cache ou de uma consulta só dele; os dias ainda abertos (hoje) vêm de uma consulta.
func cachedDays[T any](ctx context.Context, p *panels, q *queryStats, name string, from, to time.Time,
	fetch func(ctx context.Context, from, to time.Time) ([]T, error)) ([]T, error) {
	now := time.Now()
	closed := closedBefore(now)
	var out []T
	for d := from; d.Before(to); d = d.Add(24 * time.Hour) {
		if !d.Before(closed) {
			var rows []T
			if err := q.measure(name, func() (err error) { rows, err = fetch(ctx, d, to); return err }); err != nil {
				return nil, err
			}
			return append(out, rows...), nil
		}
		key := name + "|" + d.Format("2006-01-02")
		if v, ok := p.days.get(key); ok {
			out = append(out, v.([]T)...)
			continue
		}
		var rows []T
		if err := q.measure(name, func() (err error) { rows, err = fetch(ctx, d, d.Add(24*time.Hour)); return err }); err != nil {
			return nil, err
		}
		p.days.put(key, rows, now)
		out = append(out, rows...)
	}
	return out, nil
}

// totalsCache guarda a telemetria de ouro e itens de antes de hoje (uma conta por dia).
type totalsCache struct {
	mu  sync.Mutex
	val *store.TelemetryTotals
}

// before devolve os totais até a meia-noite dos dias fechados (calcula na primeira vez
// do dia).
func (p *panels) before(ctx context.Context, q *queryStats) (store.TelemetryTotals, error) {
	cut := closedBefore(time.Now())
	p.totals.mu.Lock()
	defer p.totals.mu.Unlock()
	if v := p.totals.val; v != nil && v.Cut.Equal(cut) {
		return *v, nil
	}
	var t store.TelemetryTotals
	err := q.measure("before", func() (err error) { t, err = p.a.store.TelemetryTotalsBefore(ctx, cut); return err })
	if err != nil {
		return t, err
	}
	p.totals.val = &t
	return t, nil
}
