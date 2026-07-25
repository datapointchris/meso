package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"meso/api/models"
)

type CycleRepo struct {
	pool *pgxpool.Pool
}

func NewCycleRepo(pool *pgxpool.Pool) *CycleRepo {
	return &CycleRepo{pool: pool}
}

// cycleCols is the ordered column list shared by every cycle SELECT so the scan
// stays in lockstep with the query.
const cycleCols = `id, name, goal_summary, target_metric, target_value, target_date, start_date, status, notes, created_at, updated_at`

func scanCycle(row pgx.Row) (models.Cycle, error) {
	var c models.Cycle
	var targetDate, startDate *time.Time
	err := row.Scan(
		&c.ID, &c.Name, &c.GoalSummary, &c.TargetMetric, &c.TargetValue,
		&targetDate, &startDate, &c.Status, &c.Notes, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return models.Cycle{}, err
	}
	c.TargetDate = formatNullableDate(targetDate)
	c.StartDate = formatNullableDate(startDate)
	c.Workouts = []models.CycleWorkout{}
	return c, nil
}

// List returns cycles matching the filter, ordered by status then name. Filtering
// is done in SQL so the CLI and web share one definition of each filter. The ordered
// workout sequences are attached in a single follow-up query to avoid an N+1.
func (r *CycleRepo) List(ctx context.Context, f models.CycleFilter) ([]models.Cycle, error) {
	var (
		where []string
		args  []any
	)
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}

	if f.Status != "" {
		add("status = $%d", f.Status)
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		where = append(where, fmt.Sprintf(
			"(name ILIKE $%d OR goal_summary ILIKE $%d)", len(args), len(args)))
	}

	query := `SELECT ` + cycleCols + ` FROM cycles`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY status, name"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing cycles: %w", err)
	}
	defer rows.Close()

	cycles := []models.Cycle{}
	byID := map[int64]int{}
	for rows.Next() {
		c, err := scanCycle(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning cycle: %w", err)
		}
		byID[c.ID] = len(cycles)
		cycles = append(cycles, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(cycles) > 0 {
		if err := r.attachWorkouts(ctx, cycles, byID); err != nil {
			return nil, err
		}
	}
	return cycles, nil
}

// attachWorkouts loads every cycle_workout for the given cycles in one query, joined
// with the workout's name/theme for render, and hangs each onto its cycle ordered by
// position.
func (r *CycleRepo) attachWorkouts(ctx context.Context, cycles []models.Cycle, byID map[int64]int) error {
	ids := make([]int64, 0, len(cycles))
	for _, c := range cycles {
		ids = append(ids, c.ID)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT cw.cycle_id, cw.id, cw.workout_id, w.name, w.theme, cw.position,
			cw.week, cw.phase, cw.frequency, cw.intensity, cw.conditions
		FROM cycle_workouts cw
		JOIN workouts w ON w.id = cw.workout_id
		WHERE cw.cycle_id = ANY($1)
		ORDER BY cw.cycle_id, cw.position`, ids)
	if err != nil {
		return fmt.Errorf("loading cycle workouts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cycleID int64
		var cw models.CycleWorkout
		if err := rows.Scan(&cycleID, &cw.ID, &cw.WorkoutID, &cw.WorkoutName, &cw.WorkoutTheme,
			&cw.Position, &cw.Week, &cw.Phase, &cw.Frequency, &cw.Intensity, &cw.Conditions); err != nil {
			return fmt.Errorf("scanning cycle workout: %w", err)
		}
		idx := byID[cycleID]
		cycles[idx].Workouts = append(cycles[idx].Workouts, cw)
	}
	return rows.Err()
}

func (r *CycleRepo) GetByID(ctx context.Context, id int64) (models.Cycle, error) {
	c, err := scanCycle(r.pool.QueryRow(ctx, `SELECT `+cycleCols+` FROM cycles WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Cycle{}, fmt.Errorf("cycle %d: %w", id, ErrNotFound)
		}
		return models.Cycle{}, fmt.Errorf("fetching cycle: %w", err)
	}
	list := []models.Cycle{c}
	if err := r.attachWorkouts(ctx, list, map[int64]int{c.ID: 0}); err != nil {
		return models.Cycle{}, err
	}
	return list[0], nil
}

