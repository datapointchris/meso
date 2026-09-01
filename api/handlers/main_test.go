package handlers_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"meso/api/database"
	"meso/api/handlers"
)

// sharedDatabaseURL points at the Postgres the whole package's tests run against.
// It's provisioned once in TestMain — a throwaway testcontainers Postgres by
// default, or TEST_DATABASE_URL when set (an escape hatch for a faster local
// instance). Migrations plus the lookup vocabulary are applied once up front.
var sharedDatabaseURL string

func TestMain(m *testing.M) {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		sharedDatabaseURL = url
		mustPrepare(url)
		os.Exit(m.Run())
	}

	ctx := context.Background()
	container, url, err := startPostgresContainer(ctx)
	if err != nil {
		log.Fatalf("starting postgres container: %v", err)
	}
	sharedDatabaseURL = url
	mustPrepare(url)

	code := m.Run()

	// Terminate explicitly — os.Exit skips deferred calls.
	if err := container.Terminate(ctx); err != nil {
		log.Printf("terminating postgres container: %v", err)
	}
	os.Exit(code)
}

func mustPrepare(url string) {
	if err := runMigrations(url); err != nil {
		log.Fatalf("migrating test database: %v", err)
	}
	if err := seedTestLookups(url); err != nil {
		log.Fatalf("seeding test lookups: %v", err)
	}
}

func startPostgresContainer(ctx context.Context) (*postgres.PostgresContainer, string, error) {
	container, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("meso_test"),
		postgres.WithUsername("meso"),
		postgres.WithPassword("meso"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return nil, "", err
	}
	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, "", err
	}
	return container, url, nil
}

// runMigrations applies all goose migrations. goose needs a database/sql handle
// (its contract), separate from the pgxpool the app uses.
func runMigrations(url string) error {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("opening migration handle: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("setting goose dialect: %w", err)
	}
	if err := goose.Up(db, "../migrations"); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	return nil
}

// seedTestLookups loads the categorical vocabulary the handler tests need. It
// mirrors cmd/seed but stays local so the tests never depend on the seed binary.
// Idempotent, so re-running against a provided TEST_DATABASE_URL is safe.
func seedTestLookups(url string) error {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	kinds := []string{"exercise", "stretch", "yoga_pose"}
	for _, k := range kinds {
		if _, err := db.Exec(`INSERT INTO movement_kinds (name) VALUES ($1) ON CONFLICT DO NOTHING`, k); err != nil {
			return err
		}
	}
	relationshipKinds := []string{"alternate", "antagonist", "progression", "regression", "see_also"}
	for _, k := range relationshipKinds {
		if _, err := db.Exec(`INSERT INTO relationship_kinds (name) VALUES ($1) ON CONFLICT DO NOTHING`, k); err != nil {
			return err
		}
	}
	cycleStatuses := []string{"planned", "active", "paused", "completed"}
	for _, s := range cycleStatuses {
		if _, err := db.Exec(`INSERT INTO cycle_statuses (name) VALUES ($1) ON CONFLICT DO NOTHING`, s); err != nil {
			return err
		}
	}
	setKinds := []string{"working", "warmup", "amrap", "drop", "failure"}
	for _, k := range setKinds {
		if _, err := db.Exec(`INSERT INTO set_kinds (name) VALUES ($1) ON CONFLICT DO NOTHING`, k); err != nil {
			return err
		}
	}
	loadModes := []string{"weighted", "bodyweight", "timed", "assisted"}
	for _, m := range loadModes {
		if _, err := db.Exec(`INSERT INTO load_modes (name) VALUES ($1) ON CONFLICT DO NOTHING`, m); err != nil {
			return err
		}
	}
	muscles := map[string]string{
		"chest": "anterior", "triceps": "arms", "quads": "anterior",
		"hamstrings": "posterior", "glutes": "posterior", "shoulders": "shoulders",
	}
	for name, region := range muscles {
		if _, err := db.Exec(`INSERT INTO muscles (name, region) VALUES ($1, $2) ON CONFLICT DO NOTHING`, name, region); err != nil {
			return err
		}
	}
	return nil
}

// setupTestDB returns a pool against the shared test database and registers
// cleanup of the mutable table (movements; movement_muscles cascades), so each
// test starts from the same lookup-only baseline.
func setupTestDB(t *testing.T) *database.TestPool {
	t.Helper()
	ctx := context.Background()

	pool, err := database.NewPool(ctx, sharedDatabaseURL)
	require.NoError(t, err)

	t.Cleanup(func() {
		// Order matters: both workout_movements and session_movements reference
		// movements with ON DELETE RESTRICT, so their parents must be cleared first;
		// measurements reference metric_definitions with RESTRICT likewise. cycles
		// must go before workouts (cycle_workouts RESTRICTs the workout) and before
		// metric_definitions (cycles.target_metric references it).
		pool.Exec(ctx, `DELETE FROM fitness_log_entries`) //nolint:errcheck // standalone, no FKs
		pool.Exec(ctx, `DELETE FROM cycles`)              //nolint:errcheck // cascades to cycle_workouts
		pool.Exec(ctx, `DELETE FROM measurements`)        //nolint:errcheck
		pool.Exec(ctx, `DELETE FROM metric_definitions`)  //nolint:errcheck
		pool.Exec(ctx, `DELETE FROM workout_sessions`)    //nolint:errcheck // cascades to session_movements, and those to session_sets
		pool.Exec(ctx, `DELETE FROM workouts`)            //nolint:errcheck // cascades to workout_movements
		pool.Exec(ctx, `DELETE FROM movements`)           //nolint:errcheck // cascades to movement_muscles + movement_relationships
		pool.Close()
	})

	return &database.TestPool{Pool: pool}
}

// buildTestMux builds the real production router against the test's throwaway
// Postgres. It delegates to handlers.NewRouter so the tests exercise the exact route
// table main() serves — the two can't drift.
func buildTestMux(pool *database.TestPool) *http.ServeMux {
	return handlers.NewRouter(pool.Pool)
}

func postJSON(t *testing.T, mux *http.ServeMux, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func putJSON(t *testing.T, mux *http.ServeMux, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func patchJSON(t *testing.T, mux *http.ServeMux, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func getJSON(t *testing.T, mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func deleteReq(t *testing.T, mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}
