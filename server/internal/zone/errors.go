package zone

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"maps"
	"path"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"erevia/server/internal/pb"
)

// Erros do servidor: um panic num handler (uma mensagem estranha de um jogador) ou num
// sistema do tick não pode derrubar a goroutine da zona, e com ela todo mundo.
//
//   - Mensagem de cliente (handleSafe): recover, registra (stack, jogador, tipo da mensagem,
//     versão) e derruba só aquele jogador com um Kicked. A zona segue.
//   - Sistema do tick (runSystem): recover, registra e segue. O mesmo sistema com panic
//     panicDisableAfter vezes seguidas é desligado (evita um loop de panic a 20 Hz).
//     Dentro dos sistemas, cada jogador (guardPlayer) e cada monstro (guardMonster) tem o
//     seu recover: o jogador cai como numa mensagem; o monstro é parado sozinho.
//   - Goroutines fora da zona (workers, conexões, HTTP, persistência) usam ReportPanic.
//
// Tudo vai para a telemetria (game_events), agrupado:
//
//	server_error  ref = hash da stack (3 primeiros quadros fora do runtime), amount = ocorrências
//	              desde a gravação anterior; extra = where, player, msg, panic, frames, stack,
//	              version, total, disabled
//	db_error      ref = operação (save_characters, clan_op, boss_save…), amount = falhas;
//	              extra = err, version, total
//	slow_tick     ref = sistema mais lento, amount = ms do tick; extra = systems (ms por
//	              sistema, os 5 piores), slow_ticks (ticks lentos desde a última gravação),
//	              budget_ms, players, monsters, version
//
// No máximo 1 gravação por grupo (hash da stack ou operação) a cada errorEvery; a contagem
// acumula e sai na gravação seguinte (ou pelo worker da telemetria, depois do minuto).
//
// Depois de um panic, o personagem NÃO é salvo com o estado atual (pode estar pela metade:
// item tirado de um lado e ainda não posto no outro). Vale o último estado já entregue à
// persistência (lastSaved): ele vira o "recent" do personagem, então relogar logo usa
// exatamente o que o banco vai ter, sem ler um banco atrasado em relação à fila.

const (
	errorEvery        = time.Minute // gravação por grupo de erro
	slowTickEvery     = time.Minute // gravação de slow_tick
	panicDisableAfter = 10          // panics seguidos que desligam um sistema (ou param um monstro)
	maxErrorGroups    = 500         // grupos lembrados (o resto só vai para o log)
	stackKeep         = 4000        // bytes da stack guardados no evento
	kickedServerError = "Erro no servidor. Entre de novo."
)

// system é um sistema do tick, com recover e tempo medidos por runSystem.
type system uint8

const (
	sysDaytime system = iota
	sysAuras
	sysPlayers
	sysMonsters
	sysTelegraphs
	sysGround
	sysFlags
	sysInvites
	sysParties
	sysLeaveTown
	sysTrades
	sysPendingSaves
	sysInvariants
	sysSaveDue
	sysSnapshot
	sysAfterSnapshot
	sysSelfStats
	sysSaveAll
	sysInvasion
	sysXP2
	sysPaths
	sysXPTally
	sysAdminOps
	sysVitality
	numSystems
)

var systemNames = [numSystems]string{
	"daytime", "auras", "players", "monsters", "telegraphs", "ground", "flags", "invites",
	"parties", "leave_town", "trades", "pending_saves", "invariants", "save_due", "snapshot", "after_snapshot",
	"self_stats", "save_all", "invasion", "xp2", "paths", "xp_tally", "admin_ops",
	"vitality",
}

func (s system) String() string { return systemNames[s] }

type sysState struct {
	streak int  // panics seguidos
	off    bool // desligado depois de panicDisableAfter panics seguidos
}

// zoneErrors é o estado de erros da zona. Tudo aqui é da goroutine da zona, menos rep.
type zoneErrors struct {
	sys  [numSystems]sysState
	took [numSystems]time.Duration // tempo de cada sistema no tick atual

	monStreak map[uint32]int  // monstro → panics seguidos
	parked    map[uint32]bool // monstros parados por panic repetido

	broken      map[uint32]bool       // jogadores derrubados por panic (removePlayer não salva)
	lastSaved   map[uint32]SavedState // último estado entregue à persistência, por personagem
	lastStorage map[int64]StorageSave // último armazém entregue à persistência, por conta

	lastSlow  time.Time // última gravação de slow_tick
	slowTicks int       // ticks lentos desde a última gravação

	rep errorReporter
}

