package main

// Teste de caos (tunnel#117). Cada bot sorteia, de uma semente, ações válidas e inválidas
// de todo o oneof ClientMessage (itens, armazém, troca, grupo, clã, teleporte, criação,
// correio...), com slots fora da faixa, quantidades 0 ou enormes, ids que não existem e
// combinações que nenhum teste unitário pensou (vender e encantar o mesmo espaço, cair no
// meio da troca, aceitar o convite de um clã que acabou). Também cai e volta em momentos
// sorteados.
//
// Reprodutível: o tipo de cada ação vem só da semente (6 sorteios por passo, sempre), e os
// parâmetros sorteados se aplicam ao estado que o bot vê (o espaço "k-ésimo item da bolsa").
// Com a mesma semente, a sequência de escolhas e a contagem de ações por tipo são as mesmas,
// mesmo que a rede mude a ordem em que as respostas chegam (a assinatura no relatório confere).
//
// Antes de jogar, com acesso ao banco (-db), o caos cria personagens novos e dá a cada um,
// também pela semente, nível, moedas, pergaminhos, materiais, equipamento e armazém: sem isso
// quase nada (clã, encantar, criar) passaria da primeira checagem. Depois tira o saldo
// inicial (moedas e itens no banco) e, no fim, com todos fora do jogo e os salvamentos
// gravados, confere: saldo final = inicial + entradas − saídas da telemetria (game_events).
//
// Sai com código ≠ 0 se algo quebrou: server_error ou invariante no banco, kick por erro no
// servidor, servidor fora do ar no fim, panic no log do servidor (-serverlog) ou economia que
// não fecha.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type chaosConfig struct {
	base      string
	bots      int
	duration  time.Duration
	ramp      time.Duration
	step      time.Duration
	seed      string
	prefix    string
	db        string
	serverLog string
	settle    time.Duration
}

// chaosRun é o estado compartilhado de uma rodada.
type chaosRun struct {
	cfg   chaosConfig
	seed  uint64
	names []string // nome de cada bot (índice = bot)
	tag   string   // pedaço curto do prefixo, para nomes de clã

	mu       sync.Mutex
	actions  map[string]int64 // ações por tipo (só da semente: reprodutível)
	sent     map[string]int64 // comandos enviados, com o detalhe que depende do estado
	kicks    map[string]int64 // Kicked recebidos, por motivo
	errs     map[string]int64 // erros do lado do bot (conexão, login...)
	trades   map[string]int64 // avisos do servidor sobre trocas (concluídas, canceladas e por quê)
	sessions int64
	panics   int64 // kicks "Erro no servidor" (o servidor recuperou um panic)
	sig      []uint64
	steps    []int
}

func (r *chaosRun) count(m map[string]int64, k string, n int64) {
	r.mu.Lock()
	m[k] += n
	r.mu.Unlock()
}

