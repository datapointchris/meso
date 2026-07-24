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
