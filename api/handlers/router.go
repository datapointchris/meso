package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"meso/api/auth"
	"meso/api/middleware"
	"meso/api/repository"
)

// AdminGroup is the identity provider's group that operates the app.
const AdminGroup = "admin"

func admin(h http.HandlerFunc) http.HandlerFunc {
	return middleware.RequireGroup(AdminGroup, h).ServeHTTP
}

// route is one method on one path and the handler answering it.
type route struct {
	method, path string
	handle       http.HandlerFunc
}

// NewRouter wires repositories → handlers → routes and returns the API mux. It is
// the single source of truth for the route table: main() serves it in production and
// the handler tests build it against a throwaway Postgres, so the two can never drift
// — a route added here is exercised by both.
//
// A path under /api/v1/ that no route matches is a 404, and a method no route on
// its path takes is a 405 naming the methods it does take.
//
// metadata serves the RFC 9728 document. main passes nil while PUBLIC_URL is
// unset, because only the deployment knows the address it is reached at, and
// the route is then absent.
func NewRouter(pool *pgxpool.Pool, metadata http.Handler) *http.ServeMux {
	movementRepo := repository.NewMovementRepo(pool)
	movementH := NewMovementHandler(movementRepo)
	workoutRepo := repository.NewWorkoutRepo(pool)
	workoutH := NewWorkoutHandler(workoutRepo)
	sessionRepo := repository.NewSessionRepo(pool)
	sessionH := NewSessionHandler(sessionRepo, workoutRepo)
	measurementRepo := repository.NewMeasurementRepo(pool)
	measurementH := NewMeasurementHandler(measurementRepo)
	statsH := NewStatsHandler(repository.NewStatsRepo(pool))
	logRepo := repository.NewLogRepo(pool)
	logH := NewLogHandler(logRepo)
	cycleRepo := repository.NewCycleRepo(pool)
	cycleH := NewCycleHandler(cycleRepo)
	feedbackH := NewFeedbackHandler(repository.NewFeedbackRepo(pool))
	reviewH := NewReviewHandler(
		repository.NewReviewRepo(sessionRepo, measurementRepo, logRepo, cycleRepo))

	mux := http.NewServeMux()

	// Unauthenticated (auth middleware exempts the path): a CLI reads it before
	// it holds a token.
	if metadata != nil {
		mux.Handle("GET "+auth.ProtectedResourcePath, metadata)
	}

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

	routes := []route{
		{http.MethodGet, "/api/v1/{$}", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"message": "meso API"})
		}},

		// Movements — the unified exercise/stretch/pose library (Phase 1).
		{http.MethodGet, "/api/v1/movements", movementH.List},
		{http.MethodPost, "/api/v1/movements", movementH.Create},
		{http.MethodGet, "/api/v1/movements/{id}", movementH.Get},
		{http.MethodPut, "/api/v1/movements/{id}", movementH.Update},
		{http.MethodDelete, "/api/v1/movements/{id}", movementH.Delete},

		// Movement relationships — the self-ref alternate/antagonist join (Phase 2).
		{http.MethodPost, "/api/v1/movements/{id}/related", movementH.AddRelated},
		{http.MethodDelete, "/api/v1/movements/{id}/related/{rid}", movementH.RemoveRelated},

		// Muscle lookup — the tagging vocabulary the UI offers.
		{http.MethodGet, "/api/v1/muscles", movementH.ListMuscles},

		// Workouts — ordered, themed compositions of movements (Phase 2).
		{http.MethodGet, "/api/v1/workouts", workoutH.List},
		{http.MethodPost, "/api/v1/workouts", workoutH.Create},
		{http.MethodGet, "/api/v1/workouts/{id}", workoutH.Get},
		{http.MethodPut, "/api/v1/workouts/{id}", workoutH.Update},
		{http.MethodDelete, "/api/v1/workouts/{id}", workoutH.Delete},

		// Workout composition — the ordered movement list. PATCH without an entry id
		// reorders; PATCH with one edits/swaps that entry.
		{http.MethodPost, "/api/v1/workouts/{id}/movements", workoutH.AddMovement},
		{http.MethodPatch, "/api/v1/workouts/{id}/movements", workoutH.ReorderMovements},
		{http.MethodPatch, "/api/v1/workouts/{id}/movements/{entryID}", workoutH.UpdateMovement},
		{http.MethodDelete, "/api/v1/workouts/{id}/movements/{entryID}", workoutH.RemoveMovement},

		// Sessions — workouts performed on a date, the logged instance (Phase 3).
		// POST with a workout_id copies that template's movements in as targets; without one
		// the session starts empty and grows through the sub-resource POST as the workout is
		// performed. The entry PATCH overrides done / edits this session's target / swaps an
		// entry mid-session; POST .../workout promotes what was performed free-form into a
		// reusable template; POST .../finish ends it.
		{http.MethodGet, "/api/v1/sessions", sessionH.List},
		{http.MethodPost, "/api/v1/sessions", sessionH.Create},
		{http.MethodGet, "/api/v1/sessions/{id}", sessionH.Get},
		{http.MethodPut, "/api/v1/sessions/{id}", sessionH.Update},
		{http.MethodDelete, "/api/v1/sessions/{id}", sessionH.Delete},
		{http.MethodPost, "/api/v1/sessions/{id}/finish", sessionH.Finish},
		{http.MethodPost, "/api/v1/sessions/{id}/movements", sessionH.AddMovement},
		{http.MethodPatch, "/api/v1/sessions/{id}/movements/{entryID}", sessionH.UpdateMovement},
		{http.MethodDelete, "/api/v1/sessions/{id}/movements/{entryID}", sessionH.RemoveMovement},
		{http.MethodPost, "/api/v1/sessions/{id}/workout", sessionH.Promote},

		// Sets — what was actually performed, one row per set. The POST is the most-tapped
		// write in the app: an empty body repeats the last set, so logging one costs a tap.
		{http.MethodPost, "/api/v1/sessions/{id}/movements/{entryID}/sets", sessionH.AddSet},
		{http.MethodPatch, "/api/v1/sessions/{id}/movements/{entryID}/sets/{setID}", sessionH.UpdateSet},
		{http.MethodDelete, "/api/v1/sessions/{id}/movements/{entryID}/sets/{setID}", sessionH.RemoveSet},

		// Metrics + measurements — the tracked stats time series (Phase 4). Metrics are
		// the definition vocabulary; measurements are dated readings against them; the
		// {name}/trend sub-resource returns one metric's series plus its summary numbers.
		{http.MethodGet, "/api/v1/metrics", measurementH.ListMetrics},
		{http.MethodPost, "/api/v1/metrics", measurementH.DefineMetric},
		{http.MethodGet, "/api/v1/metrics/{name}", measurementH.GetMetric},
		{http.MethodPut, "/api/v1/metrics/{name}", measurementH.UpdateMetric},
		{http.MethodDelete, "/api/v1/metrics/{name}", measurementH.DeleteMetric},
		{http.MethodGet, "/api/v1/metrics/{name}/trend", measurementH.Trend},
		{http.MethodGet, "/api/v1/measurements", measurementH.List},
		{http.MethodPost, "/api/v1/measurements", measurementH.Record},
		{http.MethodGet, "/api/v1/measurements/{id}", measurementH.Get},
		{http.MethodPut, "/api/v1/measurements/{id}", measurementH.Update},
		{http.MethodDelete, "/api/v1/measurements/{id}", measurementH.Delete},

		// Stats — the aggregated stats-page payload in one read (Phase 4).
		{http.MethodGet, "/api/v1/stats", statsH.Summary},

		// Fitness log — the dated training journal, `meso review`'s substrate (Phase 5).
		{http.MethodGet, "/api/v1/log", logH.List},
		{http.MethodPost, "/api/v1/log", logH.Create},
		{http.MethodGet, "/api/v1/log/{id}", logH.Get},
		{http.MethodPut, "/api/v1/log/{id}", logH.Update},
		{http.MethodDelete, "/api/v1/log/{id}", logH.Delete},

		// Cycles — mesocycles: ordered sequences of workouts toward a goal (Phase 6).
		// The sub-resource PATCH without an entry id reorders; with one it edits/swaps.
		{http.MethodGet, "/api/v1/cycles", cycleH.List},
		{http.MethodPost, "/api/v1/cycles", cycleH.Create},
		{http.MethodGet, "/api/v1/cycles/{id}", cycleH.Get},
		{http.MethodPut, "/api/v1/cycles/{id}", cycleH.Update},
		{http.MethodDelete, "/api/v1/cycles/{id}", cycleH.Delete},
		{http.MethodPost, "/api/v1/cycles/{id}/workouts", cycleH.AddWorkout},
		{http.MethodPatch, "/api/v1/cycles/{id}/workouts", cycleH.ReorderWorkouts},
		{http.MethodPatch, "/api/v1/cycles/{id}/workouts/{entryID}", cycleH.UpdateWorkout},
		{http.MethodDelete, "/api/v1/cycles/{id}/workouts/{entryID}", cycleH.RemoveWorkout},

		// Feedback — in-app capture of papercuts and ideas about meso itself. The only
		// non-training resource here: the web app POSTs, and the reads/writes below serve
		// the CLI's `meso admin feedback` namespace. meso owns this data outright and
		// forwards it nowhere — whoever wants it reads it back out. Any signed-in user
		// captures, and only the admin group reads, edits or deletes.
		{http.MethodGet, "/api/v1/feedback", admin(feedbackH.List)},
		{http.MethodPost, "/api/v1/feedback", feedbackH.Create},
		{http.MethodGet, "/api/v1/feedback/{id}", admin(feedbackH.Get)},
		{http.MethodPut, "/api/v1/feedback/{id}", admin(feedbackH.Update)},
		{http.MethodDelete, "/api/v1/feedback/{id}", admin(feedbackH.Delete)},

		// Review — the capstone read: active cycles + recent sessions/measurements/log
		// in one payload, for Claude to draft the next cycle from real history (Phase 6).
		{http.MethodGet, "/api/v1/review", reviewH.Review},
	}

	allowed := make(map[string][]string)
	for _, rt := range routes {
		mux.HandleFunc(rt.method+" "+rt.path, rt.handle)
		allowed[rt.path] = append(allowed[rt.path], rt.method)
	}
	for path, methods := range allowed {
		mux.HandleFunc(path, methodNotAllowed(methods))
	}
	mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, r *http.Request) {
		writeNotFound(w, fmt.Sprintf("no route answers %s", r.URL.Path))
	})

	return mux
}

func methodNotAllowed(methods []string) http.HandlerFunc {
	allow := slices.Clone(methods)
	if slices.Contains(allow, http.MethodGet) {
		allow = append(allow, http.MethodHead)
	}
	slices.Sort(allow)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", strings.Join(allow, ", "))
		writeError(w, http.StatusMethodNotAllowed, fmt.Sprintf("%s takes %s, not %s", r.URL.Path, strings.Join(allow, ", "), r.Method))
	}
}
