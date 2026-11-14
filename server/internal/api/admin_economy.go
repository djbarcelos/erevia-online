package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"erevia/server/internal/store"
)

// Economia e auditoria (#109): ouro e itens em circulação contra a telemetria (para caçar
// duplicação), histórico por personagem com alerta de picos e o gm_log com filtro.

// EconomyReport é o painel de economia de um período.
type EconomyReport struct {
	Days      int                 `json:"days"`
	Since     time.Time           `json:"since"`
	Supply    store.GoldSupply    `json:"supply"`
	GoldDaily []store.GoldNetDay  `json:"goldDaily"`
	Items     []store.ItemBalance `json:"items"`
	// Suspicious: quantos itens existem acima do que a telemetria explica.
	Suspicious  int            `json:"suspicious"`
	Spikes      []store.Spike  `json:"spikes"`
	TimingsMs   timings        `json:"timingsMs"`
	SlowestMs   timings        `json:"slowestMs"`
	Queries     map[string]int `json:"queries"`
	GeneratedAt time.Time      `json:"generatedAt"`
}

// EconomyReport roda as consultas do painel de economia. O histórico de antes de hoje
// (para o saldo de ouro e de itens) e os dias fechados (ouro por dia e picos) vêm de cache.
func (p *panels) EconomyReport(ctx context.Context, days int) (*EconomyReport, error) {
	now := time.Now()
	from, to := since(now, days), store.DayStart(now).Add(24*time.Hour)
	st := p.a.store
	q := newQueryStats()
	rep := &EconomyReport{Days: days, Since: from, TimingsMs: timings{}, SlowestMs: q.slowest, Queries: q.count}
	var before store.TelemetryTotals
	steps := []struct {
		name string
		fn   func() error
	}{
		{"supply", func() (err error) {
			if before, err = p.before(ctx, q); err != nil {
				return err
			}
			return q.measure("supply", func() (err error) { rep.Supply, err = st.GoldSupply(ctx, before); return err })
		}},
		{"goldDaily", func() error {
			rows, err := cachedDays(ctx, p, q, "gold", from, to, st.GoldDaily)
			rep.GoldDaily = store.GoldNetDays(rows, rep.Supply.Total)
			return err
		}},
		{"items", func() error {
			return q.measure("items", func() (err error) { rep.Items, err = st.ItemBalances(ctx, before); return err })
		}},
		{"spikes", func() error {
			rows, err := cachedDays(ctx, p, q, "spikes", from, to, st.SpikeDays)
			if err != nil {
				return err
			}
			rep.Spikes = store.FindSpikes(rows, 0, store.SpikeRule{})
			return q.measure("spikeNames", func() error { return st.NameSpikes(ctx, rep.Spikes) })
		}},
	}
	for _, s := range steps {
		if err := rep.TimingsMs.run(s.name, s.fn); err != nil {
			return nil, err
		}
	}
	for _, it := range rep.Items {
		if it.Surplus > 0 {
			rep.Suspicious++
		}
	}
	rep.GeneratedAt = time.Now().UTC()
	return rep, nil
}

// handleEconomy: GET /api/admin/economy?days=1|7|30|90.
func (p *panels) handleEconomy(w http.ResponseWriter, r *http.Request) {
	days := periodDays(r)
	v, _, err := p.cache.get(r.Context(), "economy:"+strconv.Itoa(days), func(ctx context.Context) (any, error) {
		return p.EconomyReport(ctx, days)
	})
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		p.a.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// handleEconomyCharacter: GET /api/admin/economy/character?name=&days= — o ouro e os itens
// do personagem por dia, de onde vieram e os dias fora da curva.
func (p *panels) handleEconomyCharacter(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" || utf8.RuneCountInString(name) > 40 {
		writeError(w, http.StatusBadRequest, "Diga o nome (ou o id) do personagem.")
		return
	}
	days := periodDays(r)
	from := since(time.Now(), days)
	c, err := p.a.store.FindCharacter(r.Context(), name)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "Personagem não encontrado.")
		return
	}
	if err != nil {
		p.a.internalError(w, err)
		return
	}
	t := timings{}
	var hist []store.CharacterDay
	var flows []store.CharacterFlow
	var spikes []store.Spike
	if err := t.run("history", func() (err error) {
		hist, flows, err = p.a.store.CharacterHistory(r.Context(), c.ID, from)
		return err
	}); err != nil {
		p.a.internalError(w, err)
		return
	}
	// Os picos comparam com os outros personagens: a mesma rodada do painel (em cache).
	v, _, err := p.cache.get(r.Context(), "economy:"+strconv.Itoa(days), func(ctx context.Context) (any, error) {
		return p.EconomyReport(ctx, days)
	})
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		p.a.internalError(w, err)
		return
	}
	for _, s := range v.(*EconomyReport).Spikes {
		if s.CharacterID == c.ID {
			spikes = append(spikes, s)
		}
	}
	if hist == nil {
		hist = []store.CharacterDay{}
	}
	if flows == nil {
		flows = []store.CharacterFlow{}
	}
	if spikes == nil {
		spikes = []store.Spike{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"days": days, "since": from, "character": c, "history": hist, "flows": flows, "spikes": spikes, "timingsMs": t,
	})
}

// handleEconomyGMLog: GET /api/admin/economy/gm-log?gm=&command=&target=&q=&days=&before=
// (days=0 ou ausente: todo o histórico).
func (p *panels) handleEconomyGMLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.GMLogFilter{
		GM: q.Get("gm"), Command: q.Get("command"), Target: q.Get("target"), Query: q.Get("q"),
		Before: queryInt64(r, "before"), Limit: int(queryInt64(r, "limit")),
	}
	for _, v := range []string{f.GM, f.Command, f.Target, f.Query} {
		if utf8.RuneCountInString(v) > 100 {
			writeError(w, http.StatusBadRequest, "Filtro grande demais.")
			return
		}
	}
	if q.Get("days") != "" {
		f.Since = since(time.Now(), periodDays(r))
	}
	entries, err := p.a.store.GMLog(r.Context(), f)
	if err != nil {
		p.a.internalError(w, err)
		return
	}
	if entries == nil {
		entries = []store.GMLogRow{}
	}
	out := map[string]any{"entries": entries}
	if f.Before == 0 {
		cmds, err := p.a.store.GMLogCommands(r.Context())
		if err != nil {
			p.a.internalError(w, err)
			return
		}
		out["commands"] = cmds
	}
	writeJSON(w, http.StatusOK, out)
}