// ---------- Mensagens de cliente ----------

// handleSafe trata um evento com recover. Run usa este; os testes ainda chamam handle direto.
func (z *Zone) handleSafe(e event) {
	defer z.recoverEvent(e)
	z.handle(e)
}

func (z *Zone) recoverEvent(e event) {
	r := recover()
	if r == nil {
		return
	}
	rethrowInvariant(r)
	stack := debug.Stack()
	id, msg, peer := eventInfo(e)
	z.notePanic("msg", z.players[id], id, msg, r, stack, false)
	if id != 0 {
		z.kickAfterPanic(id, peer)
	}
}

// eventInfo: o jogador, o tipo da mensagem (o case do oneof ClientMessage) e, no Join, a
// conexão (o jogador pode nem ter entrado em z.players).
func eventInfo(e event) (id uint32, msg string, peer Peer) {
	switch e := e.(type) {
	case joinEvent:
		return e.info.ID, "Join", e.info.Peer
	case leaveEvent:
		return e.id, "Leave", nil
	case moveEvent:
		return e.id, "MoveTo", nil
	case stopEvent:
		return e.id, "MoveTo(stop)", nil
	case chatEvent:
		return e.id, "ChatSend", nil
	case attackEvent:
		return e.id, "Attack", nil
	case skillEvent:
		return e.id, "UseSkill", nil
	case respawnEvent:
		return e.id, "Respawn", nil
	case itemEvent:
		return e.id, pbName(e.msg), nil
	case progressEvent:
		return e.id, pbName(e.msg), nil
	case socialEvent:
		return e.id, pbName(e.msg), nil
	case tradeEvent:
		return e.id, "TradeCmd", nil
	case clientStatsEvent:
		return e.id, "ClientStats", nil
	}
	// Respostas dos workers (clã, moderação, amigos, ranking…): sem jogador culpado.
	return 0, strings.TrimPrefix(fmt.Sprintf("%T", e), "zone."), nil
}

func pbName(m any) string { return strings.TrimPrefix(fmt.Sprintf("%T", m), "*pb.") }

// kickAfterPanic derruba só o jogador da mensagem: Kicked, fecha a conexão e tira da zona
// sem salvar o estado atual (ver dropAfterPanic).
func (z *Zone) kickAfterPanic(id uint32, peer Peer) {
	p := z.players[id]
	if peer != nil && (p == nil || p.pl.peer != peer) {
		closePeer(peer) // Join que não chegou a entrar (ou relogin no meio da troca de sessão)
	}
	if p == nil {
		return
	}
	z.markBroken(id)
	closePeer(p.pl.peer)
	// Gancho (tunnel#112): quando checkInvariants existir, rodar aqui sobre quem estava
	// junto (parceiro de troca, grupo) e registrar o que estiver quebrado. O jogador do
	// panic já volta ao último estado salvo.
	z.safeRemove(id)
}

// closePeer manda o Kicked e fecha a conexão. Está dentro de um recover: se a própria
// conexão for a causa do panic, não pode dar outro aqui (fora de qualquer recover).
func closePeer(peer Peer) {
	try := func(fn func()) {
		defer func() { recover() }()
		fn()
	}
	if data, err := proto.Marshal(&pb.ServerMessage{Msg: &pb.ServerMessage_Kicked{Kicked: &pb.Kicked{Reason: kickedServerError}}}); err == nil {
		try(func() { peer.Send(data) })
	}
	try(func() { peer.Close("server error") })
}

func (z *Zone) markBroken(id uint32) {
	if z.errs.broken == nil {
		z.errs.broken = map[uint32]bool{}
	}
	z.errs.broken[id] = true
}

