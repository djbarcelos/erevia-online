package alerts

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"erevia/server/internal/store"
)

// Como cada aviso é detectado (uma verificação por minuto):
//
//   - erro novo: um hash de server_error (game_events.ref) ou de client_errors que não
//     está em alert_state como seen. Avisado, vira seen (para sempre). Na primeira vez dos
//     alertas, tudo o que já existia vira seen sem aviso (AlertBootstrap).
//   - pico: mais de SpikeN ocorrências de um erro em SpikeWindow. Servidor: soma de amount
//     dos server_error e db_error da janela (a zona grava a contagem acumulada). Cliente: a
//     contagem de client_errors é acumulada por linha, então o worker guarda o total de
//     cada hash a cada verificação e soma as diferenças da janela (spikeTracker).
//   - invariante: a primeira linha "invariant" de cada regra no dia (Brasília).
//   - tick lento: os slow_tick (ver checkSlowTick).
//   - servidor no ar: cada subida grava uma linha boot; depois de BootDelay o aviso
//     compara com a subida anterior (outra versão = deploy; mesma versão sem desligamento
//     normal = queda).
//   - /bug: relatos depois do último avisado da conta; os da mesma conta vão juntos.

// Seções do portal (/admin#seção) de cada aviso.
const (
	sectionHome    = "inicio"
	sectionReports = "relatos"
	sectionOps     = "operacao"
)

const (
	newErrorsPerCheck = 5  // erros novos com aviso próprio por verificação; o resto vai num resumo
	unseenLimit       = 50 // erros novos lidos por verificação (de cada lado)
	bugsPerMessage    = 4  // relatos da mesma conta numa mensagem (o resto: "mais N" e o portal)
	bugTextMax        = 800
	recentWindow      = 24 * time.Hour  // erros e relatos mais velhos que isso não são avisados
	pruneEvery        = time.Hour       // limpeza do alert_state
	pruneAfter        = 30 * 24 * time.Hour
	slowTickLimit     = 50.0            // ms: o p95 do tick não pode passar disso...
	slowTickFor       = 2 * time.Minute // ...por esse tempo
	slowTickShare     = 0.05            // p95 > 50 ms = mais de 5% dos ticks acima de 50 ms
	slowTickFresh     = 2 * time.Minute // o slow_tick mais novo tem de ser recente (ainda lento)
	slowTickLookback  = 10 * time.Minute
)

var brasilia = time.FixedZone("BRT", -3*3600)

// ---------- Servidor no ar ----------

func (w *Worker) checkBoot(ctx context.Context, now time.Time) error {
	if !w.registered {
		prev, found, err := w.st.LatestAlertMark(ctx, "boot", w.bootID)
		if err != nil {
			return err
		}
		if found {
			w.prevBoot = prev.Key
		}
		if err := w.st.SaveAlertMark(ctx, store.AlertMark{
			Kind: "boot", Key: w.bootID, SentAt: w.bootAt, Pos: now.Unix(),
			Info: map[string]any{"version": w.cfg.Version, "clean": false},
		}); err != nil {
			return err
		}
		w.registered = true
	} else if err := w.st.SetAlertPos(ctx, "boot", w.bootID, now.Unix()); err != nil { // último sinal
		return err
	}
	if w.bootDone || now.Sub(w.bootAt) < w.cfg.BootDelay {
		return nil
	}
	var prev *store.AlertMark
	if w.prevBoot != "" { // relido agora: o anterior pode ter desligado depois que este subiu
		m, found, err := w.st.AlertMark(ctx, "boot", w.prevBoot)
		if err != nil {
			return err
		}
		if found {
			prev = &m
		}
	}
	err := w.limited(ctx, "boot_alert", w.cfg.Version, now, func(held int) []Embed {
		return []Embed{w.bootEmbed(prev, held)}
	})
	if err == nil {
		w.bootDone = true
	}
	return err
}

