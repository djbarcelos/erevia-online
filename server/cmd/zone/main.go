// Comando zone: servidor de jogo do Erevia (API HTTP + WebSocket + uma zona).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	stdnet "net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"erevia/server/internal/api"
	gamenet "erevia/server/internal/net"
	"erevia/server/internal/store"
	"erevia/server/internal/world"
	"erevia/server/internal/zone"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	startedAt := time.Now() // uptime no portal de administração
	addr := env("ADDR", "")
	if addr == "" {
		addr = ":" + env("PORT", "8080") // a Railway define PORT
	}
	publicURL := strings.TrimRight(env("PUBLIC_URL", "http://localhost:5173"), "/")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errors.New("DATABASE_URL é obrigatória (use `make db-up` localmente)")
	}
	origins := strings.Split(env("ALLOWED_ORIGINS", "localhost:*,127.0.0.1:*"), ",")
	devLogin := os.Getenv("DEV_LOGIN") == "true"

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, dbURL)
	if err != nil {
		return err
	}
	defer st.Close()
	log.Info("postgres ready (migrations applied)")

	cfg := api.Config{PublicURL: publicURL, DevLogin: devLogin}
	if id, secret := os.Getenv("GOOGLE_CLIENT_ID"), os.Getenv("GOOGLE_CLIENT_SECRET"); id != "" && secret != "" {
		cfg.Google = api.NewGoogle(id, secret, publicURL)
	}
	if id, secret := os.Getenv("DISCORD_CLIENT_ID"), os.Getenv("DISCORD_CLIENT_SECRET"); id != "" && secret != "" {
		cfg.Discord = api.NewDiscord(id, secret, publicURL)
	}
	if devLogin {
		log.Warn("DEV_LOGIN ativo: qualquer um entra sem senha. Nunca use em produção.")
	}
	log.Info("auth providers", "google", cfg.Google != nil, "discord", cfg.Discord != nil, "dev", devLogin)
	httpAPI := api.New(st, cfg, log)

	// Persistência: a zona entrega estados por canal; uma goroutine (abaixo, depois de criar
	// a zona, que registra os erros) grava em lote.
	saves := make(chan []zone.SavedState, 1024)
	persistDone := make(chan struct{})

	wld, err := world.Load()
	if err != nil {
		return err
	}
	zcfg := zone.DefaultConfig()
	zcfg.World = wld
	zcfg.Size = wld.Size
	zcfg.SafeRadius = wld.SafeRadius
	lctx, lcancel := context.WithTimeout(ctx, 30*time.Second)
	clans, err := st.LoadClans(lctx)
	lcancel()
	if err != nil {
		return fmt.Errorf("load clans: %w", err)
	}
	zcfg.Clans = clanData(clans)
	lctx, lcancel = context.WithTimeout(ctx, 30*time.Second)
	bosses, err := st.LoadBosses(lctx)
	lcancel()
	if err != nil {
		return fmt.Errorf("load bosses: %w", err)
	}
	for _, b := range bosses {
		zcfg.Bosses = append(zcfg.Bosses, zone.BossData{ID: b.ID, RespawnAt: b.RespawnAt, KilledAt: b.KilledAt, TopName: b.TopName})
	}
	lctx, lcancel = context.WithTimeout(ctx, 30*time.Second)
	events, err := st.LoadServerEvents(lctx)
	lcancel()
	if err != nil {
		return fmt.Errorf("load server events: %w", err)
	}
	for _, e := range events {
		zcfg.Events = append(zcfg.Events, zone.EventData{ID: e.ID, Until: e.Until})
	}
	// Eventos: chance de invasão por noite e dias de XP em dobro (vazio ou "nenhum" = só GM).
	if v := os.Getenv("INVASION_CHANCE"); v != "" {
		if c, err := strconv.ParseFloat(v, 64); err == nil && c >= 0 && c <= 1 {
			zcfg.InvasionChance = c
		} else {
			log.Error("INVASION_CHANCE inválida (0 a 1), usando o padrão", "value", v)
		}
	}
	if v, ok := os.LookupEnv("XP2_DAYS"); ok {
		if days, err := zone.ParseWeekdays(v); err == nil {
			zcfg.XP2Days = days
		} else {
			log.Error("XP2_DAYS inválida, usando o padrão", "err", err)
		}
	}
	zcfg.DevCommands = devLogin                         // /raid no chat: só em desenvolvimento
	zcfg.Version = env("RAILWAY_GIT_COMMIT_SHA", "dev") // a Railway define o commit do deploy
	z := zone.New(zcfg, log.With("zone", "starter"), saves)
	z.SetClanBackend(clanStore{st})
	z.SetBossBackend(bossStore{st})
	z.SetEventBackend(serverEventStore{st})
	z.SetModBackend(modStore{st})
	z.SetSocialBackend(socialStore{st})
	z.SetEventSink(eventStore{st})
	z.SetRankingBackend(rankingStore{st})
	go func() {
		defer close(persistDone)
		for batch := range saves {
			saveBatch(st, z, log, batch)
		}
	}()

	// Conta apagada: o banco já apagou tudo; a zona derruba quem estiver online e acerta os
	// clãs em memória do jeito que o banco deixou.
	httpAPI.OnAccountDeleted(func(d store.DeletedAccount) {
		g := zone.AccountGone{AccountID: d.AccountID, Characters: d.Characters}
		for _, c := range d.Clans {
			g.Clans = append(g.Clans, zone.ClanSuccession{ClanID: c.ClanID, LeaderID: c.LeaderID})
		}
		pctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		z.AccountDeleted(pctx, g)
		cancel()
	})
	zoneDone := make(chan struct{})
	go func() { z.Run(ctx); close(zoneDone) }()

	// Portal de administração (/admin e /api/admin/*): só GM; "online agora" vem da zona pelo canal.
	httpAPI.SetAdmin(api.AdminConfig{Version: zcfg.Version, StartedAt: startedAt, Zone: z})
	mux := http.NewServeMux()
	httpAPI.Register(mux)
	mux.Handle("/ws", gamenet.NewServer(z, &wsBackend{api: httpAPI, store: st}, log, origins))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		pctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := st.Ping(pctx); err != nil {
			http.Error(w, "db down", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})
	if dir := os.Getenv("STATIC_DIR"); dir != "" {
		mux.Handle("/", staticHandler(dir, httpAPI.AdminOnly))
		log.Info("serving client", "dir", dir)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           recoverHTTP(z, mux),
		ReadHeaderTimeout: 5 * time.Second,
		// Conexões WebSocket herdam ctx e são encerradas no SIGINT.
		BaseContext: func(stdnet.Listener) context.Context { return ctx },
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Info("listening", "addr", addr, "public_url", publicURL)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// Espera a zona salvar todo mundo e a persistência terminar de gravar.
	<-zoneDone
	select {
	case <-persistDone:
	case <-time.After(10 * time.Second):
		log.Warn("persistence did not finish in time")
	}
	// E a telemetria gravar o fim das sessões de quem estava online (tempo de jogo por nível).
	select {
	case <-z.TelemetryDone():
	case <-time.After(10 * time.Second):
		log.Warn("telemetry did not finish in time")
	}
	log.Info("bye")
	return nil
}

