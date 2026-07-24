package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"meso/api/config"
	"meso/api/database"
	"meso/api/handlers"
	"meso/api/middleware"
	"meso/api/repository"
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
	migrateDB.Close()
	slog.Info("migrations applied")

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database pool", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("connected to database")

	var handler http.Handler = setupRoutes(pool)

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

func setupRoutes(pool *pgxpool.Pool) *http.ServeMux {
	movementRepo := repository.NewMovementRepo(pool)
	movementH := handlers.NewMovementHandler(movementRepo)
	workoutRepo := repository.NewWorkoutRepo(pool)
	workoutH := handlers.NewWorkoutHandler(workoutRepo)
	sessionRepo := repository.NewSessionRepo(pool)
	sessionH := handlers.NewSessionHandler(sessionRepo)
	measurementRepo := repository.NewMeasurementRepo(pool)
	measurementH := handlers.NewMeasurementHandler(measurementRepo)
	statsH := handlers.NewStatsHandler(repository.NewStatsRepo(pool))
	logH := handlers.NewLogHandler(repository.NewLogRepo(pool))

	mux := http.NewServeMux()

	// Health — unauthenticated (auth middleware exempts /health).
	// Returns 503 on DB failure so docker/k8s healthchecks detect outages.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := pool.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "reason": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /api/v1/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "meso API", "version": "0.1.0"})
	})

	// Movements — the unified exercise/stretch/pose library (Phase 1).
	mux.HandleFunc("GET /api/v1/movements", movementH.List)
	mux.HandleFunc("POST /api/v1/movements", movementH.Create)
	mux.HandleFunc("GET /api/v1/movements/{id}", movementH.Get)
	mux.HandleFunc("PUT /api/v1/movements/{id}", movementH.Update)
	mux.HandleFunc("DELETE /api/v1/movements/{id}", movementH.Delete)

	// Movement relationships — the self-ref alternate/antagonist join (Phase 2).
	mux.HandleFunc("POST /api/v1/movements/{id}/related", movementH.AddRelated)
	mux.HandleFunc("DELETE /api/v1/movements/{id}/related/{rid}", movementH.RemoveRelated)

	// Muscle lookup — the tagging vocabulary the UI offers.
	mux.HandleFunc("GET /api/v1/muscles", movementH.ListMuscles)

	// Workouts — ordered, themed compositions of movements (Phase 2).
	mux.HandleFunc("GET /api/v1/workouts", workoutH.List)
	mux.HandleFunc("POST /api/v1/workouts", workoutH.Create)
	mux.HandleFunc("GET /api/v1/workouts/{id}", workoutH.Get)
	mux.HandleFunc("PUT /api/v1/workouts/{id}", workoutH.Update)
	mux.HandleFunc("DELETE /api/v1/workouts/{id}", workoutH.Delete)

	// Workout composition — the ordered movement list. PATCH without an entry id
	// reorders; PATCH with one edits/swaps that entry.
	mux.HandleFunc("POST /api/v1/workouts/{id}/movements", workoutH.AddMovement)
	mux.HandleFunc("PATCH /api/v1/workouts/{id}/movements", workoutH.ReorderMovements)
	mux.HandleFunc("PATCH /api/v1/workouts/{id}/movements/{entryID}", workoutH.UpdateMovement)
	mux.HandleFunc("DELETE /api/v1/workouts/{id}/movements/{entryID}", workoutH.RemoveMovement)

	// Sessions — workouts performed on a date, the logged instance (Phase 3).
	// POST with a workout_id copies that template's movements in; the sub-resource
	// PATCH checks off a set / records actuals / swaps an entry mid-session.
	mux.HandleFunc("GET /api/v1/sessions", sessionH.List)
	mux.HandleFunc("POST /api/v1/sessions", sessionH.Create)
	mux.HandleFunc("GET /api/v1/sessions/{id}", sessionH.Get)
	mux.HandleFunc("PUT /api/v1/sessions/{id}", sessionH.Update)
	mux.HandleFunc("DELETE /api/v1/sessions/{id}", sessionH.Delete)
	mux.HandleFunc("PATCH /api/v1/sessions/{id}/movements/{entryID}", sessionH.UpdateMovement)

	// Metrics + measurements — the tracked stats time series (Phase 4). Metrics are
	// the definition vocabulary; measurements are dated readings against them; the
	// {name}/trend sub-resource returns one metric's series plus its summary numbers.
	mux.HandleFunc("GET /api/v1/metrics", measurementH.ListMetrics)
	mux.HandleFunc("POST /api/v1/metrics", measurementH.DefineMetric)
	mux.HandleFunc("GET /api/v1/metrics/{name}/trend", measurementH.Trend)
	mux.HandleFunc("GET /api/v1/measurements", measurementH.List)
	mux.HandleFunc("POST /api/v1/measurements", measurementH.Record)
	mux.HandleFunc("GET /api/v1/measurements/{id}", measurementH.Get)
	mux.HandleFunc("PUT /api/v1/measurements/{id}", measurementH.Update)
	mux.HandleFunc("DELETE /api/v1/measurements/{id}", measurementH.Delete)

	// Stats — the aggregated stats-page payload in one read (Phase 4).
	mux.HandleFunc("GET /api/v1/stats", statsH.Summary)

	// Fitness log — the dated training journal, `meso review`'s substrate (Phase 5).
	mux.HandleFunc("GET /api/v1/log", logH.List)
	mux.HandleFunc("POST /api/v1/log", logH.Create)
	mux.HandleFunc("GET /api/v1/log/{id}", logH.Get)
	mux.HandleFunc("PUT /api/v1/log/{id}", logH.Update)
	mux.HandleFunc("DELETE /api/v1/log/{id}", logH.Delete)

	return mux
}
