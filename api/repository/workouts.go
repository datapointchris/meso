package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"meso/api/models"
)

type WorkoutRepo struct {
	pool *pgxpool.Pool
}

func NewWorkoutRepo(pool *pgxpool.Pool) *WorkoutRepo {
	return &WorkoutRepo{pool: pool}
}

// workoutCols is the ordered column list shared by every workout SELECT so the
// scan stays in lockstep with the query.
const workoutCols = `id, name, theme, tags, notes, favorite, estimated_minutes, created_at, updated_at`

func scanWorkout(row pgx.Row) (models.Workout, error) {
	var w models.Workout
	err := row.Scan(
		&w.ID, &w.Name, &w.Theme, &w.Tags, &w.Notes, &w.Favorite,
		&w.EstimatedMinutes, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		return models.Workout{}, err
	}
	if w.Tags == nil {
		w.Tags = []string{}
	}
	w.Movements = []models.WorkoutMovement{}
	return w, nil
}

// List returns workouts matching the filter, ordered by name. Filtering is done in
// SQL so the CLI and web share one definition of each filter. The ordered movement
// lists are attached in a single follow-up query to avoid an N+1.
func (r *WorkoutRepo) List(ctx context.Context, f models.WorkoutFilter) ([]models.Workout, error) {
	var (
		where []string
		args  []any
	)
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}

	if f.Theme != "" {
		add("theme = $%d", f.Theme)
	}
	if f.Favorite != nil {
		add("favorite = $%d", *f.Favorite)
	}
	if f.Tag != "" {
		add("$%d = ANY(tags)", f.Tag)
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		where = append(where, fmt.Sprintf(
			"(name ILIKE $%d OR coalesce(theme, '') ILIKE $%d OR array_to_string(tags, ' ') ILIKE $%d)",
			len(args), len(args), len(args)))
	}

	query := `SELECT ` + workoutCols + ` FROM workouts`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY name"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing workouts: %w", err)
	}
	defer rows.Close()

	workouts := []models.Workout{}
	byID := map[int64]int{}
	for rows.Next() {
		w, err := scanWorkout(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning workout: %w", err)
		}
		byID[w.ID] = len(workouts)
		workouts = append(workouts, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(workouts) > 0 {
		if err := r.attachMovements(ctx, workouts, byID); err != nil {
			return nil, err
		}
	}
	return workouts, nil
}

// attachMovements loads every workout_movement for the given workouts in one query,
// joined with the movement's name/kind for render, and hangs each onto its workout
// ordered by position.
func (r *WorkoutRepo) attachMovements(ctx context.Context, workouts []models.Workout, byID map[int64]int) error {
	ids := make([]int64, 0, len(workouts))
	for _, w := range workouts {
		ids = append(ids, w.ID)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT wm.workout_id, wm.id, wm.movement_id, m.name, m.movement_kind, wm.position,
			wm.sets, wm.reps, wm.load, wm.rest_seconds, wm.superset_group, wm.notes
		FROM workout_movements wm
		JOIN movements m ON m.id = wm.movement_id
		WHERE wm.workout_id = ANY($1)
		ORDER BY wm.workout_id, wm.position`, ids)
	if err != nil {
		return fmt.Errorf("loading workout movements: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var workoutID int64
		var wm models.WorkoutMovement
		if err := rows.Scan(&workoutID, &wm.ID, &wm.MovementID, &wm.MovementName, &wm.MovementKind,
			&wm.Position, &wm.Sets, &wm.Reps, &wm.Load, &wm.RestSeconds, &wm.SupersetGroup, &wm.Notes); err != nil {
			return fmt.Errorf("scanning workout movement: %w", err)
		}
		idx := byID[workoutID]
		workouts[idx].Movements = append(workouts[idx].Movements, wm)
	}
	return rows.Err()
}

func (r *WorkoutRepo) GetByID(ctx context.Context, id int64) (models.Workout, error) {
	w, err := scanWorkout(r.pool.QueryRow(ctx, `SELECT `+workoutCols+` FROM workouts WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Workout{}, fmt.Errorf("workout %d: %w", id, ErrNotFound)
		}
		return models.Workout{}, fmt.Errorf("fetching workout: %w", err)
	}
	list := []models.Workout{w}
	if err := r.attachMovements(ctx, list, map[int64]int{w.ID: 0}); err != nil {
		return models.Workout{}, err
	}
	return list[0], nil
}