// stopped anota que esta subida desligou normalmente (o próximo aviso de servidor no ar
// não fala em queda).
func (w *Worker) stopped() {
	if !w.registered {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	now := w.cfg.Now()
	err := w.st.SaveAlertMark(ctx, store.AlertMark{
		Kind: "boot", Key: w.bootID, SentAt: w.bootAt, Pos: now.Unix(),
		Info: map[string]any{"version": w.cfg.Version, "clean": true, "stopped": now.UTC().Format(time.RFC3339)},
	})
	if err != nil {
		w.log.Warn("alertas: anotar o desligamento", "err", err)
	}
}

func (w *Worker) bootEmbed(prev *store.AlertMark, held int) Embed {
	v := w.cfg.Version
	e := Embed{Title: "Servidor no ar", URL: w.link(sectionHome), Color: colorInfo}
	lines := []string{fmt.Sprintf("Versão %s no ar desde %s.", inline(short(v)), clock(w.bootAt))}
	fields := []Field{
		{Name: "Versão", Value: inline(v), Inline: true},
		{Name: "Subiu", Value: clock(w.bootAt), Inline: true},
	}
	if prev == nil {
		lines = append(lines, "Primeira subida registrada pelos alertas.")
	} else {
		pv, _ := prev.Info["version"].(string)
		clean, _ := prev.Info["clean"].(bool)
		last := time.Unix(prev.Pos, 0)
		fields = append(fields, Field{Name: "Anterior", Value: fmt.Sprintf("%s, no ar desde %s", inline(short(pv)), clock(prev.SentAt)), Inline: true})
		switch {
		case pv != v:
			e.Title, e.Color = "Deploy: servidor no ar", colorOK
			lines = append(lines, fmt.Sprintf("Deploy de %s para %s.", inline(short(pv)), inline(short(v))))
			if !clean {
				lines = append(lines, fmt.Sprintf("O servidor anterior não registrou o desligamento (último sinal às %s).", clock(last)))
			}
		case clean:
			e.Title = "Servidor reiniciado"
			lines = append(lines, "Mesma versão; o anterior desligou normalmente.")
		default:
			e.Title, e.Color = "Servidor caiu e voltou", colorCritical
			lines = append(lines, fmt.Sprintf("Mesma versão, e o anterior **não desligou normalmente**: último sinal às %s (%s antes desta subida). Provável queda (panic fora de um recover, falta de memória ou a máquina).",
				clock(last), ago(w.bootAt.Sub(last))))
		}
	}
	if held > 0 {
		lines = append(lines, fmt.Sprintf("Mais %d subida(s) desta versão nos últimos %s sem aviso (limite).", held, ago(w.cfg.Limit)))
	}
	e.Description = strings.Join(lines, "\n")
	e.Fields = fields
	return e
}

// ---------- Erro novo ----------

func (w *Worker) checkNewErrors(ctx context.Context, now time.Time) error {
	since := now.Add(-recentWindow)
	srv, err := w.st.UnseenServerErrors(ctx, since, unseenLimit)
	if err != nil {
		return err
	}
	cli, err := w.st.UnseenClientErrors(ctx, since, unseenLimit)
	if err != nil {
		return err
	}
	type item struct {
		key, line string
		embed     Embed
	}
	var items []item // servidor primeiro: é o mais grave
	for _, e := range srv {
		p, _ := e.Extra["panic"].(string)
		items = append(items, item{"server:" + e.Ref, "servidor " + inline(short12(e.Ref)) + ": " + plain(oneLine(p, 120)), w.serverErrorEmbed(e)})
	}
	for _, c := range cli {
		items = append(items, item{"client:" + c.Hash, "navegador " + inline(short12(c.Hash)) + ": " + plain(oneLine(c.Message, 120)), w.clientErrorEmbed(c)})
	}
	n := min(len(items), newErrorsPerCheck)
	for _, it := range items[:n] {
		if err := w.send(ctx, []Embed{it.embed}); err != nil {
			return err
		}
		if err := w.st.MarkAlertSeen(ctx, []string{it.key}, now); err != nil {
			return err
		}
	}
	rest := items[n:]
	if len(rest) == 0 {
		return nil
	}
	lines := make([]string, 0, len(rest))
	keys := make([]string, 0, len(rest))
	for i, it := range rest {
		keys = append(keys, it.key)
		if i < 15 {
			lines = append(lines, "• "+it.line)
		}
	}
	if len(rest) > 15 {
		lines = append(lines, fmt.Sprintf("… e mais %d.", len(rest)-15))
	}
	if err := w.send(ctx, []Embed{{
		Title: fmt.Sprintf("Mais %d erros novos", len(rest)), URL: w.link(sectionOps), Color: colorHigh,
		Description: strings.Join(lines, "\n"),
	}}); err != nil {
		return err
	}
	return w.st.MarkAlertSeen(ctx, keys, now)
}

func (w *Worker) serverErrorEmbed(e store.AlertEvent) Embed {
	ex := e.Extra
	panicText, _ := ex["panic"].(string)
	desc := "**" + plain(oneLine(panicText, 300)) + "**"
	if frames := strList(ex["frames"]); len(frames) > 0 {
		desc += "\n" + code(strings.Join(frames, "\n"))
	}
	fields := []Field{
		{Name: "Onde", Value: inline(str(ex["where"])), Inline: true},
		{Name: "Versão", Value: inline(short(str(ex["version"]))), Inline: true},
		{Name: "Ocorrências", Value: strconv.FormatInt(max(e.Amount, 1), 10), Inline: true},
	}
	if p := int64(num(ex["player"])); p > 0 {
		v := "personagem " + strconv.FormatInt(p, 10)
		if m := str(ex["msg"]); m != "" {
			v += ", mensagem " + inline(m)
		}
		if e.Region != "" && e.CharacterID > 0 {
			v += fmt.Sprintf(", %s (%.0f, %.0f)", e.Region, e.X, e.Z)
		}
		fields = append(fields, Field{Name: "Jogador", Value: v})
	}
	fields = append(fields, Field{Name: "Hash", Value: inline(e.Ref), Inline: true}, Field{Name: "Quando", Value: clock(e.At), Inline: true})
	return Embed{Title: "Erro novo no servidor", Description: desc, URL: w.link(sectionOps), Color: colorCritical, Fields: fields}
}

func (w *Worker) clientErrorEmbed(c store.AlertClientError) Embed {
	desc := "**" + plain(oneLine(c.Message, 300)) + "**"
	if s := firstLines(c.Stack, 8); s != "" {
		desc += "\n" + code(cut(s, 900))
	}
	fields := []Field{
		{Name: "Tipo", Value: inline(c.Kind), Inline: true},
		{Name: "Versão", Value: inline(short(c.Version)), Inline: true},
		{Name: "Ocorrências", Value: strconv.FormatInt(c.Count, 10), Inline: true},
	}
	if c.Region != "" {
		fields = append(fields, Field{Name: "Região", Value: plain(c.Region), Inline: true})
	}
	if c.UserAgent != "" {
		fields = append(fields, Field{Name: "Navegador", Value: plain(cut(c.UserAgent, 160))})
	}
	fields = append(fields, Field{Name: "Hash", Value: inline(c.Hash), Inline: true}, Field{Name: "Primeira vez", Value: clock(c.FirstSeen), Inline: true})
	return Embed{Title: "Erro novo no navegador", Description: desc, URL: w.link(sectionOps), Color: colorHigh, Fields: fields}
}

// ---------- Pico ----------

func (w *Worker) checkSpikes(ctx context.Context, now time.Time) error {
	srv, err := w.st.ErrorSpikes(ctx, now.Add(-w.cfg.SpikeWindow), w.cfg.SpikeN)
	if err != nil {
		return err
	}
	for _, e := range srv {
		key := "server:" + e.Ref
		if e.Kind == "db_error" {
			key = "db:" + e.Ref
		}
		if err := w.limited(ctx, "spike", key, now, func(held int) []Embed {
			return []Embed{w.serverSpikeEmbed(e, held)}
		}); err != nil {
			return err
		}
	}
	// Cliente: a janela de leitura cobre a janela do pico mais uma verificação (o que não
	// aparece nela não mudou desde a anterior; ver spikeTracker).
	totals, err := w.st.ClientErrorTotals(ctx, now.Add(-w.cfg.SpikeWindow-w.cfg.Every))
	if err != nil {
		return err
	}
	counts := w.clients.observe(now, w.cfg.SpikeWindow+w.cfg.Every, w.cfg.SpikeWindow, totals)
	for _, t := range totals {
		n := counts[t.Hash]
		if n <= w.cfg.SpikeN {
			continue
		}
		if err := w.limited(ctx, "spike", "client:"+t.Hash, now, func(held int) []Embed {
			return []Embed{w.clientSpikeEmbed(t, n, held)}
		}); err != nil {
			return err
		}
	}
	return nil
}

func (w *Worker) spikeHead(n int64, held int) string {
	s := fmt.Sprintf("**%d ocorrências** em %s (aviso a partir de mais de %d).", n, ago(w.cfg.SpikeWindow), w.cfg.SpikeN)
	if held > 0 {
		s += fmt.Sprintf("\nO pico continuou: %d verificação(ões) sem aviso desde o anterior (limite de %s).", held, ago(w.cfg.Limit))
	}
	return s
}

func (w *Worker) serverSpikeEmbed(e store.AlertEvent, held int) Embed {
	ex := e.Extra
	title, what := "Pico de erros no servidor", str(ex["panic"])
	fields := []Field{{Name: "Hash", Value: inline(e.Ref), Inline: true}}
	if e.Kind == "db_error" {
		title, what = "Pico de falhas no banco", str(ex["err"])
		fields = []Field{{Name: "Operação", Value: inline(e.Ref), Inline: true}}
	} else {
		fields = append(fields, Field{Name: "Onde", Value: inline(str(ex["where"])), Inline: true})
	}
	fields = append(fields,
		Field{Name: "Versão", Value: inline(short(str(ex["version"]))), Inline: true},
		Field{Name: "Última", Value: clock(e.At), Inline: true})
	return Embed{
		Title: title, URL: w.link(sectionOps), Color: colorCritical, Fields: fields,
		Description: w.spikeHead(e.Amount, held) + "\n" + code(oneLine(what, 300)),
	}
}

func (w *Worker) clientSpikeEmbed(t store.ClientErrorTotal, n int64, held int) Embed {
	return Embed{
		Title: "Pico de erros no navegador", URL: w.link(sectionOps), Color: colorCritical,
		Description: w.spikeHead(n, held) + "\n**" + plain(oneLine(t.Message, 300)) + "**",
		Fields: []Field{
			{Name: "Tipo", Value: inline(t.Kind), Inline: true},
			{Name: "Versão", Value: inline(short(t.Version)), Inline: true},
			{Name: "Total desde a primeira vez", Value: strconv.FormatInt(t.Total, 10), Inline: true},
			{Name: "Hash", Value: inline(t.Hash), Inline: true},
		},
	}
}

// spikeTracker conta as ocorrências de cada erro do cliente por verificação. A tabela só
// tem a contagem acumulada; a diferença entre duas verificações é o que aconteceu entre
// elas. Um hash que não aparece na leitura (sem atividade na janela de leitura) não mudou
// desde a última vez que apareceu, então a diferença dele, quando volta, é toda da
// verificação atual. Depois de um reinício (ou de uma verificação perdida), o total de um
// erro antigo vira só a base; um erro que apareceu dentro da janela conta inteiro.
type spikeTracker struct {
	lastAt time.Time
	total  map[string]int64     // hash → total na última vez que apareceu
	seen   map[string]time.Time // hash → quando apareceu pela última vez
	hits   map[string][]hit     // hash → ocorrências contadas em cada verificação da janela
}

type hit struct {
	at time.Time
	n  int64
}

// observe registra uma leitura (read = janela da leitura) e devolve as ocorrências de cada
// hash nas verificações dos últimos window.
func (t *spikeTracker) observe(now time.Time, read, window time.Duration, rows []store.ClientErrorTotal) map[string]int64 {
	// Sem leitura há mais que a janela de leitura (verificações perdidas): o que mudou
	// nesse meio pode não ter aparecido, então os totais antigos não servem de base. O que
	// já foi contado continua valendo.
	if t.total == nil || now.Sub(t.lastAt) >= read {
		t.total, t.seen = map[string]int64{}, map[string]time.Time{}
	}
	if t.hits == nil {
		t.hits = map[string][]hit{}
	}
	t.lastAt = now
	for _, r := range rows {
		prev, ok := t.total[r.Hash]
		var d int64
		switch {
		case ok:
			d = r.Total - prev
		case r.FirstSeen.After(now.Add(-window)):
			d = r.Total // apareceu dentro da janela: tudo é da janela
		}
		t.total[r.Hash], t.seen[r.Hash] = r.Total, now
		if d > 0 {
			t.hits[r.Hash] = append(t.hits[r.Hash], hit{now, d})
		}
	}
	out := map[string]int64{}
	for h, hs := range t.hits {
		keep := hs[:0]
		for _, x := range hs {
			if now.Sub(x.at) < window {
				keep = append(keep, x)
				out[h] += x.n
			}
		}
		if len(keep) == 0 {
			delete(t.hits, h)
		} else {
			t.hits[h] = keep
		}
	}
	for h, at := range t.seen {
		if now.Sub(at) > time.Hour {
			delete(t.seen, h)
			delete(t.total, h)
		}
	}
	return out
}

// ---------- Invariante ----------

func (w *Worker) checkInvariants(ctx context.Context, now time.Time) error {
	local := now.In(brasilia)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, brasilia)
	rows, err := w.st.InvariantsSince(ctx, day)
	if err != nil {
		return err
	}
	for _, e := range rows {
		key := e.Ref + "@" + day.Format("2006-01-02")
		_, found, err := w.st.AlertMark(ctx, "invariant", key)
		if err != nil {
			return err
		}
		if found {
			continue
		}
		if err := w.send(ctx, []Embed{w.invariantEmbed(e)}); err != nil {
			return err
		}
		if err := w.st.SaveAlertMark(ctx, store.AlertMark{Kind: "invariant", Key: key, SentAt: now}); err != nil {
			return err
		}
	}
	return nil
}

