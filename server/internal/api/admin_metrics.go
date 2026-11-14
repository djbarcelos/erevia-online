package api

import (
	"context"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"erevia/server/internal/store"
	"erevia/server/internal/zone"
)

// Métricas (#108): a zona ao vivo (pelo canal, com prazo) e os gráficos da telemetria
// (do banco, guardados por 1 minuto; os dias fechados ficam no dayCache).

// MetricsZone é o retrato ao vivo da zona. O *zone.Zone que o cmd/zone passa em
// AdminConfig.Zone implementa; sem ele, o painel mostra só a versão e o uptime.
type MetricsZone interface {
	LiveStats(ctx context.Context) (zone.LiveStats, error)
}

// handleMetricsLive: GET /api/admin/metrics/live. Não guarda nada: o pedido à zona é uma
// contagem do que está em memória (ver zone/admin_metrics.go).
func (p *panels) handleMetricsLive(w http.ResponseWriter, r *http.Request) {
	cfg := p.a.admin.cfg
	var live *zone.LiveStats // null: a zona não respondeu no prazo
	if z, ok := cfg.Zone.(MetricsZone); ok {
		if st, err := z.LiveStats(r.Context()); err == nil {
			live = &st
		} else {
			p.a.log.Warn("admin live stats", "err", err)
		}
	}
	now := time.Now()
	writeJSON(w, http.StatusOK, map[string]any{
		"version":    cfg.Version,
		"startedAt":  cfg.StartedAt.UTC(),
		"uptimeSec":  int64(now.Sub(cfg.StartedAt).Seconds()),
		"serverTime": now.UTC(),
		"regions":    zone.Regions,
		"zone":       live,
	})
}

// DeathRow: mortes no período por região e por quem matou (v_deaths_by_region).
type DeathRow struct {
	Region   string  `json:"region"`
	Cause    string  `json:"cause"`
	Deaths   int64   `json:"deaths"`
	AvgLevel float64 `json:"avgLevel"`
}

// FPSRow: FPS médio e dos piores momentos no período por região e aparelho (v_fps).
type FPSRow struct {
	Region  string  `json:"region"`
	Device  string  `json:"device"`
	Samples int64   `json:"samples"`
	AvgFPS  float64 `json:"avgFps"`
	AvgLow  float64 `json:"avgLow"`
}