// saveBatch grava um lote de personagens (e armazéns). Falhas viram db_error; um panic
// (dado estranho no lote) é registrado e perde só este lote, sem derrubar o processo.
func saveBatch(st *store.Store, z *zone.Zone, log *slog.Logger, batch []zone.SavedState) {
	defer func() {
		if r := recover(); r != nil {
			z.ReportPanic("persist", 0, "", r, debug.Stack())
		}
	}()
	states := make([]store.CharacterState, len(batch))
	for i, p := range batch {
		states[i] = store.CharacterState{
			CharacterID: int64(p.ID), X: p.X, Z: p.Z, Level: p.Level, XP: p.XP, HP: p.HP, MP: p.MP,
			Gold: p.Gold, Items: p.Items, Attrs: p.Attrs, Ranks: p.Ranks, Hotbar: p.Hotbar, Seq: p.Seq,
			Karma: p.Karma, PKs: p.PKs, PvPKills: p.PvPKills, Appearance: p.Look, Quests: p.Quests,
			Achievements: p.Achievements, Title: p.Title, Spec: p.Spec,
		}
		if s := p.Storage; s != nil {
			states[i].Storage = &store.StorageState{AccountID: s.AccountID, Gold: s.Gold, Items: s.Items, Seq: s.Seq}
		}
	}
	// Tenta de novo se o banco falhar (o save_seq impede que uma tentativa atrasada
	// desfaça um salvamento mais novo).
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err = st.SaveStates(sctx, states)
		cancel()
		if err == nil {
			return
		}
		log.Error("save states", "err", err, "count", len(states), "attempt", attempt)
		z.ReportDBError("save_characters", err)
		time.Sleep(time.Duration(attempt) * time.Second)
	}
	// As 3 tentativas falharam: este lote não foi gravado (o próximo salvamento do
	// personagem, a cada 30 s, leva o estado mais novo).
	z.ReportDBError("save_characters_lost", err)
}