func (w *Worker) invariantEmbed(e store.AlertEvent) Embed {
	ex := e.Extra
	fixed, _ := ex["fixed"].(bool)
	desc := "Primeira quebra desta regra hoje. A zona **não tinha correção segura**: só anotou."
	if fixed {
		desc = "Primeira quebra desta regra hoje. A zona corrigiu o estado e anotou."
	}
	who := plain(str(ex["name"]))
	if who == "" {
		who = "—"
	}
	if e.CharacterID > 0 {
		who += fmt.Sprintf(" (personagem %d, nível %d)", e.CharacterID, e.Level)
	}
	return Embed{
		Title: "Invariante quebrada: " + e.Ref, Description: desc, URL: w.link(sectionOps), Color: colorWarn,
		Fields: []Field{
			{Name: "Personagem", Value: who},
			{Name: "Valor", Value: inline(cut(str(ex["value"]), 200)), Inline: true},
			{Name: "Posição", Value: fmt.Sprintf("%.1f, %.1f", e.X, e.Z), Inline: true},
			{Name: "Região", Value: e.Region, Inline: true},
			{Name: "Versão", Value: inline(short(str(ex["version"]))), Inline: true},
			{Name: "Registros hoje", Value: strconv.FormatInt(e.Count, 10), Inline: true},
			{Name: "Quando", Value: clock(e.At), Inline: true},
		},
	}
}