// safeRemove tira o jogador; se a saída também der panic (estado ruim), tira à força.
func (z *Zone) safeRemove(id uint32) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		z.notePanic("remove", z.players[id], id, "", r, debug.Stack(), false)
		if p := z.players[id]; p != nil {
			delete(z.players, id)
			if z.byName[strings.ToLower(p.name)] == id {
				delete(z.byName, strings.ToLower(p.name))
			}
			z.dropAfterPanic(p)
			p.gone = true
			z.aoiDirty = true
		}
	}()
	z.removePlayer(id)
}

// dropAfterPanic é chamado por removePlayer: se o jogador caiu por panic, não grava o
// estado atual e devolve true. O "recent" passa a ser o último estado entregue à
// persistência (ou nenhum: aí o banco tem o estado da entrada, sem gravação na fila).
//
// Por que não salvar o estado atual: o panic pode ter parado um handler no meio (moedas
// tiradas e item ainda não entregue, item no armazém e ainda na bolsa), e gravar isso
// tornaria o erro permanente. Por que não só "não salvar": sem o recent, relogar logo
// leria o banco antes da última gravação da fila chegar, e a gravação atrasada perderia
// para a seguinte (save_seq). Com o último estado salvo, memória e banco convergem.
// Custo: o jogador perde o que fez desde o último salvamento (no máximo SaveEvery).
func (z *Zone) dropAfterPanic(p *entity) bool {
	if !z.errs.broken[p.id] {
		return false
	}
	delete(z.errs.broken, p.id)
	if st, ok := z.errs.lastSaved[p.id]; ok {
		z.recent[p.id] = recentState{state: st, at: z.cfg.Clock()}
	} else {
		delete(z.recent, p.id)
	}
	z.rollbackStorage(p)
	return true
}

// rollbackStorage volta o armazém da conta ao último salvo se ele mudou depois disso e não
// há outro personagem da conta online (o armazém é de todos eles).
func (z *Zone) rollbackStorage(p *entity) {
	acc := p.pl.accountID
	st := z.storages[acc]
	if acc == 0 || st == nil || !st.dirty {
		return
	}
	for _, o := range z.players {
		if o != p && o.pl.accountID == acc {
			return
		}
	}
	last, ok := z.errs.lastStorage[acc]
	if !ok { // nada gravado nesta execução: o banco tem o armazém da entrada
		delete(z.storages, acc)
		return
	}
	ns := newStorage(acc, &StorageData{Gold: last.Gold, Items: last.Items, Seq: last.Seq})
	ns.touched = st.touched
	z.storages[acc] = ns
}

// noteSaved lembra o último estado entregue à persistência (chamado por save).
func (z *Zone) noteSaved(batch []SavedState) {
	if len(batch) == 0 {
		return
	}
	if z.errs.lastSaved == nil {
		z.errs.lastSaved = map[uint32]SavedState{}
		z.errs.lastStorage = map[int64]StorageSave{}
	}
	for _, s := range batch {
		z.errs.lastSaved[s.ID] = s
		if s.Storage != nil {
			z.errs.lastStorage[s.Storage.AccountID] = *s.Storage
		}
	}
	// Esquece quem não está mais online (quem sai normalmente fica no recent).
	if len(z.errs.lastSaved) > 2*len(z.players)+128 {
		for id := range z.errs.lastSaved {
			if z.players[id] == nil {
				delete(z.errs.lastSaved, id)
			}
		}
		for acc := range z.errs.lastStorage {
			if z.storages[acc] == nil {
				delete(z.errs.lastStorage, acc)
			}
		}
	}
}

// ---------- Sistemas do tick ----------

// runSystem roda um sistema do tick com recover e mede o tempo dele (para o slow_tick).
// fn não escapa: o compilador deixa o closure na pilha.
func (z *Zone) runSystem(s system, fn func()) {
	if z.errs.sys[s].off {
		return
	}
	defer z.endSystem(s, time.Now())
	fn()
}

func (z *Zone) endSystem(s system, start time.Time) {
	z.errs.took[s] += time.Since(start)
	st := &z.errs.sys[s]
	r := recover()
	if r == nil {
		st.streak = 0
		return
	}
	rethrowInvariant(r)
	st.streak++
	st.off = st.streak >= panicDisableAfter
	z.notePanic("tick:"+s.String(), nil, 0, "", r, debug.Stack(), st.off)
	if st.off {
		z.log.Error("system disabled after repeated panics", "system", s.String(), "panics", st.streak, "version", z.version())
	}
}