// recoverHTTP registra (server_error) um panic num handler HTTP e responde 500. O net/http
// já não deixa o processo cair, mas sem isto o erro só ia para o log.
func recoverHTTP(z *zone.Zone, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler { // aborto pedido pelo próprio handler
				panic(v)
			}
			path := r.URL.Path
			if len(path) > 80 {
				path = path[:80]
			}
			z.ReportPanic("http:"+r.Method+" "+path, 0, "", v, debug.Stack())
			http.Error(w, "erro interno", http.StatusInternalServerError)
		}()
		h.ServeHTTP(w, r)
	})
}

// wsBackend liga o WebSocket à sessão HTTP e ao banco.
type wsBackend struct {
	api   *api.API
	store *store.Store
}

func (b *wsBackend) Authenticate(r *http.Request) (int64, error) { return b.api.Authenticate(r) }

func (b *wsBackend) LoadCharacter(ctx context.Context, accountID int64, id uint32) (gamenet.Character, error) {
	c, err := b.store.CharacterForAccount(ctx, accountID, int64(id))
	if errors.Is(err, store.ErrNotFound) {
		return gamenet.Character{}, gamenet.ErrCharacterNotFound
	}
	if err != nil {
		return gamenet.Character{}, err
	}
	return gamenet.Character{
		ID: uint32(c.ID), Name: c.Name, Class: c.Class, X: c.X, Z: c.Z,
		XP: c.XP, HP: c.HP, MP: c.MP, Gold: c.Gold, Items: c.Items,
		Attrs: c.Attrs, Ranks: c.Ranks, Hotbar: c.Hotbar, Emblem: c.Emblem, Look: c.Appearance,
		AccountID: c.AccountID, Karma: c.Karma, PKs: c.PKs, PvPKills: c.PvPKills,
		ClanLeftAt: derefTime(c.ClanLeftAt),
		Storage:    &zone.StorageData{Gold: c.Storage.Gold, Items: c.Storage.Items, Seq: c.Storage.Seq},
		GM:         c.GM, MutedUntil: derefTime(c.MutedUntil), Quests: c.Quests,
		Achievements: c.Achievements, Title: c.Title, Spec: c.Spec,
	}, nil
}

// eventStore liga a telemetria da zona ao banco.
type eventStore struct{ st *store.Store }

func (e eventStore) InsertEvents(ctx context.Context, events []zone.GameEvent) error {
	rows := make([]store.GameEvent, len(events))
	for i, ev := range events {
		rows[i] = store.GameEvent(ev)
	}
	return e.st.InsertEvents(ctx, rows)
}

// modStore liga a moderação da zona ao banco.
type modStore struct{ st *store.Store }

func (m modStore) SetBanned(ctx context.Context, name string, banned bool, reason string) error {
	_, err := m.st.SetBanned(ctx, name, banned, reason)
	if errors.Is(err, store.ErrNotFound) {
		return zone.ErrNoSuchCharacter
	}
	return err
}

func (m modStore) SetMute(ctx context.Context, accountID int64, until time.Time) error {
	return m.st.SetMute(ctx, accountID, until)
}

func (m modStore) LogGM(ctx context.Context, e zone.GMLog) error {
	return m.st.LogGM(ctx, store.GMEntry{GMAccountID: e.GMAccountID, GMName: e.GMName, Command: e.Command, Target: e.Target, Detail: e.Detail})
}

func (m modStore) SaveBug(ctx context.Context, b zone.BugReport) error {
	return m.st.SaveBugReport(ctx, store.BugReport{
		AccountID: b.AccountID, CharacterID: int64(b.CharacterID), CharacterName: b.Name, Level: b.Level,
		X: b.X, Z: b.Z, Version: b.Version, UserAgent: b.UserAgent, Text: b.Text,
	})
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

// clanData converte os clãs do banco para a zona.
func clanData(list []store.Clan) []zone.ClanData {
	out := make([]zone.ClanData, len(list))
	for i, c := range list {
		d := zone.ClanData{ID: c.ID, Name: c.Name, Notice: c.Notice, LeaderID: c.LeaderID}
		for _, m := range c.Members {
			d.Members = append(d.Members, zone.ClanMemberData{CharacterID: m.CharacterID, Name: m.Name, Class: m.Class, Level: m.Level, Rank: m.Rank})
		}
		out[i] = d
	}
	return out
}

// clanStore liga o worker de clãs da zona ao banco.
type clanStore struct{ st *store.Store }

func (c clanStore) CreateClan(ctx context.Context, name string, leaderID int64) (int64, error) {
	id, err := c.st.CreateClan(ctx, name, leaderID)
	if errors.Is(err, store.ErrNameTaken) {
		return 0, zone.ErrClanNameTaken
	}
	return id, err
}

func (c clanStore) ApplyClanOp(ctx context.Context, op zone.ClanOp) error {
	return c.st.ApplyClanOp(ctx, store.ClanOp{
		Kind: store.ClanOpKind(op.Kind), ClanID: op.ClanID, CharacterID: op.CharacterID,
		Rank: op.Rank, Text: op.Text, LeftAt: op.LeftAt,
	})
}

// socialStore liga amigos e correio da zona ao banco.
type socialStore struct{ st *store.Store }

// socialErr traduz os erros do banco para os da zona.
func socialErr(err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return zone.ErrNoSuchCharacter
	case errors.Is(err, store.ErrSelf):
		return zone.ErrSelf
	case errors.Is(err, store.ErrFriendLimit):
		return zone.ErrFriendListFull
	case errors.Is(err, store.ErrMailboxFull):
		return zone.ErrMailboxFull
	case errors.Is(err, store.ErrMailRate):
		return zone.ErrMailRate
	}
	return err
}