// ---------- Tick lento ----------

// checkSlowTick: o p95 do tick acima de 50 ms por 2 min, pelos slow_tick da zona. Um
// slow_tick sai no máximo 1×/min e traz em extra.slow_ticks quantos ticks passaram do
// orçamento (1/TickRate = 50 ms) desde o anterior. Entre dois slow_tick seguidos, a fração
// de ticks lentos é slow_ticks / (intervalo × ticks por segundo); p95 > 50 ms é essa fração
// acima de 5%. O aviso sai quando os intervalos mais recentes, todos acima de 5%, cobrem
// 2 min. Com a zona atrasada ela roda menos ticks por segundo que o nominal, então a conta
// subestima a fração (erra para o lado de não avisar).
func (w *Worker) checkSlowTick(ctx context.Context, now time.Time) error {
	evs, err := w.st.SlowTicks(ctx, now.Add(-slowTickLookback))
	if err != nil {
		return err
	}
	s, ok := findSlowSpell(evs, now)
	if !ok {
		return nil
	}
	return w.limited(ctx, "slow_tick", "zone", now, func(held int) []Embed {
		return []Embed{w.slowTickEmbed(s, held)}
	})
}

type slowSpell struct {
	since       time.Time
	covered     time.Duration
	slow, ticks float64
	worst       int64
	last        store.AlertEvent
}

