package store_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"erevia/server/internal/game"
	"erevia/server/internal/store"
	"erevia/server/internal/store/storetest"
)

// rawDB abre uma conexão direta no schema do teste (para forjar dados).
func rawDB(t *testing.T, url string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close(context.Background()) })
	return c
}

func mustExec(t *testing.T, db *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := db.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func newChar(t *testing.T, s *store.Store, user, name, class string) store.Character {
	t.Helper()
	ctx := context.Background()
	acc, err := s.LoginIdentity(ctx, "dev", strings.ToLower(user), user)
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.CreateCharacter(ctx, acc, name, class, game.DefaultLook(class))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func balanceOf(rows []store.ItemBalance, id string) store.ItemBalance {
	for _, r := range rows {
		if r.Item == id {
			return r
		}
	}
	return store.ItemBalance{Item: id}
}

// Um item duplicado (existe sem ter entrado pela telemetria) aparece como sobra; a troca
// entre jogadores, o armazém e o kit inicial não.
func TestItemBalanceDetectsDuplication(t *testing.T) {
	s, url := storetest.NewWithURL(t)
	db := rawDB(t, url)
	ctx := context.Background()
	ana := newChar(t, s, "ana", "Ana", "knight")
	beto := newChar(t, s, "beto", "Beto", "mage")
	now := time.Now().Add(-10 * time.Minute) // já salvo (ver saveLag)

	// Ana pegou 3 anéis no chão e vendeu 1: sobram 2 explicados pela telemetria.
	ev := func(kind, ref string, n int64, how string) store.GameEvent {
		return store.GameEvent{At: now, Kind: kind, CharacterID: ana.ID, AccountID: ana.AccountID, Ref: ref, Amount: n,
			Extra: map[string]any{"how": how}}
	}
	if err := s.InsertEvents(ctx, []store.GameEvent{
		ev("item_in", "ring_good", 3, "loot"), ev("item_out", "ring_good", 1, "sell"),
		{At: now, Kind: "gold", CharacterID: ana.ID, Ref: "sell", Amount: 40},
	}); err != nil {
		t.Fatal(err)
	}
	// Ana ficou com 1; o outro foi para Beto numa troca, que guardou no armazém (sem evento).
	mustExec(t, db, `INSERT INTO character_items (character_id, place, slot, item_id, count) VALUES ($1, 0, 9, 'ring_good', 1)`, ana.ID)
	mustExec(t, db, `INSERT INTO account_items (account_id, slot, item_id, count) VALUES ($1, 0, 'ring_good', 1)`, beto.AccountID)

	cut := time.Now().Add(-5 * time.Minute)
	before, err := s.TelemetryTotalsBefore(ctx, cut)
	if err != nil || before.ItemsIn["ring_good"] != 3 || before.Gold != 40 {
		t.Fatalf("before = %+v %v", before, err)
	}
	rows, err := s.ItemBalances(ctx, before)
	if err != nil {
		t.Fatal(err)
	}
	ring := balanceOf(rows, "ring_good")
	if ring.Existing != 2 || ring.Bags != 1 || ring.Storage != 1 || ring.In != 3 || ring.Out != 1 || ring.Expected != 2 || ring.Surplus != 0 {
		t.Fatalf("ring before dup = %+v", ring)
	}
	// O kit inicial (arma, escudo e poções) não gera evento e não pode virar suspeita.
	for _, r := range rows {
		if r.Surplus > 0 {
			t.Fatalf("false positive before dup: %+v", r)
		}
	}
	if sw := balanceOf(rows, "sword_common"); sw.Existing != 1 || sw.Kit < 1 || sw.Surplus > 0 {
		t.Fatalf("kit sword = %+v", sw)
	}

	// Uma venda de agora ainda não chegou ao banco do personagem: não vira sobra.
	if err := s.InsertEvents(ctx, []store.GameEvent{{At: time.Now(), Kind: "item_out", CharacterID: ana.ID, Ref: "ring_good", Amount: 1,
		Extra: map[string]any{"how": "sell"}}}); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.ItemBalances(ctx, before); balanceOf(r, "ring_good").Surplus != 0 {
		t.Fatalf("fresh sale counted: %+v", balanceOf(r, "ring_good"))
	}

	// Duplicação forjada: a mesma pilha aparece de novo na bolsa de Beto.
	mustExec(t, db, `INSERT INTO character_items (character_id, place, slot, item_id, count) VALUES ($1, 0, 9, 'ring_good', 1)`, beto.ID)
	rows, err = s.ItemBalances(ctx, before)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Item != "ring_good" || rows[0].Surplus != 1 || rows[0].Existing != 3 || rows[0].Name == "" {
		t.Fatalf("dup not first: %+v", rows[0])
	}

	// Ouro: o que existe é o inicial dos dois mais os 40 da venda.
	sup, err := s.GoldSupply(ctx, before)
	if err != nil {
		t.Fatal(err)
	}
	if sup.Characters != 2 || sup.Expected != 2*game.StarterGold+40 || sup.Total != 2*game.StarterGold || sup.Surplus != -40 {
		t.Fatalf("supply = %+v", sup)
	}
	// Ouro forjado no armazém aparece como sobra.
	mustExec(t, db, `UPDATE accounts SET stored_gold = 10000 WHERE id = $1`, beto.AccountID)
	if sup, _ = s.GoldSupply(ctx, before); sup.Surplus != 10000-40 || sup.Storage != 10000 {
		t.Fatalf("forged gold = %+v", sup)
	}
}

func TestKitCeilingCoversEveryChoice(t *testing.T) {
	for class, opts := range game.StarterLooks {
		ceil := store.KitCeiling(class)
		for _, w := range opts.Weapons {
			if ceil[w] < 1 {
				t.Errorf("%s: weapon %s not in the ceiling", class, w)
			}
		}
		if ceil["potion_hp_s"] < 5 {
			t.Errorf("%s: potions = %d", class, ceil["potion_hp_s"])
		}
	}
}

// Um dia com 10× a média dos outros personagens do mesmo nível vira alerta; dias normais não.
func TestSpikesFlagOutliers(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	day := time.Now().UTC().Truncate(24 * time.Hour).Add(2 * time.Hour)
	var evs []store.GameEvent
	var chars []store.Character
	for i, n := range []string{"Ana", "Beto", "Caio", "Duda", "Eva"} {
		c := newChar(t, s, n, n, "knight")
		chars = append(chars, c)
		for d := range 3 {
			amount := int64(100 + 10*i)
			if n == "Eva" && d == 1 {
				amount = 5000 // o pico
			}
			evs = append(evs, store.GameEvent{At: day.AddDate(0, 0, -d), Kind: "gold", CharacterID: c.ID, Level: 5, Ref: "loot", Amount: amount})
		}
	}
	if err := s.InsertEvents(ctx, evs); err != nil {
		t.Fatal(err)
	}
	from := day.Truncate(24*time.Hour).AddDate(0, 0, -6)
	sp, err := s.Spikes(ctx, from, 0, store.SpikeRule{})
	if err != nil {
		t.Fatal(err)
	}
	if len(sp) != 1 || sp[0].Name != "Eva" || sp[0].What != "gold" || sp[0].Amount != 5000 || sp[0].Ratio < 10 {
		t.Fatalf("spikes = %+v", sp)
	}
	if only, _ := s.Spikes(ctx, from, chars[0].ID, store.SpikeRule{}); len(only) != 0 {
		t.Fatalf("Ana has no spike: %+v", only)
	}
	hist, flows, err := s.CharacterHistory(ctx, chars[4].ID, from)
	if err != nil || len(hist) != 3 || len(flows) != 1 || flows[0].Amount != 5000+2*140 {
		t.Fatalf("history = %+v %+v %v", hist, flows, err)
	}
	if c, err := s.FindCharacter(ctx, "eva"); err != nil || c.ID != chars[4].ID {
		t.Fatalf("find = %+v %v", c, err)
	}
}

func TestBugReportsListResolveAndExtra(t *testing.T) {
	s, url := storetest.NewWithURL(t)
	db := rawDB(t, url)
	ctx := context.Background()
	ana := newChar(t, s, "ana", "Ana", "knight")
	for _, b := range []store.BugReport{
		{AccountID: ana.AccountID, CharacterID: ana.ID, CharacterName: "Ana", Level: 3, X: 0, Z: 0, Version: "abc1234def", Text: "porta da taverna 100% travada"},
		{AccountID: ana.AccountID, CharacterID: ana.ID, CharacterName: "Ana", Level: 4, X: 60, Z: 5, Version: "abc1234def", Text: "monstro preso na pedra"},
		{AccountID: ana.AccountID, CharacterName: "Ana", Level: 9, X: 0, Z: -270, Version: "fff0000", Text: "golem invisível"},
	} {
		if err := s.SaveBugReport(ctx, b); err != nil {
			t.Fatal(err)
		}
	}
	list := func(f store.BugFilter) []store.BugRow {
		t.Helper()
		rows, err := s.ListBugs(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	if all := list(store.BugFilter{}); len(all) != 3 || all[0].Text != "golem invisível" || all[0].Region != "Vale das Pedras Vivas" {
		t.Fatalf("open = %+v", all)
	}
	if r := list(store.BugFilter{Version: "abc1234"}); len(r) != 2 {
		t.Fatalf("by version = %d", len(r))
	}
	if r := list(store.BugFilter{Region: "Arredores"}); len(r) != 1 || r[0].Text != "monstro preso na pedra" {
		t.Fatalf("by region = %+v", r)
	}
	// % no texto é literal (não curinga).
	if r := list(store.BugFilter{Query: "100%"}); len(r) != 1 {
		t.Fatalf("by text = %+v", r)
	}
	if r := list(store.BugFilter{Query: "1%0"}); len(r) != 0 {
		t.Fatalf("escaped like = %+v", r)
	}

	first := list(store.BugFilter{})[2]
	got, err := s.ResolveBug(ctx, first.ID, true, ana.AccountID, "Mestre", "corrigido no abc")
	if err != nil || !got.Resolved || got.ResolvedAt == nil || got.ResolvedByName != "Mestre" || got.ResolvedNote != "corrigido no abc" {
		t.Fatalf("resolve = %+v %v", got, err)
	}
	if r := list(store.BugFilter{}); len(r) != 2 {
		t.Fatalf("open after resolve = %d", len(r))
	}
	if r := list(store.BugFilter{Status: "resolved"}); len(r) != 1 || r[0].ID != first.ID {
		t.Fatalf("resolved = %+v", r)
	}
	if r := list(store.BugFilter{Status: "all", Before: first.ID + 1, Limit: 1}); len(r) != 1 || r[0].ID != first.ID {
		t.Fatalf("page = %+v", r)
	}
	f, err := s.BugFacets(ctx)
	if err != nil || f.Open != 2 || f.Resolved != 1 || len(f.Versions) != 2 || len(f.Regions) != 3 {
		t.Fatalf("facets = %+v %v", f, err)
	}
	re, _ := s.ResolveBug(ctx, first.ID, false, ana.AccountID, "Mestre", "")
	if re.Resolved || re.ResolvedAt != nil || re.ResolvedByName != "" {
		t.Fatalf("reopen = %+v", re)
	}
	if _, err := s.ResolveBug(ctx, 999999, true, 1, "x", ""); err != store.ErrNotFound {
		t.Fatalf("missing = %v", err)
	}

	// Colunas que outra migração acrescentar (o /bug completo) aparecem no detalhe.
	_, extra, err := s.BugDetail(ctx, first.ID)
	if err != nil || len(extra) != 0 {
		t.Fatalf("extra before = %v %v", extra, err)
	}
	// O contexto do /bug completo (00026) e uma coluna futura grande (uma imagem, por exemplo).
	mustExec(t, db, `ALTER TABLE bug_reports ADD COLUMN screenshot BYTEA`)
	mustExec(t, db, `UPDATE bug_reports SET context = '{"fps": 41, "ping": 80}', screenshot = decode(repeat('ab', 20000), 'hex') WHERE id = $1`, first.ID)
	mustExec(t, db, `INSERT INTO bug_report_shots (report_id, width, height, jpeg) VALUES ($1, 320, 180, '\xffd8ffd9')`, first.ID)
	withShot, extra, err := s.BugDetail(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if withShot.Shot == nil || withShot.Shot.Width != 320 || withShot.Shot.Height != 180 {
		t.Fatalf("shot = %+v", withShot.Shot)
	}
	if r := list(store.BugFilter{Status: "all"}); r[len(r)-1].Shot == nil || r[0].Shot != nil {
		t.Fatalf("shot in list = %+v / %+v", r[len(r)-1].Shot, r[0].Shot)
	}
	var ctxVal map[string]float64
	var shot struct {
		Omitted bool
		Bytes   int
	}
	json.Unmarshal(extra["context"], &ctxVal)
	json.Unmarshal(extra["screenshot"], &shot)
	if ctxVal["fps"] != 41 || !shot.Omitted || shot.Bytes < 20000 {
		t.Fatalf("extra = %s", extra)
	}
	if _, _, err := s.BugDetail(ctx, 999999); err != store.ErrNotFound {
		t.Fatalf("missing detail = %v", err)
	}
}

func TestClientErrorsTopAndVersions(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	h := store.ClientErrorHash("boom", "at f (a.js:1:2)")
	if err := s.RecordClientErrors(ctx, []store.ClientError{
		{Hash: h, Version: "v1", Kind: "error", Message: "boom", Stack: "at f (a.js:1:2)", Count: 5, Region: "Cidade"},
		{Hash: h, Version: "v2", Kind: "error", Message: "boom", Stack: "at f (a.js:1:2)", Count: 2},
		{Hash: store.ClientErrorHash("raro", ""), Version: "v2", Kind: "webgl", Message: "raro", Count: 1},
	}); err != nil {
		t.Fatal(err)
	}
	top, err := s.TopClientErrors(ctx, "", 7, 10)
	if err != nil || len(top) != 2 || top[0].Count != 7 || len(top[0].Versions) != 2 || top[0].Message != "boom" {
		t.Fatalf("top = %+v %v", top, err)
	}
	if v2, _ := s.TopClientErrors(ctx, "v2", 7, 10); len(v2) != 2 || v2[0].Count != 2 {
		t.Fatalf("v2 = %+v", v2)
	}
	vs, err := s.ClientErrorVersions(ctx, 7)
	if err != nil || len(vs) != 2 {
		t.Fatalf("versions = %+v %v", vs, err)
	}
}

// As consultas do painel batem com as views da telemetria (00011) no mesmo período.
func TestMetricsQueriesMatchViews(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	now := time.Now()
	old := now.AddDate(0, 0, -20) // fora do período de 7 dias
	evs := []store.GameEvent{
		{At: now, Kind: "gold", CharacterID: 1, Ref: "loot", Amount: 30},
		{At: now, Kind: "gold", CharacterID: 1, Ref: "buy", Amount: -20},
		{At: old, Kind: "gold", CharacterID: 1, Ref: "loot", Amount: 999},
		{At: now.Add(-10 * time.Minute), Kind: "level", CharacterID: 1, Amount: 2},
		{At: now, Kind: "level", CharacterID: 1, Amount: 3},
		{At: now, Kind: "death", CharacterID: 1, X: 60, Z: 0, Ref: "monster:skeleton_minion", Level: 3},
		{At: now, Kind: "fps", CharacterID: 1, Amount: 58, Extra: map[string]any{"fps_low": 41, "device": "desktop mac"}},
		{At: now, Kind: "item_in", CharacterID: 1, Ref: "ring_rare", Amount: 2, Extra: map[string]any{"how": "craft"}},
		{At: now, Kind: "item_out", CharacterID: 1, Ref: "ring_rare", Amount: 1, Extra: map[string]any{"how": "sell"}},
		{At: now, Kind: "slow_tick", Ref: "tick", Amount: 4, Extra: map[string]any{"systems": "ai 30ms", "version": "abc"}},
		{At: now, Kind: "server_error", Ref: "h1", Amount: 1, Extra: map[string]any{"where": "msg", "panic": "nil map", "version": "abc"}},
		{At: now, Kind: "session", CharacterID: 1, Ref: "start"},
		{At: now, Kind: "session", CharacterID: 1, Ref: "end", Amount: 1800},
	}
	if err := s.InsertEvents(ctx, evs); err != nil {
		t.Fatal(err)
	}
	from := store.DayStart(now).AddDate(0, 0, -6)
	to := store.DayStart(now).Add(24 * time.Hour)
	gold, err := s.GoldDaily(ctx, from, to)
	if err != nil || len(gold) != 2 {
		t.Fatalf("gold = %+v %v", gold, err)
	}
	var in, out int64
	for _, g := range gold {
		in, out = in+g.In, out+g.Out
	}
	if in != 30 || out != 20 {
		t.Fatalf("gold in/out = %d/%d", in, out)
	}
	// Um dia fechado só tem o que é dele.
	if g, _ := s.GoldDaily(ctx, old.UTC().Truncate(24*time.Hour), old.UTC().Truncate(24*time.Hour).Add(24*time.Hour)); len(g) != 1 || g[0].In != 999 {
		t.Fatalf("old day = %+v", g)
	}
	if lv, err := s.LevelTimes(ctx, from); err != nil || len(lv) != 1 || lv[0].Level != 3 || lv[0].AvgMinutes != 10 {
		t.Fatalf("levels = %+v %v", lv, err)
	}
	if d, err := s.DeathsDaily(ctx, from, to); err != nil || len(d) != 1 || d[0].Region != "Arredores" || d[0].Deaths != 1 || d[0].LevelSum != 3 {
		t.Fatalf("deaths = %+v %v", d, err)
	}
	if f, err := s.FPSDaily(ctx, from, to); err != nil || len(f) != 1 || f[0].FPSSum != 58 || f[0].LowSum != 41 || f[0].LowN != 1 || f[0].Device != "desktop mac" {
		t.Fatalf("fps = %+v %v", f, err)
	}
	if days, err := s.ItemsDaily(ctx, from, to); err != nil || len(days) != 2 {
		t.Fatalf("items = %+v %v", days, err)
	}
	if top, err := s.ItemsByItemDaily(ctx, from, to); err != nil || len(top) != 1 || top[0].In != 2 || top[0].Out != 1 || top[0].Item != "ring_rare" {
		t.Fatalf("items by item = %+v %v", top, err)
	}
	sd, err := s.ServerEventsDaily(ctx, from, to)
	if err != nil || len(sd) != 2 {
		t.Fatalf("server = %+v %v", sd, err)
	}
	tops := []store.ServerEventTop{}
	for _, d := range sd {
		tops = append(tops, store.ServerEventTop{Kind: d.Kind, Ref: d.Ref, Count: d.Count, Last: d.Last})
	}
	if err := s.ServerEventContext(ctx, tops); err != nil {
		t.Fatal(err)
	}
	for _, tp := range tops {
		if (tp.Kind == "server_error" && (tp.Message != "nil map" || tp.Where != "msg" || tp.Version != "abc")) ||
			(tp.Kind == "slow_tick" && (tp.Count != 4 || tp.Message != "ai 30ms")) {
			t.Fatalf("server context = %+v", tops)
		}
	}
	if p, err := s.PlayersDaily(ctx, from, to); err != nil || len(p) != 1 || p[0].Characters != 1 || p[0].Sessions != 1 || p[0].Minutes != 30 {
		t.Fatalf("players = %+v %v", p, err)
	}
	if _, err := s.QuitLevel(ctx); err != nil {
		t.Fatal(err)
	}
	// A evolução do ouro termina na circulação de agora.
	nd := store.GoldNetDays(gold, 500)
	if len(nd) != 1 || nd[0].Net != 10 || nd[0].Supply != 500 {
		t.Fatalf("net daily = %+v", nd)
	}
}

func TestGMLogFilter(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	for _, e := range []store.GMEntry{
		{GMAccountID: 1, GMName: "Mestre", Command: "portal:bug.resolve", Target: "bug #1", Detail: "ok"},
		{GMAccountID: 1, GMName: "Mestre", Command: "give", Target: "Ana", Detail: "ring_good x1"},
		{GMAccountID: 2, GMName: "Outro", Command: "ban", Target: "Beto", Detail: "spam_100%"},
	} {
		if err := s.LogGM(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	get := func(f store.GMLogFilter) []store.GMLogRow {
		t.Helper()
		r, err := s.GMLog(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	if r := get(store.GMLogFilter{}); len(r) != 3 || r[0].Command != "ban" {
		t.Fatalf("all = %+v", r)
	}
	if r := get(store.GMLogFilter{Command: "portal:"}); len(r) != 1 {
		t.Fatalf("portal = %+v", r)
	}
	if r := get(store.GMLogFilter{GM: "mest"}); len(r) != 2 {
		t.Fatalf("gm = %+v", r)
	}
	if r := get(store.GMLogFilter{Target: "ana", Command: "give"}); len(r) != 1 {
		t.Fatalf("target = %+v", r)
	}
	if r := get(store.GMLogFilter{Query: "100%"}); len(r) != 1 || r[0].GMName != "Outro" {
		t.Fatalf("q = %+v", r)
	}
	if r := get(store.GMLogFilter{Since: time.Now().Add(time.Hour)}); len(r) != 0 {
		t.Fatalf("since = %+v", r)
	}
	if c, err := s.GMLogCommands(ctx); err != nil || len(c) != 3 {
		t.Fatalf("commands = %+v %v", c, err)
	}
}