func friendData(f store.Friend) zone.FriendData {
	return zone.FriendData{ID: uint32(f.ID), Name: f.Name, Class: f.Class, Level: f.Level}
}

func (s socialStore) Friends(ctx context.Context, id uint32) ([]zone.FriendData, error) {
	list, err := s.st.Friends(ctx, int64(id))
	out := make([]zone.FriendData, len(list))
	for i, f := range list {
		out[i] = friendData(f)
	}
	return out, err
}

func (s socialStore) AddFriend(ctx context.Context, id uint32, name string, max int) (zone.FriendData, error) {
	f, err := s.st.AddFriend(ctx, int64(id), name, max)
	return friendData(f), socialErr(err)
}

func (s socialStore) RemoveFriend(ctx context.Context, id, friend uint32) error {
	return s.st.RemoveFriend(ctx, int64(id), int64(friend))
}

func (s socialStore) SendMail(ctx context.Context, m zone.MailOut) (zone.MailSent, error) {
	r, err := s.st.SendMail(ctx, store.MailOut{
		FromID: int64(m.FromID), FromName: m.FromName, ToName: m.ToName, Subject: m.Subject, Body: m.Body,
		At: m.At, PerHour: m.PerHour, MaxBox: m.MaxBox,
	})
	return zone.MailSent{ToID: uint32(r.ToID), ToName: r.ToName, Unread: r.Unread, Total: r.Total}, socialErr(err)
}

func (s socialStore) MailSummary(ctx context.Context, id uint32) (int, int, error) {
	return s.st.MailSummary(ctx, int64(id))
}

func (s socialStore) Inbox(ctx context.Context, id uint32) ([]zone.MailData, error) {
	list, err := s.st.Inbox(ctx, int64(id))
	out := make([]zone.MailData, len(list))
	for i, m := range list {
		out[i] = zone.MailData(m)
	}
	return out, err
}

func (s socialStore) MarkMailRead(ctx context.Context, id uint32, mailID int64) error {
	return s.st.MarkMailRead(ctx, int64(id), mailID)
}

func (s socialStore) DeleteMail(ctx context.Context, id uint32, mailID int64, at time.Time) error {
	return s.st.DeleteMail(ctx, int64(id), mailID, at)
}

// rankingStore liga o worker do ranking ao banco.
type rankingStore struct{ st *store.Store }

func (r rankingStore) Rankings(ctx context.Context, limit int) (zone.RankingData, error) {
	d, err := r.st.Rankings(ctx, limit)
	if err != nil {
		return zone.RankingData{}, err
	}
	rows := func(list []store.RankRow) []zone.RankRow {
		out := make([]zone.RankRow, len(list))
		for i, x := range list {
			out[i] = zone.RankRow(x)
		}
		return out
	}
	out := zone.RankingData{At: d.At, Level: rows(d.Level), PvP: rows(d.PvP), LevelPos: d.LevelPos, PvPPos: d.PvPPos, ClanPos: d.ClanPos}
	for _, c := range d.Clans {
		out.Clans = append(out.Clans, zone.ClanRankRow(c))
	}
	return out, nil
}

// bossStore liga o worker de renascimento dos raid bosses ao banco.
type bossStore struct{ st *store.Store }

func (b bossStore) SaveBoss(ctx context.Context, d zone.BossData) error {
	return b.st.SaveBoss(ctx, store.Boss{ID: d.ID, RespawnAt: d.RespawnAt, KilledAt: d.KilledAt, TopName: d.TopName})
}

// serverEventStore liga o worker dos eventos de GM (XP em dobro) ao banco.
type serverEventStore struct{ st *store.Store }

func (e serverEventStore) SaveEvent(ctx context.Context, d zone.EventData) error {
	return e.st.SaveServerEvent(ctx, store.ServerEvent{ID: d.ID, Until: d.Until})
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
