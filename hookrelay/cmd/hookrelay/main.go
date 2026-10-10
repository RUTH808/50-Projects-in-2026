package main

import (
	"context"
	"errors"
	"github.com/RUTH808/hookrelay/internal/relay"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	token := os.Getenv("API_TOKEN")
	if len(token) < 24 {
		slog.Error("API_TOKEN must contain at least 24 characters")
		os.Exit(1)
	}
	mode := env("MODE", "all")
	if mode != "all" && mode != "api" && mode != "worker" {
		slog.Error("MODE must be all, api, or worker")
		os.Exit(1)
	}
	count, err := strconv.Atoi(env("WORKERS", "2"))
	if err != nil || count < 1 || count > 32 {
		slog.Error("WORKERS must be between 1 and 32")
		os.Exit(1)
	}
	pool, err := pgxpool.New(ctx, env("DATABASE_URL", "postgres://hookrelay:hookrelay-local@localhost:5433/hookrelay?sslmode=disable"))
	if err != nil {
		slog.Error("database config", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	store := &relay.Store{Pool: pool}
	if err = store.Migrate(ctx); err != nil {
		slog.Error("schema startup failed", "error", err)
		os.Exit(1)
	}
	allowPrivate := os.Getenv("ALLOW_PRIVATE_ENDPOINTS") == "true"
	var wg sync.WaitGroup
	if mode != "api" {
		for i := 0; i < count; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				worker := relay.Worker{Store: store, Client: relay.HTTPClient(allowPrivate), MaxAttempts: 5, Poll: 500 * time.Millisecond, Lease: 30 * time.Second, AllowPrivate: allowPrivate}
				worker.Run(ctx)
			}()
		}
	}
	server := &http.Server{Addr: env("LISTEN_ADDR", ":8090"), Handler: (&relay.API{Store: store, Token: token, AllowPrivate: allowPrivate}).Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	if mode != "worker" {
		go func() {
			slog.Info("API listening", "address", server.Addr)
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("HTTP server failed", "error", err)
				stop()
			}
		}()
	}
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	server.Shutdown(shutdown)
	wg.Wait()
	slog.Info("shutdown complete")
}
