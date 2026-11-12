// Comando zone: servidor de jogo do Erevia (API HTTP + WebSocket + uma zona).
package main

import (
	"context"
	"errors"
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
				states[i] = store.CharacterState{CharacterID: int64(p.ID), X: p.X, Z: p.Z, Level: p.Level, XP: p.XP, HP: p.HP, MP: p.MP}
			}
			sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := st.SaveStates(sctx, states); err != nil {
				log.Error("save states", "err", err, "count", len(states))
			}
			cancel()
		}
	}()

	z := zone.New(zone.DefaultConfig(), log.With("zone", "starter"), saves)
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
		XP: c.XP, HP: c.HP, MP: c.MP,
	}, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