// guardPlayer roda a parte do tick de um jogador; um panic derruba só ele.
func (z *Zone) guardPlayer(p *entity, fn func()) {
	defer z.recoverPlayer(p)
	fn()
}

func (z *Zone) recoverPlayer(p *entity) {
	r := recover()
	if r == nil {
		return
	}
	rethrowInvariant(r)
	z.notePanic("tick:player", p, p.id, "", r, debug.Stack(), false)
	if z.players[p.id] == p {
		z.kickAfterPanic(p.id, nil) // apagar do mapa durante o range é permitido
	}
}

// guardMonster roda a IA e o movimento de um monstro; panics seguidos param só ele.
func (z *Zone) guardMonster(m *entity, fn func()) {
	if len(z.errs.parked) > 0 && z.errs.parked[m.id] {
		return
	}
	defer z.recoverMonster(m)
	fn()
}

func (z *Zone) recoverMonster(m *entity) {
	r := recover()
	if r == nil {
		if len(z.errs.monStreak) > 0 {
			delete(z.errs.monStreak, m.id)
		}
		return
	}
	rethrowInvariant(r)
	if z.errs.monStreak == nil {
		z.errs.monStreak = map[uint32]int{}
		z.errs.parked = map[uint32]bool{}
	}
	z.errs.monStreak[m.id]++
	off := z.errs.monStreak[m.id] >= panicDisableAfter
	where := "tick:monster"
	if m.mon != nil && m.mon.def != nil {
		where += ":" + m.mon.def.ID
	}
	z.notePanic(where, nil, 0, "", r, debug.Stack(), off)
	if off {
		z.errs.parked[m.id] = true
		delete(z.errs.monStreak, m.id)
		z.log.Error("monster stopped after repeated panics", "monster", m.id, "where", where)
	}
}

// ---------- Tick lento ----------

// checkSlowTick (depois de cada tick, em Run): se passou do orçamento (1/TickRate), conta e
// grava um slow_tick com os sistemas mais lentos, no máximo 1 por slowTickEvery.
func (z *Zone) checkSlowTick(took time.Duration) {
	times := z.errs.took
	z.errs.took = [numSystems]time.Duration{}
	budget := time.Second / time.Duration(max(1, z.cfg.TickRate))
	if took <= budget {
		return
	}
	z.stats.slow++
	z.errs.slowTicks++
	now := z.cfg.Clock()
	if !z.errs.lastSlow.IsZero() && now.Sub(z.errs.lastSlow) < slowTickEvery {
		return
	}
	z.errs.lastSlow = now
	order := make([]system, 0, numSystems)
	for s := range numSystems {
		if times[s] > 0 {
			order = append(order, s)
		}
	}
	slices.SortFunc(order, func(a, b system) int { return cmp.Compare(times[b], times[a]) })
	systems := map[string]any{}
	var parts []string
	for _, s := range order[:min(5, len(order))] {
		systems[s.String()] = msOf(times[s])
		parts = append(parts, s.String()+"="+strconv.FormatFloat(msOf(times[s]), 'f', 1, 64))
	}
	ref := ""
	if len(order) > 0 {
		ref = order[0].String()
	}
	z.log.Warn("slow tick", "ms", msOf(took), "budget_ms", msOf(budget), "systems", strings.Join(parts, " "),
		"slow_ticks", z.errs.slowTicks, "players", len(z.players), "monsters", len(z.monsters))
	z.track(nil, "slow_tick", ref, took.Milliseconds(), map[string]any{
		"systems": systems, "slow_ticks": z.errs.slowTicks, "budget_ms": msOf(budget),
		"players": len(z.players), "monsters": len(z.monsters), "version": z.version(),
	})
	z.errs.slowTicks = 0
}

func msOf(d time.Duration) float64 { return float64(d.Microseconds()/100) / 10 }

// ---------- Registro (vale para qualquer goroutine) ----------

// ReportPanic registra um panic recuperado fora da goroutine da zona (conexões, workers,
// HTTP, persistência). Thread-safe.
func (z *Zone) ReportPanic(where string, playerID uint32, msg string, r any, stack []byte) {
	z.notePanic(where, nil, playerID, msg, r, stack, false)
}