func (s slowSpell) share() float64 {
	if s.ticks <= 0 {
		return 0
	}
	return min(1, s.slow/s.ticks)
}

func findSlowSpell(evs []store.AlertEvent, now time.Time) (slowSpell, bool) {
	if len(evs) < 2 || now.Sub(evs[len(evs)-1].At) > slowTickFresh {
		return slowSpell{}, false
	}
	s := slowSpell{last: evs[len(evs)-1]}
	for i := len(evs) - 1; i >= 1; i-- {
		iv := evs[i].At.Sub(evs[i-1].At)
		if iv <= 0 {
			break
		}
		budget := num(evs[i].Extra["budget_ms"])
		if budget <= 0 {
			budget = slowTickLimit
		}
		ticks := iv.Seconds() * 1000 / budget
		slow := num(evs[i].Extra["slow_ticks"])
		if slow/ticks <= slowTickShare {
			break
		}
		s.covered += iv
		s.slow += slow
		s.ticks += ticks
		s.worst = max(s.worst, evs[i].Amount)
		s.since = evs[i-1].At
	}
	return s, s.covered >= slowTickFor
}

func (w *Worker) slowTickEmbed(s slowSpell, held int) Embed {
	ex := s.last.Extra
	desc := fmt.Sprintf("O p95 do tick passou de %.0f ms por %s (desde %s): %.0f%% dos ticks acima de %.0f ms. Pior tick registrado: **%d ms**; sistema mais lento: %s.",
		slowTickLimit, ago(s.covered), clock(s.since), s.share()*100, slowTickLimit, s.worst, inline(s.last.Ref))
	if held > 0 {
		desc += fmt.Sprintf("\nContinua lento: %d verificação(ões) sem aviso desde o anterior (limite de %s).", held, ago(w.cfg.Limit))
	}
	var systems []string
	if m, ok := ex["systems"].(map[string]any); ok {
		names := slices.Collect(maps.Keys(m))
		slices.SortFunc(names, func(a, b string) int { return cmp.Compare(num(m[b]), num(m[a])) })
		for _, k := range names {
			systems = append(systems, fmt.Sprintf("%s %.1f ms", k, num(m[k])))
		}
	}
	return Embed{
		Title: "Tick lento na zona", Description: desc, URL: w.link(sectionOps), Color: colorHigh,
		Fields: []Field{
			{Name: "Sistemas (último tick lento)", Value: inline(strings.Join(systems, ", "))},
			{Name: "Jogadores", Value: strconv.Itoa(int(num(ex["players"]))), Inline: true},
			{Name: "Monstros", Value: strconv.Itoa(int(num(ex["monsters"]))), Inline: true},
			{Name: "Versão", Value: inline(short(str(ex["version"]))), Inline: true},
		},
	}
}