func (r *WorkoutRepo) Create(ctx context.Context, in models.WorkoutCreate) (models.Workout, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return models.Workout{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after a successful commit is a no-op

	id, err := insertWorkout(ctx, tx, in)
	if err != nil {
		return models.Workout{}, err
	}
	if err := insertMovementsInOrder(ctx, tx, id, in.Movements); err != nil {
		return models.Workout{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return models.Workout{}, fmt.Errorf("commit: %w", err)
	}
	return r.GetByID(ctx, id)
}

func (r *WorkoutRepo) Update(ctx context.Context, id int64, in models.WorkoutUpdate) (models.Workout, error) {
	current, err := r.GetByID(ctx, id)
	if err != nil {
		return models.Workout{}, err
	}

	name := valueOr(in.Name, current.Name)
	notes := valueOr(in.Notes, current.Notes)
	favorite := valueOr(in.Favorite, current.Favorite)
	theme := pickPtr(in.Theme, current.Theme)
	estimated := pickPtr(in.EstimatedMinutes, current.EstimatedMinutes)
	tags := current.Tags
	if in.Tags != nil {
		tags = *in.Tags
	}

	_, err = r.pool.Exec(ctx, `
		UPDATE workouts SET
			name = $1, theme = $2, tags = $3, notes = $4, favorite = $5,
			estimated_minutes = $6, updated_at = now()
		WHERE id = $7`,
		name, theme, normalizeArray(tags), notes, favorite, estimated, id)
	if err != nil {
		return models.Workout{}, mapWriteError("updating workout", err)
	}
	return r.GetByID(ctx, id)
}

// Upsert inserts a workout or overwrites an existing one matched by name, replacing
// its movement list wholesale — the idempotent write path for cmd/seed and the CLI
// import pass.
func (r *WorkoutRepo) Upsert(ctx context.Context, in models.WorkoutCreate) (models.Workout, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return models.Workout{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO workouts (name, theme, tags, notes, favorite, estimated_minutes)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (name) DO UPDATE SET
			theme = EXCLUDED.theme, tags = EXCLUDED.tags, notes = EXCLUDED.notes,
			favorite = EXCLUDED.favorite, estimated_minutes = EXCLUDED.estimated_minutes,
			updated_at = now()
		RETURNING id`,
		in.Name, in.Theme, normalizeArray(in.Tags), in.Notes, in.Favorite, in.EstimatedMinutes).Scan(&id)
	if err != nil {
		return models.Workout{}, mapWriteError("upserting workout", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM workout_movements WHERE workout_id = $1`, id); err != nil {
		return models.Workout{}, fmt.Errorf("clearing workout movements: %w", err)
	}
	if err := insertMovementsInOrder(ctx, tx, id, in.Movements); err != nil {
		return models.Workout{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return models.Workout{}, fmt.Errorf("commit: %w", err)
	}
	return r.GetByID(ctx, id)
}

func (r *WorkoutRepo) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM workouts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting workout: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("workout %d: %w", id, ErrNotFound)
	}
	return nil
}

// AddMovement appends a movement to a workout at the next position and returns the
// refreshed workout. Guards the workout's existence for a clean 404.
func (r *WorkoutRepo) AddMovement(ctx context.Context, workoutID int64, in models.WorkoutMovementInput) (models.Workout, error) {
	if _, err := r.GetByID(ctx, workoutID); err != nil {
		return models.Workout{}, err
	}
	// coalesce so the first entry lands at position 1.
	var next int
	if err := r.pool.QueryRow(ctx,
		`SELECT coalesce(max(position), 0) + 1 FROM workout_movements WHERE workout_id = $1`,
		workoutID).Scan(&next); err != nil {
		return models.Workout{}, fmt.Errorf("computing next position: %w", err)
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO workout_movements (workout_id, movement_id, position, sets, reps, load,
			rest_seconds, superset_group, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		workoutID, in.MovementID, next, in.Sets, in.Reps, in.Load, in.RestSeconds, in.SupersetGroup, in.Notes)
	if err != nil {
		return models.Workout{}, mapWriteError("adding movement to workout", err)
	}
	return r.GetByID(ctx, workoutID)
}

// UpdateMovement applies a partial update to one entry (identified by entryID within
// workoutID) and returns the refreshed workout. A nil field is left unchanged;
// MovementID re-points the entry (the swap), carrying the untouched prescription to
// the substitute.
func (r *WorkoutRepo) UpdateMovement(ctx context.Context, workoutID, entryID int64, in models.WorkoutMovementUpdate) (models.Workout, error) {
	current, err := r.getMovementEntry(ctx, workoutID, entryID)
	if err != nil {
		return models.Workout{}, err
	}

	movementID := current.MovementID
	if in.MovementID != nil {
		movementID = *in.MovementID
	}
	notes := valueOr(in.Notes, current.Notes)
	sets := pickPtr(in.Sets, current.Sets)
	reps := pickPtr(in.Reps, current.Reps)
	load := pickPtr(in.Load, current.Load)
	rest := pickPtr(in.RestSeconds, current.RestSeconds)
	superset := pickPtr(in.SupersetGroup, current.SupersetGroup)

	_, err = r.pool.Exec(ctx, `
		UPDATE workout_movements SET
			movement_id = $1, sets = $2, reps = $3, load = $4, rest_seconds = $5,
			superset_group = $6, notes = $7
		WHERE id = $8 AND workout_id = $9`,
		movementID, sets, reps, load, rest, superset, notes, entryID, workoutID)
	if err != nil {
		return models.Workout{}, mapWriteError("updating workout movement", err)
	}
	return r.GetByID(ctx, workoutID)
}

// ReorderMovements rewrites the positions of a workout's entries to match the given
// id order (1..n). The supplied set must be exactly the workout's current entries —
// no more, no fewer — else ErrInvalid. Runs inside a transaction with the position
// uniqueness deferred so the intermediate swaps never trip the constraint.
func (r *WorkoutRepo) ReorderMovements(ctx context.Context, workoutID int64, entryIDs []int64) (models.Workout, error) {
	current, err := r.GetByID(ctx, workoutID)
	if err != nil {
		return models.Workout{}, err
	}
	if !sameEntrySet(current.Movements, entryIDs) {
		return models.Workout{}, fmt.Errorf("reorder must list every entry exactly once: %w", ErrInvalid)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return models.Workout{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Defer the (workout_id, position) uniqueness to commit so reassigning
	// positions across rows doesn't collide mid-transaction.
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS workout_movements_position_uniq DEFERRED`); err != nil {
		return models.Workout{}, fmt.Errorf("deferring constraint: %w", err)
	}
	for pos, entryID := range entryIDs {
		_, err := tx.Exec(ctx,
			`UPDATE workout_movements SET position = $1 WHERE id = $2 AND workout_id = $3`,
			pos+1, entryID, workoutID)
		if err != nil {
			return models.Workout{}, fmt.Errorf("reordering: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return models.Workout{}, fmt.Errorf("commit: %w", err)
	}
	return r.GetByID(ctx, workoutID)
}

// RemoveMovement deletes one entry from a workout, leaving remaining positions as
// they are (order is by position, so a gap is harmless). Returns ErrNotFound when
// the entry is not part of the workout.
func (r *WorkoutRepo) RemoveMovement(ctx context.Context, workoutID, entryID int64) (models.Workout, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM workout_movements WHERE id = $1 AND workout_id = $2`, entryID, workoutID)
	if err != nil {
		return models.Workout{}, fmt.Errorf("removing workout movement: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return models.Workout{}, fmt.Errorf("workout %d entry %d: %w", workoutID, entryID, ErrNotFound)
	}
	return r.GetByID(ctx, workoutID)
}

// getMovementEntry fetches one entry scoped to its workout, so an entry id from a
// different workout is a clean 404 rather than a cross-workout edit.
func (r *WorkoutRepo) getMovementEntry(ctx context.Context, workoutID, entryID int64) (models.WorkoutMovement, error) {
	var wm models.WorkoutMovement
	err := r.pool.QueryRow(ctx, `
		SELECT id, movement_id, position, sets, reps, load, rest_seconds, superset_group, notes
		FROM workout_movements WHERE id = $1 AND workout_id = $2`, entryID, workoutID).Scan(
		&wm.ID, &wm.MovementID, &wm.Position, &wm.Sets, &wm.Reps, &wm.Load,
		&wm.RestSeconds, &wm.SupersetGroup, &wm.Notes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.WorkoutMovement{}, fmt.Errorf("workout %d entry %d: %w", workoutID, entryID, ErrNotFound)
		}
		return models.WorkoutMovement{}, fmt.Errorf("fetching workout movement: %w", err)
	}
	return wm, nil
}

// insertWorkout inserts the workout row and returns its id, within the caller's tx.
func insertWorkout(ctx context.Context, tx pgx.Tx, in models.WorkoutCreate) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO workouts (name, theme, tags, notes, favorite, estimated_minutes)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		in.Name, in.Theme, normalizeArray(in.Tags), in.Notes, in.Favorite, in.EstimatedMinutes).Scan(&id)
	if err != nil {
		return 0, mapWriteError("creating workout", err)
	}
	return id, nil
}

// insertMovementsInOrder inserts the entries at positions 1..n in array order,
// within the caller's tx. A movement referenced by an unknown id fails the FK and
// surfaces as ErrReferenced.
func insertMovementsInOrder(ctx context.Context, tx pgx.Tx, workoutID int64, entries []models.WorkoutMovementInput) error {
	for i, e := range entries {
		_, err := tx.Exec(ctx, `
			INSERT INTO workout_movements (workout_id, movement_id, position, sets, reps, load,
				rest_seconds, superset_group, notes)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			workoutID, e.MovementID, i+1, e.Sets, e.Reps, e.Load, e.RestSeconds, e.SupersetGroup, e.Notes)
		if err != nil {
			return mapWriteError("adding movement to workout", err)
		}
	}
	return nil
}

// sameEntrySet reports whether entryIDs is a permutation of the workout's current
// entry ids — the precondition for a well-formed reorder.
func sameEntrySet(current []models.WorkoutMovement, entryIDs []int64) bool {
	if len(current) != len(entryIDs) {
		return false
	}
	want := make(map[int64]bool, len(current))
	for _, wm := range current {
		want[wm.ID] = true
	}
	seen := make(map[int64]bool, len(entryIDs))
	for _, id := range entryIDs {
		if !want[id] || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
