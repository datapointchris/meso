package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"meso/api/config"
	"meso/api/database"
	"meso/api/handlers"
	"meso/api/middleware"
)

func main() {
	cfg := config.Load()

	// Structured JSON logs to stdout — Promtail on docker_hosts scrapes
	// container stdout into Loki, where JSON fields become queryable labels.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ctx := context.Background()

	// Migrations need a database/sql driver, not pgxpool — goose's contract.
	migrateDB, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		slog.Error("open db for migration", "err", err)
		os.Exit(1)
	}
	if err = goose.SetDialect("postgres"); err != nil {
		slog.Error("goose dialect", "err", err)
		os.Exit(1)
	}
	if err = goose.Up(migrateDB, "migrations"); err != nil {
		slog.Error("migrations", "err", err)
		os.Exit(1)
	}
	_ = migrateDB.Close()
	slog.Info("migrations applied")

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database pool", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("connected to database")

	var handler http.Handler = handlers.NewRouter(pool)

	if cfg.Env == "development" {
		handler = middleware.DevCORS(handler)
	} else {
		handler = middleware.RequireAuthelia(handler)
	}

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("api listening", "port", cfg.Port, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutting down", "signal", sig.String())

	shutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("shutdown", "err", err)
	} else {
		slog.Info("stopped gracefully")
	}
}
