package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"meso/api/repository"
)

// NewRouter wires repositories → handlers → routes and returns the API mux. It is
// the single source of truth for the route table: main() serves it in production and
// the handler tests build it against a throwaway Postgres, so the two can never drift
// — a route added here is exercised by both.
func NewRouter(pool *pgxpool.Pool) *http.ServeMux {
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
	// POST with a workout_id copies that template's movements in; without one the
	// session starts empty and grows through the sub-resource POST as the workout is
	// performed. The sub-resource PATCH checks off a set / records actuals / swaps an
	// entry mid-session; POST .../workout promotes what was performed ad-hoc into a
	// reusable template.
	mux.HandleFunc("GET /api/v1/sessions", sessionH.List)
	mux.HandleFunc("POST /api/v1/sessions", sessionH.Create)
	mux.HandleFunc("GET /api/v1/sessions/{id}", sessionH.Get)
	mux.HandleFunc("PUT /api/v1/sessions/{id}", sessionH.Update)
	mux.HandleFunc("DELETE /api/v1/sessions/{id}", sessionH.Delete)
	mux.HandleFunc("POST /api/v1/sessions/{id}/movements", sessionH.AddMovement)
	mux.HandleFunc("PATCH /api/v1/sessions/{id}/movements/{entryID}", sessionH.UpdateMovement)
	mux.HandleFunc("DELETE /api/v1/sessions/{id}/movements/{entryID}", sessionH.RemoveMovement)
	mux.HandleFunc("POST /api/v1/sessions/{id}/workout", sessionH.Promote)

	// Metrics + measurements — the tracked stats time series (Phase 4). Metrics are
	// the definition vocabulary; measurements are dated readings against them; the
	// {name}/trend sub-resource returns one metric's series plus its summary numbers.
	mux.HandleFunc("GET /api/v1/metrics", measurementH.ListMetrics)
	mux.HandleFunc("POST /api/v1/metrics", measurementH.DefineMetric)
	mux.HandleFunc("GET /api/v1/metrics/{name}", measurementH.GetMetric)
	mux.HandleFunc("PUT /api/v1/metrics/{name}", measurementH.UpdateMetric)
	mux.HandleFunc("DELETE /api/v1/metrics/{name}", measurementH.DeleteMetric)
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

	// Cycles — mesocycles: ordered sequences of workouts toward a goal (Phase 6).
	// The sub-resource PATCH without an entry id reorders; with one it edits/swaps.
	mux.HandleFunc("GET /api/v1/cycles", cycleH.List)
	mux.HandleFunc("POST /api/v1/cycles", cycleH.Create)
	mux.HandleFunc("GET /api/v1/cycles/{id}", cycleH.Get)
	mux.HandleFunc("PUT /api/v1/cycles/{id}", cycleH.Update)
	mux.HandleFunc("DELETE /api/v1/cycles/{id}", cycleH.Delete)
	mux.HandleFunc("POST /api/v1/cycles/{id}/workouts", cycleH.AddWorkout)
	mux.HandleFunc("PATCH /api/v1/cycles/{id}/workouts", cycleH.ReorderWorkouts)
	mux.HandleFunc("PATCH /api/v1/cycles/{id}/workouts/{entryID}", cycleH.UpdateWorkout)
	mux.HandleFunc("DELETE /api/v1/cycles/{id}/workouts/{entryID}", cycleH.RemoveWorkout)

	// Feedback — in-app capture of papercuts and ideas about meso itself. The only
	// non-training resource here: the web app POSTs, and the reads/writes below serve
	// the CLI's `meso admin feedback` namespace. meso owns this data outright and
	// forwards it nowhere — whoever wants it reads it back out.
	mux.HandleFunc("GET /api/v1/feedback", feedbackH.List)
	mux.HandleFunc("POST /api/v1/feedback", feedbackH.Create)
	mux.HandleFunc("GET /api/v1/feedback/{id}", feedbackH.Get)
	mux.HandleFunc("PUT /api/v1/feedback/{id}", feedbackH.Update)
	mux.HandleFunc("DELETE /api/v1/feedback/{id}", feedbackH.Delete)

	// Review — the capstone read: active cycles + recent sessions/measurements/log
	// in one payload, for Claude to draft the next cycle from real history (Phase 6).
	mux.HandleFunc("GET /api/v1/review", reviewH.Review)

	return mux
}
