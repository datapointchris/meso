// Command seed is a one-shot bootstrap that loads the FK-backbone lookups a blank
// DB needs before it can accept any write. It UPSERTs every row (ON CONFLICT DO
// NOTHING / DO UPDATE on the lookups only), so it's idempotent and safe to re-run
// on every deploy against a live, populated DB — it only ever ensures the backbone
// exists, never touching content. This is NOT a migration: it carries data, which
// must never ride the goose ledger. The API applies migrations on startup, then
// this runs:
//
//	DATABASE_URL=postgres://... go run ./cmd/seed
//
// Scope is deliberately minimal — only the categoricals with no CLI verb behind
// them: the enum lookups (movement_kinds, relationship_kinds, cycle_statuses) and
// the muscle lookup. All actual content — metrics, movements, workouts, cycles,
// measurements, the journal — is loaded through the `meso` CLI, so it stays out of
// the seed and out of migrations.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"meso/api/config"
	"meso/api/database"
	"meso/api/models"
)

// lookupSeed is a table and the categorical values it should contain.
type lookupSeed struct {
	table  string
	values []string
}

var lookups = []lookupSeed{
	{"movement_kinds", []string{"exercise", "stretch", "yoga_pose"}},
	{"relationship_kinds", []string{"alternate", "antagonist", "progression", "regression", "see_also"}},
	{"cycle_statuses", []string{"planned", "active", "paused", "completed"}},
	{"set_kinds", []string{"working", "warmup", "amrap", "drop", "failure"}},
	{"load_modes", []string{"weighted", "bodyweight", "timed", "assisted"}},
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if err := run(); err != nil {
		slog.Error("seed", "err", err)
		os.Exit(1)
	}
}

// run holds the body so os.Exit happens in main, once run has returned and its
// defers have unwound. Calling os.Exit beside `defer pool.Close()` skipped the
// close on both seeding error paths, the ones that most need it.
func run() error {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database pool: %w", err)
	}
	defer pool.Close()

	total, err := seedLookups(ctx, pool, lookups)
	if err != nil {
		return fmt.Errorf("seeding lookups after %d: %w", total, err)
	}
	slog.Info("seeded lookups", "count", total)

	muscleCount, err := seedMuscles(ctx, pool, muscleCatalog)
	if err != nil {
		return fmt.Errorf("seeding muscles after %d: %w", muscleCount, err)
	}
	slog.Info("seeded muscles", "count", muscleCount)
	return nil
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
