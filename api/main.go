package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/datapointchris/gobrowserlogin"
	"github.com/datapointchris/gobrowserlogin/postgresstore"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"meso/api/auth"
	"meso/api/config"
	"meso/api/database"
	"meso/api/handlers"
	"meso/api/middleware"
)

func main() {
	cfg := config.Load()

	// Structured JSON logs to stdout, so a log collector reading container
	// stdout can query their fields as labels.
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

	var metadata http.Handler
	if cfg.PublicURL != "" {
		metadata = auth.ProtectedResourceMetadata(cfg.PublicURL, cfg.OIDCIssuer)
	} else {
		slog.Warn("PUBLIC_URL is unset, so no RFC 9728 metadata is served and `meso auth login <url>` cannot discover the issuer")
	}
	mux := handlers.NewRouter(pool, metadata)

	var handler http.Handler
	if cfg.Env == "development" {
		handler = middleware.DevCORS(middleware.DevGroups(mux, handlers.AdminGroup))
	} else {
		verifier, verifierErr := auth.NewVerifier(ctx, cfg.OIDCIssuer, cfg.CLIClientIDPrefix)
		if verifierErr != nil {
			slog.Error("oidc verifier", "err", verifierErr)
			os.Exit(1)
		}
		sessions, sessionsErr := browserSessions(ctx, cfg, pool)
		if sessionsErr != nil {
			slog.Error("browser login", "err", sessionsErr)
			os.Exit(1)
		}
		mux.HandleFunc("GET "+middleware.LoginPath, sessions.Login)
		mux.HandleFunc("GET "+middleware.CallbackPath, sessions.Callback)
		handler = middleware.RequireAuth(verifier, cfg.ServiceToken, sessions, slog.Default())(mux)
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

// browserSessions logs the SPA's browsers in as this API's own confidential
// client, and keeps each session in the API's database. It asks for groups,
// because the admin routes read them from the session.
func browserSessions(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool) (*gobrowserlogin.Sessions, error) {
	if cfg.PublicURL == "" || cfg.WebClientSecret == "" {
		return nil, errors.New("PUBLIC_URL and WEB_CLIENT_SECRET must both be set: the login's callback is under the one, and the API authenticates to the provider with the other")
	}
	store, err := postgresstore.New(ctx, stdlib.OpenDBFromPool(pool))
	if err != nil {
		return nil, err
	}
	return gobrowserlogin.New(ctx, gobrowserlogin.Config{
		Issuer:       cfg.OIDCIssuer,
		ClientID:     cfg.WebClientID,
		ClientSecret: cfg.WebClientSecret,
		RedirectURL:  strings.TrimSuffix(cfg.PublicURL, "/") + middleware.CallbackPath,
		Scopes:       []string{"openid", "profile", "groups"},
		Store:        store,
		HTTPClient:   auth.IdentifiedClient(),
		Logger:       slog.Default(),
	})
}
