// Command seed is a one-shot bootstrap that loads catalog/lookup data into the
// DB. It UPSERTs every row (ON CONFLICT DO NOTHING), so it's idempotent and safe
// to re-run. This is NOT a migration — it carries data, which must never ride
// the goose ledger. Run once per environment after the schema is in place (the
// API applies migrations on startup):
//
//	DATABASE_URL=postgres://... go run ./cmd/seed
//
// As later phases add richer lookups (metric_definitions, muscles) and a
// baseline movement/workout catalog, they extend this command.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"meso/api/config"
	"meso/api/database"
	"meso/api/models"
	"meso/api/repository"
)

// lookupSeed is a table and the categorical values it should contain.
type lookupSeed struct {
	table  string
	values []string
}

var lookups = []lookupSeed{
	{"movement_kinds", []string{"exercise", "stretch", "yoga_pose"}},
	{"relationship_kinds", []string{"alternate", "antagonist", "progression", "regression", "see_also"}},
	{"cycle_statuses", []string{"planned", "active", "paused", "complete"}},
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.Load()
	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database pool", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	total, err := seedLookups(ctx, pool, lookups)
	if err != nil {
		slog.Error("seeding lookups", "seeded", total, "err", err)
		os.Exit(1)
	}
	slog.Info("seeded lookups", "count", total)

	muscleCount, err := seedMuscles(ctx, pool, muscleCatalog)
	if err != nil {
		slog.Error("seeding muscles", "seeded", muscleCount, "err", err)
		os.Exit(1)
	}
	slog.Info("seeded muscles", "count", muscleCount)

	metricCount, err := seedMetrics(ctx, pool, metricCatalog)
	if err != nil {
		slog.Error("seeding metrics", "seeded", metricCount, "err", err)
		os.Exit(1)
	}
	slog.Info("seeded metrics", "count", metricCount)

	// Movements go through the repository (not raw SQL) so the baseline catalog
	// is written by the exact same code path the API and CLI use — muscle tags in
	// a transaction, upsert-by-name idempotency. Muscles must exist first (FK).
	movementRepo := repository.NewMovementRepo(pool)
	movementCount := 0
	for _, m := range baselineMovements {
		if _, err := movementRepo.Upsert(ctx, m); err != nil {
			slog.Error("seeding movement", "name", m.Name, "err", err)
			os.Exit(1)
		}
		movementCount++
	}
	slog.Info("seeded movements", "count", movementCount)
}

// seedMuscles upserts the muscle lookup (name + region), returning the count
// written. ON CONFLICT keeps re-runs no-ops while refreshing region if it changed.
func seedMuscles(ctx context.Context, pool *pgxpool.Pool, muscles []models.Muscle) (int, error) {
	count := 0
	for _, m := range muscles {
		_, err := pool.Exec(ctx,
			`INSERT INTO muscles (name, region) VALUES ($1, $2)
			 ON CONFLICT (name) DO UPDATE SET region = EXCLUDED.region`,
			m.Name, m.Region)
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// seedMetrics upserts the metric-definition vocabulary (a lookup), returning the
// count written. ON CONFLICT refreshes the metadata so a re-run converges each
// definition to the catalog while leaving its recorded measurements untouched.
func seedMetrics(ctx context.Context, pool *pgxpool.Pool, metrics []models.MetricDefinitionCreate) (int, error) {
	count := 0
	for _, m := range metrics {
		_, err := pool.Exec(ctx,
			`INSERT INTO metric_definitions (name, unit, direction, category) VALUES ($1, $2, $3, $4)
			 ON CONFLICT (name) DO UPDATE SET
			     unit = EXCLUDED.unit, direction = EXCLUDED.direction, category = EXCLUDED.category`,
			m.Name, m.Unit, m.Direction, m.Category)
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// seedLookups upserts every value across every lookup table, returning the count
// of rows written. Uses ON CONFLICT DO NOTHING so re-runs are no-ops.
func seedLookups(ctx context.Context, pool *pgxpool.Pool, seeds []lookupSeed) (int, error) {
	total := 0
	for _, seed := range seeds {
		for _, value := range seed.values {
			// Table name is a trusted constant from this file, never user input.
			query := "INSERT INTO " + seed.table + " (name) VALUES ($1) ON CONFLICT (name) DO NOTHING"
			if _, err := pool.Exec(ctx, query, value); err != nil {
				return total, err
			}
			total++
		}
	}
	return total, nil
}