// ReportDBError conta uma falha de gravação no banco (db_error). Thread-safe.
func (z *Zone) ReportDBError(op string, err error) {
	if err == nil {
		return
	}
	z.reportError("db:"+op, GameEvent{
		At: z.cfg.Clock(), Kind: "db_error", Ref: op,
		Extra: map[string]any{"err": cleanText(err.Error(), 300), "version": z.version()},
	})
}

// reportSocialDBError: amigos e correio. Recusas de regra (lista cheia, limite de cartas,
// personagem inexistente) não são erro do banco.
func (z *Zone) reportSocialDBError(what string, err error) {
	if err == nil || errors.Is(err, ErrNoSuchCharacter) || errors.Is(err, ErrSelf) ||
		errors.Is(err, ErrFriendListFull) || errors.Is(err, ErrMailboxFull) || errors.Is(err, ErrMailRate) {
		return
	}
	z.ReportDBError("social:"+strings.ReplaceAll(what, " ", "_"), err)
}

// notePanic monta o server_error e registra no log. p só é lido na goroutine da zona
// (fora dela vem nil).
func (z *Zone) notePanic(where string, p *entity, id uint32, msg string, r any, stack []byte, disabled bool) {
	key, frames := stackKey(stack)
	ev := GameEvent{At: z.cfg.Clock(), Kind: "server_error", Ref: key, CharacterID: int64(id)}
	if p != nil && p.pl != nil {
		ev.CharacterID, ev.AccountID, ev.Level, ev.X, ev.Z = int64(p.id), p.pl.accountID, p.level, p.pos.X, p.pos.Z
	}
	panicText := cleanText(fmt.Sprint(r), 300)
	ev.Extra = map[string]any{
		"where": where, "player": id, "msg": msg, "panic": panicText, "frames": frames,
		"stack": cleanText(string(stack), stackKeep), "version": z.version(), "disabled": disabled,
	}
	group := "panic:" + key
	if disabled { // o desligamento sempre aparece, mesmo dentro do minuto do mesmo erro
		group += ":off"
	}
	first := z.reportError(group, ev)
	args := []any{"where", where, "player", id, "msg", msg, "panic", panicText, "hash", key, "version", z.version()}
	if first {
		args = append(args, "stack", string(stack))
	} else {
		args = append(args, "repeat", true)
	}
	z.log.Error("panic recovered", args...)
}

// reportError passa o evento pelo agrupamento e, se for a vez dele, põe na fila da
// telemetria. Devolve true quando gravou (primeira vez no minuto).
func (z *Zone) reportError(key string, ev GameEvent) bool {
	out, ok := z.errs.rep.note(key, ev, ev.At)
	if ok {
		z.emitError(out)
	}
	return ok
}

// emitError põe na fila da telemetria sem bloquear (fila cheia descarta). Thread-safe.
func (z *Zone) emitError(ev GameEvent) {
	if z.eventSink == nil {
		return
	}
	select {
	case z.telemetry <- ev:
	default:
	}
}

// dueErrors: contagens acumuladas cujo minuto já passou (o worker da telemetria grava).
func (z *Zone) dueErrors() []GameEvent { return z.errs.rep.due(z.cfg.Clock()) }

func (z *Zone) version() string {
	if z.cfg.Version == "" {
		return "dev"
	}
	return z.cfg.Version
}

// errorReporter agrupa erros por chave: no máximo 1 gravação por errorEvery, contando o
// resto. Usado por várias goroutines (por isso o mutex).
type errorReporter struct {
	mu     sync.Mutex
	groups map[string]*errGroup
}

type errGroup struct {
	last    time.Time // última gravação
	pending int64     // ocorrências ainda não gravadas
	total   int64     // ocorrências desde que o servidor subiu
	sample  GameEvent // a última ocorrência não gravada
}

// note conta uma ocorrência e devolve o evento a gravar agora (ok=false: só contou).
func (r *errorReporter) note(key string, ev GameEvent, now time.Time) (GameEvent, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.groups == nil {
		r.groups = map[string]*errGroup{}
	}
	g := r.groups[key]
	if g == nil {
		if len(r.groups) >= maxErrorGroups {
			return ev, false
		}
		g = &errGroup{}
		r.groups[key] = g
	}
	g.total++
	g.pending++
	if !g.last.IsZero() && now.Sub(g.last) < errorEvery {
		g.sample = ev
		return ev, false
	}
	return g.take(ev, now), true
}