// MetricsReport é o conteúdo dos gráficos de um período.
type MetricsReport struct {
	Days       int                    `json:"days"`
	Since      time.Time              `json:"since"`
	Gold       []store.GoldDayRow     `json:"gold"`
	Levels     []store.LevelTimeRow   `json:"levels"`
	Deaths     []DeathRow             `json:"deaths"`
	Quit       []store.LevelCount     `json:"quit"`
	FPS        []FPSRow               `json:"fps"`
	ItemsDaily []store.ItemsDayRow    `json:"itemsDaily"`
	ItemsTop   []store.ItemFlowRow    `json:"itemsTop"`
	ServerDays []store.ServerEventDay `json:"serverDays"`
	ServerTop  []store.ServerEventTop `json:"serverTop"`
	Players    []store.PlayersDay     `json:"players"`
	// TimingsMs: quanto cada família levou nesta rodada (com o cache dos dias fechados);
	// SlowestMs: a consulta ao banco mais lenta de cada uma; Queries: quantas foram ao banco.
	TimingsMs   timings        `json:"timingsMs"`
	SlowestMs   timings        `json:"slowestMs"`
	Queries     map[string]int `json:"queries"`
	GeneratedAt time.Time      `json:"generatedAt"`
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// MetricsReport roda as consultas dos gráficos (uma de cada vez: o banco é o mesmo do jogo,
// e a rodada só acontece uma vez por minuto). Os dias fechados vêm do dayCache.
func (p *panels) MetricsReport(ctx context.Context, days int) (*MetricsReport, error) {
	now := time.Now()
	from, to := since(now, days), store.DayStart(now).Add(24*time.Hour)
	st := p.a.store
	q := newQueryStats()
	rep := &MetricsReport{Days: days, Since: from, TimingsMs: timings{}, SlowestMs: q.slowest, Queries: q.count}
	steps := []struct {
		name string
		fn   func() error
	}{
		{"gold", func() (err error) { rep.Gold, err = cachedDays(ctx, p, q, "gold", from, to, st.GoldDaily); return err }},
		{"levels", func() error {
			return q.measure("levels", func() (err error) { rep.Levels, err = st.LevelTimes(ctx, from); return err })
		}},
		{"deaths", func() error {
			rows, err := cachedDays(ctx, p, q, "deaths", from, to, st.DeathsDaily)
			rep.Deaths = mergeDeaths(rows)
			return err
		}},
		{"quit", func() error {
			return q.measure("quit", func() (err error) { rep.Quit, err = st.QuitLevel(ctx); return err })
		}},
		{"fps", func() error {
			rows, err := cachedDays(ctx, p, q, "fps", from, to, st.FPSDaily)
			rep.FPS = mergeFPS(rows)
			return err
		}},
		{"items", func() error {
			var err error
			if rep.ItemsDaily, err = cachedDays(ctx, p, q, "items", from, to, st.ItemsDaily); err != nil {
				return err
			}
			rows, err := cachedDays(ctx, p, q, "itemsByItem", from, to, st.ItemsByItemDaily)
			rep.ItemsTop = mergeItems(rows, 15)
			return err
		}},
		{"server", func() error {
			rows, err := cachedDays(ctx, p, q, "server", from, to, st.ServerEventsDaily)
			if err != nil {
				return err
			}
			rep.ServerDays, rep.ServerTop = mergeServer(rows, 20)
			return q.measure("serverContext", func() error { return st.ServerEventContext(ctx, rep.ServerTop) })
		}},
		{"players", func() (err error) {
			rep.Players, err = cachedDays(ctx, p, q, "players", from, to, st.PlayersDaily)
			return err
		}},
	}
	for _, s := range steps {
		if err := rep.TimingsMs.run(s.name, s.fn); err != nil {
			return nil, err
		}
	}
	rep.GeneratedAt = time.Now().UTC()
	return rep, nil
}

func mergeDeaths(rows []store.DeathDayRow) []DeathRow {
	type key struct{ region, cause string }
	type acc struct{ n, lv int64 }
	m := map[key]*acc{}
	for _, r := range rows {
		k := key{r.Region, r.Cause}
		if m[k] == nil {
			m[k] = &acc{}
		}
		m[k].n += r.Deaths
		m[k].lv += r.LevelSum
	}
	out := make([]DeathRow, 0, len(m))
	for k, a := range m {
		out = append(out, DeathRow{Region: k.region, Cause: k.cause, Deaths: a.n, AvgLevel: round1(float64(a.lv) / float64(max(a.n, 1)))})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Deaths != out[j].Deaths {
			return out[i].Deaths > out[j].Deaths
		}
		return out[i].Region+out[i].Cause < out[j].Region+out[j].Cause
	})
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

func mergeFPS(rows []store.FPSDayRow) []FPSRow {
	type key struct{ region, device string }
	type acc struct {
		n, lowN  int64
		fps, low float64
	}
	m := map[key]*acc{}
	for _, r := range rows {
		k := key{r.Region, r.Device}
		if m[k] == nil {
			m[k] = &acc{}
		}
		a := m[k]
		a.n += r.Samples
		a.fps += r.FPSSum
		a.lowN += r.LowN
		a.low += r.LowSum
	}
	out := make([]FPSRow, 0, len(m))
	for k, a := range m {
		out = append(out, FPSRow{Region: k.region, Device: k.device, Samples: a.n,
			AvgFPS: round1(a.fps / float64(max(a.n, 1))), AvgLow: round1(a.low / float64(max(a.lowN, 1)))})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Region+"|"+out[i].Device < out[j].Region+"|"+out[j].Device })
	return out
}

func mergeItems(rows []store.ItemFlowRow, limit int) []store.ItemFlowRow {
	m := map[string]*store.ItemFlowRow{}
	for _, r := range rows {
		if m[r.Item] == nil {
			m[r.Item] = &store.ItemFlowRow{Item: r.Item}
		}
		m[r.Item].In += r.In
		m[r.Item].Out += r.Out
	}
	out := make([]store.ItemFlowRow, 0, len(m))
	for _, v := range m {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].In != out[j].In {
			return out[i].In > out[j].In
		}
		return out[i].Item < out[j].Item
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// mergeServer soma os grupos de erro por dia e tipo (o gráfico) e no período (os mais
// frequentes, com a última vez de cada um).
func mergeServer(rows []store.ServerEventDay, limit int) ([]store.ServerEventDay, []store.ServerEventTop) {
	days := []store.ServerEventDay{}
	idx := map[[2]string]int{}
	type key struct{ kind, ref string }
	tops := map[key]*store.ServerEventTop{}
	for _, r := range rows {
		dk := [2]string{r.Day, r.Kind}
		if i, ok := idx[dk]; ok {
			days[i].Count += r.Count
		} else {
			idx[dk] = len(days)
			days = append(days, store.ServerEventDay{Day: r.Day, Kind: r.Kind, Count: r.Count})
		}
		k := key{r.Kind, r.Ref}
		t := tops[k]
		if t == nil {
			t = &store.ServerEventTop{Kind: r.Kind, Ref: r.Ref}
			tops[k] = t
		}
		t.Count += r.Count
		if r.Last.After(t.Last) {
			t.Last = r.Last
		}
	}
	out := make([]store.ServerEventTop, 0, len(tops))
	for _, t := range tops {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Last.After(out[j].Last)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return days, out
}

// handleMetrics: GET /api/admin/metrics?days=1|7|30|90.
func (p *panels) handleMetrics(w http.ResponseWriter, r *http.Request) {
	days := periodDays(r)
	v, _, err := p.cache.get(r.Context(), "metrics:"+strconv.Itoa(days), func(ctx context.Context) (any, error) {
		return p.MetricsReport(ctx, days)
	})
	if err != nil {
		if r.Context().Err() != nil {
			return // quem pediu desistiu
		}
		p.a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