func runChaos(cfg chaosConfig) int {
	seed, err := parseSeed(cfg.seed)
	if err != nil {
		log.Printf("semente inválida: %v", err)
		return 2
	}
	if cfg.prefix == "" {
		// Personagens novos a cada rodada: a mesma semente começa sempre do mesmo estado.
		cfg.prefix = "Cx" + strconv.FormatInt(time.Now().Unix()%(36*36*36*36*36*36), 36)
	}
	if len(cfg.prefix) > 13 || !onlyAlnum(cfg.prefix) {
		log.Printf("prefixo inválido %q: até 13 letras ou números (o nome leva mais 3 dígitos)", cfg.prefix)
		return 2
	}
	if cfg.bots < 2 || cfg.bots > 999 {
		log.Printf("-bots precisa estar entre 2 e 999")
		return 2
	}
	run := &chaosRun{
		cfg: cfg, seed: seed, tag: shortTag(cfg.prefix),
		actions: map[string]int64{}, sent: map[string]int64{}, kicks: map[string]int64{}, errs: map[string]int64{},
		trades: map[string]int64{},
		sig:    make([]uint64, cfg.bots), steps: make([]int, cfg.bots),
	}
	for i := range cfg.bots {
		run.names = append(run.names, fmt.Sprintf("%s%03d", cfg.prefix, i))
	}
	fmt.Printf("caos: semente %d, %d bots (%s000…%s%03d), %v, um passo a cada %v\n",
		seed, cfg.bots, cfg.prefix, cfg.prefix, cfg.bots-1, cfg.duration, cfg.step)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var db *chaosDB
	if cfg.db != "" {
		if db, err = openChaosDB(ctx, cfg.db); err != nil {
			log.Printf("banco: %v", err)
			return 2
		}
		defer db.close()
	} else {
		fmt.Println("AVISO: sem -db (nem DATABASE_URL): sem preparo, sem conferência da economia e sem invariantes do banco")
	}

	// 1) Login e personagens (em paralelo, 8 por vez).
	bots := make([]*chaosBot, cfg.bots)
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	var loginErr error
	var loginMu sync.Mutex
	for i := range cfg.bots {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			b, err := newChaosBot(ctx, run, i)
			if err != nil {
				loginMu.Lock()
				loginErr = fmt.Errorf("%s: %w", run.names[i], err)
				loginMu.Unlock()
				return
			}
			bots[i] = b
		}(i)
	}
	wg.Wait()
	if loginErr != nil {
		log.Printf("login: %v", loginErr)
		return 2
	}

	// 2) Preparo (nível, moedas, itens, armazém) e saldo inicial.
	var ids, accounts []int64
	var before economy
	var eventMark int64
	if db != nil {
		ids = make([]int64, len(bots))
		for i, b := range bots {
			ids[i] = int64(b.charID)
		}
		if accounts, err = db.accountsOf(ctx, ids); err != nil {
			log.Printf("contas: %v", err)
			return 2
		}
		fresh := 0
		for _, b := range bots {
			if !b.created {
				continue
			}
			fresh++
			if err := db.prepare(ctx, b.charID, b.class, prepFor(seed, b.idx)); err != nil {
				log.Printf("preparo de %s: %v", b.name, err)
				return 2
			}
		}
		if fresh < len(bots) {
			fmt.Printf("AVISO: %d personagens já existiam (prefixo repetido): sem preparo para eles, e a rodada não é reprodutível\n", len(bots)-fresh)
		}
		if eventMark, err = db.lastEventID(ctx); err != nil {
			log.Printf("telemetria: %v", err)
			return 2
		}
		if before, err = db.economy(ctx, ids, accounts); err != nil {
			log.Printf("saldo inicial: %v", err)
			return 2
		}
		fmt.Printf("saldo inicial: %d moedas e %d itens (%d tipos)\n", before.gold, before.totalItems(), len(before.items))
	}

	// 3) O caos.
	start := time.Now()
	steps := int(cfg.duration / cfg.step)
	// Prazo duro: os passos são contados (reprodutível), o relógio só limita um servidor travado.
	hard, cancel := context.WithTimeout(ctx, cfg.duration+time.Duration(cfg.bots)*cfg.ramp+3*time.Minute)
	defer cancel()
	for i, b := range bots {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.play(hard, steps)
		}()
		if i < len(bots)-1 {
			select {
			case <-hard.Done():
			case <-time.After(cfg.ramp):
			}
		}
	}
	progress := time.NewTicker(30 * time.Second)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
wait:
	for {
		select {
		case <-done:
			break wait
		case <-progress.C:
			run.mu.Lock()
			total := int64(0)
			for _, v := range run.actions {
				total += v
			}
			fmt.Printf("  %v: %d ações, %d sessões, %d kicks, %d panics recuperados\n",
				time.Since(start).Round(time.Second), total, run.sessions, sumMap(run.kicks), run.panics)
			run.mu.Unlock()
		}
	}
	progress.Stop()
	elapsed := time.Since(start)

	// 4) Depois: servidor de pé? Salvamentos e telemetria no banco, conferência.
	rep := &chaosReport{run: run, elapsed: elapsed, steps: steps}
	rep.healthy = healthz(cfg.base)
	if db != nil {
		fmt.Printf("esperando %v para os salvamentos e a telemetria chegarem ao banco...\n", cfg.settle)
		rep.after, rep.flow, rep.econErr = db.settledEconomy(context.Background(), ids, accounts, eventMark, cfg.settle)
		rep.before = before
		rep.problems, rep.eventKinds, rep.dbErr = db.problems(context.Background(), eventMark)
		rep.pool, rep.poolErr = db.auctionPool(context.Background(), ids, eventMark)
	}
	if cfg.serverLog != "" {
		rep.logHits, rep.logErr = scanServerLog(cfg.serverLog)
	}
	return rep.print()
}