func (r *CycleRepo) Create(ctx context.Context, in models.CycleCreate) (models.Cycle, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return models.Cycle{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after a successful commit is a no-op

	id, err := insertCycle(ctx, tx, in)
	if err != nil {
		return models.Cycle{}, err
	}
	if err := insertCycleWorkoutsInOrder(ctx, tx, id, in.Workouts); err != nil {
		return models.Cycle{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return models.Cycle{}, fmt.Errorf("commit: %w", err)
	}
	return r.GetByID(ctx, id)
}

func (r *CycleRepo) Update(ctx context.Context, id int64, in models.CycleUpdate) (models.Cycle, error) {
	current, err := r.GetByID(ctx, id)
	if err != nil {
		return models.Cycle{}, err
	}

	name := valueOr(in.Name, current.Name)
	goal := valueOr(in.GoalSummary, current.GoalSummary)
	status := valueOr(in.Status, current.Status)
	notes := valueOr(in.Notes, current.Notes)
	targetValue := pickPtr(in.TargetValue, current.TargetValue)
	targetMetric := clearableString(pickPtr(in.TargetMetric, current.TargetMetric))
	// A target value is bound to its metric. If the metric is being changed or
	// cleared and no new value is supplied, the old value is stale — a number
	// against the wrong (or no) metric — so clear it too.
	if in.TargetMetric != nil && in.TargetValue == nil {
		targetValue = nil
	}

	targetDate, err := resolveNullableDate(in.TargetDate, current.TargetDate)
	if err != nil {
		return models.Cycle{}, err
	}
	startDate, err := resolveNullableDate(in.StartDate, current.StartDate)
	if err != nil {
		return models.Cycle{}, err
	}

	_, err = r.pool.Exec(ctx, `
		UPDATE cycles SET
			name = $1, goal_summary = $2, target_metric = $3, target_value = $4,
			target_date = $5, start_date = $6, status = $7, notes = $8, updated_at = now()
		WHERE id = $9`,
		name, goal, targetMetric, targetValue, targetDate, startDate, status, notes, id)
	if err != nil {
		return models.Cycle{}, mapWriteError("updating cycle", err)
	}
	return r.GetByID(ctx, id)
}

// Upsert inserts a cycle or overwrites an existing one matched by name, replacing
// its workout sequence wholesale — the idempotent write path for cmd/seed and the
// CLI import pass (the research plans become cycles this way).
func (r *CycleRepo) Upsert(ctx context.Context, in models.CycleCreate) (models.Cycle, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return models.Cycle{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	targetDate, err := parseNullableDate(in.TargetDate)
	if err != nil {
		return models.Cycle{}, err
	}
	startDate, err := parseNullableDate(in.StartDate)
	if err != nil {
		return models.Cycle{}, err
	}
	status := in.Status
	if status == "" {
		status = "planned"
	}

	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO cycles (name, goal_summary, target_metric, target_value, target_date, start_date, status, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (name) DO UPDATE SET
			goal_summary = EXCLUDED.goal_summary, target_metric = EXCLUDED.target_metric,
			target_value = EXCLUDED.target_value, target_date = EXCLUDED.target_date,
			start_date = EXCLUDED.start_date, status = EXCLUDED.status,
			notes = EXCLUDED.notes, updated_at = now()
		RETURNING id`,
		in.Name, in.GoalSummary, in.TargetMetric, in.TargetValue, targetDate, startDate, status, in.Notes).Scan(&id)
	if err != nil {
		return models.Cycle{}, mapWriteError("upserting cycle", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM cycle_workouts WHERE cycle_id = $1`, id); err != nil {
		return models.Cycle{}, fmt.Errorf("clearing cycle workouts: %w", err)
	}
	if err := insertCycleWorkoutsInOrder(ctx, tx, id, in.Workouts); err != nil {
		return models.Cycle{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return models.Cycle{}, fmt.Errorf("commit: %w", err)
	}
	return r.GetByID(ctx, id)
}

func (r *CycleRepo) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM cycles WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting cycle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("cycle %d: %w", id, ErrNotFound)
	}
	return nil
}

// AddWorkout appends a workout to a cycle at the next position and returns the
// refreshed cycle. Guards the cycle's existence for a clean 404.
func (r *CycleRepo) AddWorkout(ctx context.Context, cycleID int64, in models.CycleWorkoutInput) (models.Cycle, error) {
	if _, err := r.GetByID(ctx, cycleID); err != nil {
		return models.Cycle{}, err
	}
	var next int
	if err := r.pool.QueryRow(ctx,
		`SELECT coalesce(max(position), 0) + 1 FROM cycle_workouts WHERE cycle_id = $1`,
		cycleID).Scan(&next); err != nil {
		return models.Cycle{}, fmt.Errorf("computing next position: %w", err)
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO cycle_workouts (cycle_id, workout_id, position, week, phase, frequency, intensity, conditions)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		cycleID, in.WorkoutID, next, in.Week, in.Phase, in.Frequency, in.Intensity, in.Conditions)
	if err != nil {
		return models.Cycle{}, mapWriteError("adding workout to cycle", err)
	}
	return r.GetByID(ctx, cycleID)
}

// UpdateWorkout applies a partial update to one entry (identified by entryID within
// cycleID) and returns the refreshed cycle. A nil field is left unchanged; WorkoutID
// re-points the entry (the swap), carrying the untouched prescription to the
// substitute.
func (r *CycleRepo) UpdateWorkout(ctx context.Context, cycleID, entryID int64, in models.CycleWorkoutUpdate) (models.Cycle, error) {
	current, err := r.getWorkoutEntry(ctx, cycleID, entryID)
	if err != nil {
		return models.Cycle{}, err
	}

	workoutID := current.WorkoutID
	if in.WorkoutID != nil {
		workoutID = *in.WorkoutID
	}
	week := pickPtr(in.Week, current.Week)
	phase := pickPtr(in.Phase, current.Phase)
	frequency := pickPtr(in.Frequency, current.Frequency)
	intensity := pickPtr(in.Intensity, current.Intensity)
	conditions := pickPtr(in.Conditions, current.Conditions)

	_, err = r.pool.Exec(ctx, `
		UPDATE cycle_workouts SET
			workout_id = $1, week = $2, phase = $3, frequency = $4, intensity = $5, conditions = $6
		WHERE id = $7 AND cycle_id = $8`,
		workoutID, week, phase, frequency, intensity, conditions, entryID, cycleID)
	if err != nil {
		return models.Cycle{}, mapWriteError("updating cycle workout", err)
	}
	return r.GetByID(ctx, cycleID)
}

// ReorderWorkouts rewrites the positions of a cycle's entries to match the given id
// order (1..n). The supplied set must be exactly the cycle's current entries — no
// more, no fewer — else ErrInvalid. Runs inside a transaction with the position
// uniqueness deferred so the intermediate swaps never trip the constraint.
func (r *CycleRepo) ReorderWorkouts(ctx context.Context, cycleID int64, entryIDs []int64) (models.Cycle, error) {
	current, err := r.GetByID(ctx, cycleID)
	if err != nil {
		return models.Cycle{}, err
	}
	currentIDs := make([]int64, len(current.Workouts))
	for i, cw := range current.Workouts {
		currentIDs[i] = cw.ID
	}
	if !sameIDSet(currentIDs, entryIDs) {
		return models.Cycle{}, fmt.Errorf("reorder must list every entry exactly once: %w", ErrInvalid)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return models.Cycle{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `SET CONSTRAINTS cycle_workouts_position_uniq DEFERRED`); err != nil {
		return models.Cycle{}, fmt.Errorf("deferring constraint: %w", err)
	}
	for pos, entryID := range entryIDs {
		_, err := tx.Exec(ctx,
			`UPDATE cycle_workouts SET position = $1 WHERE id = $2 AND cycle_id = $3`,
			pos+1, entryID, cycleID)
		if err != nil {
			return models.Cycle{}, fmt.Errorf("reordering: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return models.Cycle{}, fmt.Errorf("commit: %w", err)
	}
	return r.GetByID(ctx, cycleID)
}

// RemoveWorkout deletes one entry from a cycle, leaving remaining positions as they
// are (order is by position, so a gap is harmless). Returns ErrNotFound when the
// entry is not part of the cycle.
func (r *CycleRepo) RemoveWorkout(ctx context.Context, cycleID, entryID int64) (models.Cycle, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM cycle_workouts WHERE id = $1 AND cycle_id = $2`, entryID, cycleID)
	if err != nil {
		return models.Cycle{}, fmt.Errorf("removing cycle workout: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return models.Cycle{}, fmt.Errorf("cycle %d entry %d: %w", cycleID, entryID, ErrNotFound)
	}
	return r.GetByID(ctx, cycleID)
}

// getWorkoutEntry fetches one entry scoped to its cycle, so an entry id from a
// different cycle is a clean 404 rather than a cross-cycle edit.
func (r *CycleRepo) getWorkoutEntry(ctx context.Context, cycleID, entryID int64) (models.CycleWorkout, error) {
	var cw models.CycleWorkout
	err := r.pool.QueryRow(ctx, `
		SELECT id, workout_id, position, week, phase, frequency, intensity, conditions
		FROM cycle_workouts WHERE id = $1 AND cycle_id = $2`, entryID, cycleID).Scan(
		&cw.ID, &cw.WorkoutID, &cw.Position, &cw.Week, &cw.Phase, &cw.Frequency, &cw.Intensity, &cw.Conditions)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.CycleWorkout{}, fmt.Errorf("cycle %d entry %d: %w", cycleID, entryID, ErrNotFound)
		}
		return models.CycleWorkout{}, fmt.Errorf("fetching cycle workout: %w", err)
	}
	return cw, nil
}

// insertCycle inserts the cycle row and returns its id, within the caller's tx.
// Status defaults to "planned" when the caller left it empty.
func insertCycle(ctx context.Context, tx pgx.Tx, in models.CycleCreate) (int64, error) {
	targetDate, err := parseNullableDate(in.TargetDate)
	if err != nil {
		return 0, err
	}
	startDate, err := parseNullableDate(in.StartDate)
	if err != nil {
		return 0, err
	}
	status := in.Status
	if status == "" {
		status = "planned"
	}

	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO cycles (name, goal_summary, target_metric, target_value, target_date, start_date, status, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		in.Name, in.GoalSummary, in.TargetMetric, in.TargetValue, targetDate, startDate, status, in.Notes).Scan(&id)
	if err != nil {
		return 0, mapWriteError("creating cycle", err)
	}
	return id, nil
}

// insertCycleWorkoutsInOrder inserts the entries at positions 1..n in array order,
// within the caller's tx. A workout referenced by an unknown id fails the FK and
// surfaces as ErrReferenced.
func insertCycleWorkoutsInOrder(ctx context.Context, tx pgx.Tx, cycleID int64, entries []models.CycleWorkoutInput) error {
	for i, e := range entries {
		_, err := tx.Exec(ctx, `
			INSERT INTO cycle_workouts (cycle_id, workout_id, position, week, phase, frequency, intensity, conditions)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			cycleID, e.WorkoutID, i+1, e.Week, e.Phase, e.Frequency, e.Intensity, e.Conditions)
		if err != nil {
			return mapWriteError("adding workout to cycle", err)
		}
	}
	return nil
}

// formatNullableDate renders a scanned nullable DATE as the wire string, or nil.
func formatNullableDate(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(dateLayout)
	return &s
}

// parseNullableDate converts an optional wire date to a nullable time: a nil or
// empty string is NULL (stored as no date); a malformed value is ErrInvalid (400).
func parseNullableDate(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := parseDate(*s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// resolveNullableDate computes the value to write for a nullable DATE update field.
// A supplied incoming value wins (empty string clears it to NULL); when the field
// was absent, the current stored value is carried through unchanged.
func resolveNullableDate(incoming, current *string) (*time.Time, error) {
	if incoming != nil {
		return parseNullableDate(incoming)
	}
	return parseNullableDate(current)
}

// clearableString treats an explicit empty string as "clear to NULL" for a nullable
// TEXT field (target_metric), where an empty value can never be a valid FK anyway.
func clearableString(s *string) *string {
	if s != nil && *s == "" {
		return nil
	}
	return s
}
