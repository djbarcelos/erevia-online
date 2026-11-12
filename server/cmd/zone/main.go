// Comando zone: servidor de jogo do Erevia (uma zona por processo, por enquanto).
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

	"erevia/server/internal/db"
	gamenet "erevia/server/internal/net"
	"erevia/server/internal/zone"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	addr := env("ADDR", ":8080")
	origins := strings.Split(env("ALLOWED_ORIGINS", "localhost:*,127.0.0.1:*"), ",")
	dbURL := os.Getenv("DATABASE_URL")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if dbURL != "" {
		pool, err := db.Connect(ctx, dbURL)
		if err != nil {
			log.Error("database", "err", err)
			os.Exit(1)
		}
		defer pool.Close()
		log.Info("postgres connected")
	} else {
		log.Warn("DATABASE_URL vazio: rodando sem banco")
	}

	z := zone.New(zone.DefaultConfig(), log.With("zone", "starter"))
	go z.Run(ctx)

	mux := http.NewServeMux()
	mux.Handle("/ws", gamenet.NewServer(z, log, origins))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})

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

	log.Info("listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("http", "err", err)
		os.Exit(1)
	}
	log.Info("bye")
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