func parseSeed(s string) (uint64, error) {
	if s == "" {
		return uint64(time.Now().UnixNano()) % 1_000_000, nil
	}
	return strconv.ParseUint(strings.TrimSpace(s), 10, 64)
}

func onlyAlnum(s string) bool {
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return s != ""
}

// shortTag: 4 letras/números do prefixo para os nomes de clã (únicos no servidor).
func shortTag(prefix string) string {
	h := fnv.New32a()
	h.Write([]byte(prefix))
	return strconv.FormatUint(uint64(h.Sum32()%(36*36*36*36)), 36)
}

func sumMap(m map[string]int64) int64 {
	n := int64(0)
	for _, v := range m {
		n += v
	}
	return n
}

func healthz(base string) bool {
	c := &http.Client{Timeout: 5 * time.Second}
	for range 3 {
		res, err := c.Get(base + "/healthz")
		if err == nil {
			res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(time.Second)
	}
	return false
}

// chaosLogin entra pelo login de desenvolvimento e garante o personagem.
// created: o personagem foi criado agora (nunca jogou).
func chaosLogin(ctx context.Context, client *http.Client, base, name, class string) (id uint32, created bool, err error) {
	post := func(path string, body any, out any) error {
		b, _ := json.Marshal(body)
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode >= 300 {
			return fmt.Errorf("POST %s: %s", path, res.Status)
		}
		if out != nil {
			return json.NewDecoder(res.Body).Decode(out)
		}
		return nil
	}
	if err := post("/api/auth/dev", map[string]string{"username": strings.ToLower(name)}, nil); err != nil {
		return 0, false, fmt.Errorf("%w (o servidor precisa de DEV_LOGIN=true)", err)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/characters", nil)
	res, err := client.Do(req)
	if err != nil {
		return 0, false, err
	}
	var list struct {
		Characters []struct {
			ID   uint32 `json:"id"`
			Name string `json:"name"`
		} `json:"characters"`
	}
	err = json.NewDecoder(res.Body).Decode(&list)
	res.Body.Close()
	if err != nil {
		return 0, false, err
	}
	for _, c := range list.Characters {
		if strings.EqualFold(c.Name, name) {
			return c.ID, false, nil
		}
	}
	var out struct {
		ID uint32 `json:"id"`
	}
	if err := post("/api/characters", map[string]string{"name": name, "class": class}, &out); err != nil {
		return 0, false, err
	}
	return out.ID, true, nil
}

func newChaosBot(ctx context.Context, run *chaosRun, idx int) (*chaosBot, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 15 * time.Second}
	name := run.names[idx]
	class := classes[idx%len(classes)]
	var id uint32
	var created bool
	var err error
	for attempt := range 3 { // o servidor pode estar recebendo 50 logins de uma vez
		if id, created, err = chaosLogin(ctx, client, run.cfg.base, name, class); err == nil {
			break
		}
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	if err != nil {
		return nil, err
	}
	return &chaosBot{
		run: run, idx: idx, name: name, class: class, client: client, jar: jar,
		charID: id, created: created,
		rng: rand.New(rand.NewPCG(run.seed, uint64(idx)*0x9e3779b97f4a7c15+1)),
	}, nil
}

// ---------- Relatório ----------

type chaosReport struct {
	run      *chaosRun
	elapsed  time.Duration
	steps    int
	healthy  bool
	before   economy
	after    economy
	flow     economyFlow
	econErr  error
	problems []eventProblem
	// eventKinds: eventos da rodada por tipo (game_events), para contexto.
	eventKinds map[string]int64
	dbErr      error
	logHits    []string
	// pool: a custódia da casa de leilões (anúncios ativos e anexos) contra a telemetria.
	pool    auctionPool
	poolErr error
	logErr     error
}

func (r *chaosReport) print() int {
	run := r.run
	failed := []string{}
	fmt.Println()
	fmt.Println("==================== RELATÓRIO DO CAOS ====================")
	fmt.Printf("semente %d · %d bots · %d passos por bot · %v\n", run.seed, run.cfg.bots, r.steps, r.elapsed.Round(time.Second))

	// Assinatura das escolhas: a mesma semente dá a mesma (a rede não muda as escolhas).
	h := fnv.New64a()
	incomplete := 0
	for i, s := range run.sig {
		fmt.Fprintf(h, "%d:%x;", i, s)
		if run.steps[i] < r.steps {
			incomplete++
		}
	}
	fmt.Printf("assinatura das escolhas: %016x", h.Sum64())
	if incomplete > 0 {
		fmt.Printf(" (ATENÇÃO: %d bots não terminaram os passos no prazo; a assinatura não vale)", incomplete)
	}
	fmt.Println()

	fmt.Println()
	fmt.Println("-- Ações por tipo (sorteadas pela semente) --")
	printCounts(run.actions)
	fmt.Println()
	fmt.Println("-- Comandos enviados (com o detalhe do estado; ! = inválido de propósito) --")
	printCounts(run.sent)

	fmt.Println()
	fmt.Printf("-- Sessões, erros e kicks --\nsessões abertas: %d\n", run.sessions)
	if len(run.errs) == 0 {
		fmt.Println("erros do bot: nenhum")
	} else {
		fmt.Println("erros do bot (conexão, login, quedas que o bot não pediu):")
		printCounts(run.errs)
	}
	if len(run.kicks) == 0 {
		fmt.Println("kicks: nenhum")
	} else {
		fmt.Println("kicks (Kicked recebidos):")
		printCounts(run.kicks)
	}
	if len(run.trades) > 0 {
		fmt.Println("avisos do servidor sobre trocas:")
		for _, k := range sortedKeys(run.trades) {
			fmt.Printf("  %6d  %s\n", run.trades[k], k)
		}
	}
	if run.panics > 0 {
		failed = append(failed, fmt.Sprintf("%d kicks por erro no servidor (panic recuperado)", run.panics))
	}
	if !r.healthy {
		failed = append(failed, "o servidor não responde ao /healthz no fim (caiu? veja o log: invariante com DEBUG_INVARIANTS=1 derruba o processo)")
		fmt.Println("servidor no fim: FORA DO AR")
	} else {
		fmt.Println("servidor no fim: de pé (/healthz ok)")
	}

	fmt.Println()
	fmt.Println("-- Invariantes, panics e erros do banco (game_events desta rodada) --")
	switch {
	case r.dbErr != nil:
		fmt.Printf("não deu para consultar: %v\n", r.dbErr)
		failed = append(failed, "consulta de game_events falhou")
	case run.cfg.db == "":
		fmt.Println("(sem banco)")
	case len(r.problems) == 0:
		fmt.Println("nenhum invariant, server_error ou db_error")
	default:
		for _, p := range r.problems {
			fmt.Printf("%-12s %-20s ×%d  %s\n", p.kind, p.ref, p.count, p.detail)
		}
		inv, se, de := 0, 0, 0
		for _, p := range r.problems {
			switch p.kind {
			case "invariant":
				inv += int(p.count)
			case "server_error":
				se += int(p.count)
			case "db_error":
				de += int(p.count)
			}
		}
		if inv > 0 {
			failed = append(failed, fmt.Sprintf("%d invariantes quebradas", inv))
		}
		if se > 0 {
			failed = append(failed, fmt.Sprintf("%d server_error (panics)", se))
		}
		if de > 0 {
			failed = append(failed, fmt.Sprintf("%d db_error", de))
		}
	}
	if len(r.eventKinds) > 0 {
		var parts []string
		for _, k := range sortedKeys(r.eventKinds) {
			parts = append(parts, fmt.Sprintf("%s=%d", k, r.eventKinds[k]))
		}
		fmt.Println("eventos da rodada: " + strings.Join(parts, " "))
	}

	if run.cfg.serverLog != "" {
		fmt.Println()
		fmt.Println("-- Log do servidor --")
		switch {
		case r.logErr != nil:
			fmt.Printf("não deu para ler %s: %v\n", run.cfg.serverLog, r.logErr)
		case len(r.logHits) == 0:
			fmt.Println("nenhum panic nem invariante")
		default:
			for _, l := range r.logHits {
				fmt.Println("  " + l)
			}
			failed = append(failed, fmt.Sprintf("%d linhas de panic/invariante no log do servidor", len(r.logHits)))
		}
	}

	fmt.Println()
	fmt.Println("-- Conferência da economia (banco × telemetria) --")
	switch {
	case run.cfg.db == "":
		fmt.Println("(sem banco)")
	case r.econErr != nil:
		fmt.Printf("não deu para conferir: %v\n", r.econErr)
		failed = append(failed, "conferência da economia falhou")
	default:
		bad := r.printEconomy()
		if bad > 0 {
			failed = append(failed, fmt.Sprintf("economia não fecha em %d linhas", bad))
		}
	}

	fmt.Println()
	if len(failed) == 0 {
		fmt.Println("RESULTADO: OK, nada quebrou")
		return 0
	}
	fmt.Println("RESULTADO: QUEBROU")
	for _, f := range failed {
		fmt.Println("  - " + f)
	}
	return 1
}

// printEconomy imprime a conferência e devolve quantas linhas não fecham.
func (r *chaosReport) printEconomy() int {
	b, a, f := r.before, r.after, r.flow
	bad := 0
	wantGold := b.gold + f.gold
	fmt.Printf("moedas: inicial %d + telemetria %+d = esperado %d; no banco %d", b.gold, f.gold, wantGold, a.gold)
	if a.gold != wantGold {
		fmt.Printf("  <-- NÃO FECHA (diferença %+d)\n", a.gold-wantGold)
		bad++
	} else {
		fmt.Println("  ok")
	}
	if len(f.goldBy) > 0 {
		var parts []string
		for _, k := range sortedKeys(f.goldBy) {
			parts = append(parts, fmt.Sprintf("%s %+d", k, f.goldBy[k]))
		}
		fmt.Println("  telemetria de moedas por origem: " + strings.Join(parts, ", "))
	}
	ids := map[string]bool{}
	for k := range b.items {
		ids[k] = true
	}
	for k := range a.items {
		ids[k] = true
	}
	for k := range f.in {
		ids[k] = true
	}
	for k := range f.out {
		ids[k] = true
	}
	keys := make([]string, 0, len(ids))
	for k := range ids {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	ok := 0
	for _, k := range keys {
		want := b.items[k] + f.in[k] - f.out[k]
		if a.items[k] == want {
			ok++
			continue
		}
		bad++
		fmt.Printf("item %-22s inicial %6d + entrou %6d − saiu %6d = esperado %6d; no banco %6d  <-- diferença %+d (%s)\n",
			k, b.items[k], f.in[k], f.out[k], want, a.items[k], a.items[k]-want, f.howText(k))
	}
	fmt.Printf("itens: %d tipos conferidos, %d fecham, %d não fecham\n", len(keys), ok, bad-boolInt(a.gold != wantGold))
	bad += r.printAuctionPool()
	if r.after.unsettled {
		fmt.Println("  (o banco ainda mudava no fim da espera: aumente -settle se a diferença for de salvamento atrasado)")
	}
	for _, p := range r.problems {
		if bad > 0 && p.kind == "db_error" && (strings.HasPrefix(p.ref, "save_characters") || p.ref == "telemetry") {
			fmt.Printf("  (houve db_error %s: salvamento ou lote de telemetria perdido; a diferença pode ser só isso)\n", p.ref)
		}
	}
	return bad
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func printCounts(m map[string]int64) {
	keys := sortedKeys(m)
	width := 0
	for _, k := range keys {
		width = max(width, len(k))
	}
	line := ""
	for i, k := range keys {
		cell := fmt.Sprintf("%-*s %7d", width, k, m[k])
		if i%3 == 2 || i == len(keys)-1 {
			fmt.Println("  " + line + cell)
			line = ""
		} else {
			line += cell + "   "
		}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// scanServerLog procura panic e invariante quebrada no log do servidor.
func scanServerLog(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var hits []string
	lines := strings.Split(string(data), "\n")
	for i, l := range lines {
		if strings.Contains(l, "panic") || strings.Contains(l, "invariante") || strings.Contains(l, "invariant broken") ||
			strings.HasPrefix(l, "fatal error") {
			l = strings.TrimSpace(l)
			if len(l) > 400 {
				l = l[:400] + "…"
			}
			hits = append(hits, fmt.Sprintf("%d: %s", i+1, l))
			if len(hits) >= 30 {
				hits = append(hits, "…")
				break
			}
		}
	}
	return hits, nil
}
