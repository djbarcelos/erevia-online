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

	// Persistência: a zona entrega estados por canal; esta goroutine grava em lote.
	saves := make(chan []zone.SavedState, 1024)
	persistDone := make(chan struct{})
	go func() {
		defer close(persistDone)
		for batch := range saves {
			states := make([]store.CharacterState, len(batch))
			for i, p := range batch {
				states[i] = store.CharacterState{
					CharacterID: int64(p.ID), X: p.X, Z: p.Z, Level: p.Level, XP: p.XP, HP: p.HP, MP: p.MP,
					Gold: p.Gold, Items: p.Items, Attrs: p.Attrs, Ranks: p.Ranks, Hotbar: p.Hotbar, Seq: p.Seq,
					Karma: p.Karma, PKs: p.PKs, PvPKills: p.PvPKills, Appearance: p.Look, Quests: p.Quests,
				}
				if s := p.Storage; s != nil {
					states[i].Storage = &store.StorageState{AccountID: s.AccountID, Gold: s.Gold, Items: s.Items, Seq: s.Seq}
				}
			}
			// Tenta de novo se o banco falhar (o save_seq impede que uma tentativa atrasada
			// desfaça um salvamento mais novo).
			for attempt := 1; attempt <= 3; attempt++ {
				sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				err := st.SaveStates(sctx, states)
				cancel()
				if err == nil {
					break
				}
				log.Error("save states", "err", err, "count", len(states), "attempt", attempt)
				time.Sleep(time.Duration(attempt) * time.Second)
			}
		}
	}()

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
	zcfg.DevCommands = devLogin                         // /raid no chat: só em desenvolvimento
	zcfg.Version = env("RAILWAY_GIT_COMMIT_SHA", "dev") // a Railway define o commit do deploy
	z := zone.New(zcfg, log.With("zone", "starter"), saves)
	z.SetClanBackend(clanStore{st})
	z.SetBossBackend(bossStore{st})
	z.SetModBackend(modStore{st})
	z.SetEventSink(eventStore{st})
	zoneDone := make(chan struct{})
	go func() { z.Run(ctx); close(zoneDone) }()

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
		mux.Handle("/", staticHandler(dir))
		log.Info("serving client", "dir", dir)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
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
	log.Info("bye")
	return nil
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

// bossStore liga o worker de renascimento dos raid bosses ao banco.
type bossStore struct{ st *store.Store }

func (b bossStore) SaveBoss(ctx context.Context, d zone.BossData) error {
	return b.st.SaveBoss(ctx, store.Boss{ID: d.ID, RespawnAt: d.RespawnAt, KilledAt: d.KilledAt, TopName: d.TopName})
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