// ---------- /bug ----------

func (w *Worker) checkBugs(ctx context.Context, now time.Time) error {
	bugs, err := w.st.PendingBugReports(ctx, now.Add(-recentWindow), 200)
	if err != nil {
		return err
	}
	var order []int64
	byAcc := map[int64][]store.AlertBug{}
	for _, b := range bugs {
		if byAcc[b.AccountID] == nil {
			order = append(order, b.AccountID)
		}
		byAcc[b.AccountID] = append(byAcc[b.AccountID], b)
	}
	for _, acc := range order {
		key := strconv.FormatInt(acc, 10)
		m, ok, err := w.gate(ctx, "bug", key, now)
		if err != nil {
			return err
		}
		if !ok { // os relatos esperam o limite e saem juntos no próximo aviso da conta
			continue
		}
		list := byAcc[acc]
		embeds := make([]Embed, 0, bugsPerMessage)
		for _, b := range list[:min(len(list), bugsPerMessage)] {
			embeds = append(embeds, w.bugEmbed(b))
		}
		if extra := len(list) - len(embeds); extra > 0 {
			last := &embeds[len(embeds)-1]
			last.Description += fmt.Sprintf("\n\n*Mais %d relato(s) desta conta: veja no portal.*", extra)
		}
		if err := w.send(ctx, embeds); err != nil {
			return err
		}
		if err := w.st.SaveAlertMark(ctx, store.AlertMark{Kind: "bug", Key: key, SentAt: now, Pos: list[len(list)-1].ID, Info: m.Info}); err != nil {
			return err
		}
	}
	return nil
}