// take fecha a contagem: o evento leva as ocorrências pendentes e o total.
func (g *errGroup) take(ev GameEvent, now time.Time) GameEvent {
	ev.Amount = g.pending
	ev.Extra = maps.Clone(ev.Extra)
	if ev.Extra == nil {
		ev.Extra = map[string]any{}
	}
	ev.Extra["total"] = g.total
	g.pending = 0
	g.last = now
	g.sample = GameEvent{}
	return ev
}

// due devolve as contagens pendentes cujo minuto passou (a última ocorrência de cada grupo).
func (r *errorReporter) due(now time.Time) []GameEvent { return r.collect(now, false) }

// drain devolve todas as contagens pendentes (desligamento).
func (r *errorReporter) drain(now time.Time) []GameEvent { return r.collect(now, true) }

func (r *errorReporter) collect(now time.Time, all bool) []GameEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []GameEvent
	for _, g := range r.groups {
		if g.pending > 0 && (all || now.Sub(g.last) >= errorEvery) {
			out = append(out, g.take(g.sample, now))
		}
	}
	return out
}

// stackKey: hash dos 3 primeiros quadros da stack depois do panic, fora do runtime (função
// e arquivo:linha). O mesmo erro cai no mesmo grupo; os quadros vão junto no evento.
func stackKey(stack []byte) (string, []string) {
	lines := strings.Split(string(stack), "\n")
	start := 1 // linha 0: "goroutine N [running]:"
	for i, l := range lines {
		if strings.HasPrefix(l, "panic(") {
			start = i + 2 // o que vem antes é o próprio recover
			break
		}
	}
	var frames []string
	for i := start; i+1 < len(lines) && len(frames) < 3; i += 2 {
		fn, file := lines[i], strings.TrimSpace(lines[i+1])
		if fn == "" || strings.HasPrefix(fn, "\t") || strings.HasPrefix(fn, "runtime.") ||
			strings.HasPrefix(fn, "runtime/") || strings.HasPrefix(fn, "panic(") {
			continue
		}
		if strings.HasSuffix(fn, ")") {
			if k := strings.LastIndex(fn, "("); k > 0 {
				fn = fn[:k] // sem os argumentos (ponteiros mudam a cada vez)
			}
		}
		if k := strings.LastIndex(fn, "/"); k >= 0 {
			fn = fn[k+1:]
		}
		if k := strings.Index(file, " +0x"); k >= 0 {
			file = file[:k]
		}
		frames = append(frames, fn+" "+path.Base(file))
	}
	h := fnv.New64a()
	h.Write([]byte(strings.Join(frames, "|")))
	return fmt.Sprintf("%016x", h.Sum64()), frames
}

// cleanText corta o texto e tira o NUL (o JSONB do Postgres recusa \u0000, e um só evento
// ruim derrubaria o lote inteiro da telemetria).
func cleanText(s string, n int) string {
	if len(s) > n {
		s = s[:n]
	}
	return strings.ToValidUTF8(strings.ReplaceAll(s, "\x00", ""), "?")
}

// ---------- Goroutines dos workers ----------

// goWorker roda um worker da zona numa goroutine com recover: um panic é registrado e o
// worker recomeça depois de 1 s (o trabalho do panic se perde; o processo não cai).
func (z *Zone) goWorker(ctx context.Context, name string, fn func(context.Context)) {
	go func() {
		for z.runWorker(ctx, name, fn) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}()
}

// runWorker devolve true se o worker terminou por panic.
func (z *Zone) runWorker(ctx context.Context, name string, fn func(context.Context)) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			z.ReportPanic("worker:"+name, 0, "", r, debug.Stack())
			panicked = true
		}
	}()
	fn(ctx)
	return false
}

// rethrowInvariant: no modo estrito (testes e DEBUG_INVARIANTS=1) a invariante quebrada
// tem que derrubar de verdade; os recovers deixam esse panic passar.
func rethrowInvariant(r any) {
	if v, ok := r.(invariantPanic); ok {
		panic(v)
	}
}