func (w *Worker) bugEmbed(b store.AlertBug) Embed {
	fields := []Field{
		{Name: "Personagem", Value: fmt.Sprintf("%s (nível %d)", plain(b.Name), b.Level), Inline: true},
		{Name: "Posição", Value: fmt.Sprintf("%.1f, %.1f", b.X, b.Z), Inline: true},
		{Name: "Região", Value: b.Region, Inline: true},
		{Name: "Versão", Value: inline(short(b.Version)), Inline: true},
		{Name: "Quando", Value: clock(b.At), Inline: true},
	}
	if b.UserAgent != "" {
		fields = append(fields, Field{Name: "Navegador", Value: plain(cut(b.UserAgent, 160))})
	}
	return Embed{
		Title: fmt.Sprintf("Relato de bug #%d", b.ID), URL: w.link(sectionReports), Color: colorInfo,
		Description: plain(cut(b.Text, bugTextMax)), Fields: fields,
	}
}

// ---------- Limpeza ----------

func (w *Worker) prune(ctx context.Context, now time.Time) error {
	if !w.lastPrune.IsZero() && now.Sub(w.lastPrune) < pruneEvery {
		return nil
	}
	if err := w.st.PruneAlertMarks(ctx, now.Add(-pruneAfter)); err != nil {
		return err
	}
	w.lastPrune = now
	return nil
}

// ---------- Formatação ----------

// clock: data e hora de Brasília ("13/11 14:05").
func clock(t time.Time) string { return t.In(brasilia).Format("02/01 15:04") }

// ago: duração curta em português ("5 min", "2 h", "40 s").
func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d s", int(d.Seconds()))
	case d < 2*time.Hour:
		return fmt.Sprintf("%d min", int(d.Round(time.Minute).Minutes()))
	default:
		return fmt.Sprintf("%d h", int(d.Round(time.Hour).Hours()))
	}
}

func short12(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func firstLines(s string, n int) string {
	var out []string
	for line := range strings.SplitSeq(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
			if len(out) == n {
				break
			}
		}
	}
	return strings.Join(out, "\n")
}

func str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	}
	return fmt.Sprint(v)
}

func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int64:
		return float64(x)
	case int:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}

func strList(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, x := range list {
		if s := str(x); s != "" {
			out = append(out, s)
		}
	}
	return out
}
