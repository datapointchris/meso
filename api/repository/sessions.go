package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"meso/api/models"
)

type SessionRepo struct {
	pool *pgxpool.Pool
}

func NewSessionRepo(pool *pgxpool.Pool) *SessionRepo {
	return &SessionRepo{pool: pool}
}

// dateLayout is the wire format for the DATE-typed performed_on: a calendar date
// with no time component.
const dateLayout = "2006-01-02"

// sessionSelect is the shared read query. It LEFT JOINs workouts so the template's
// name renders inline; the join is left so an ad-hoc session (workout_id null) or one
// whose template was deleted (SET NULL) still returns.
const sessionSelect = `
	SELECT s.id, s.workout_id, w.name, s.performed_on, s.duration_minutes,
		s.overall_notes, s.felt, s.created_at
	FROM workout_sessions s
	LEFT JOIN workouts w ON w.id = s.workout_id`

func scanSession(row pgx.Row) (models.WorkoutSession, error) {
	var s models.WorkoutSession
	var performedOn time.Time
	err := row.Scan(
		&s.ID, &s.WorkoutID, &s.WorkoutName, &performedOn,
		&s.DurationMinutes, &s.OverallNotes, &s.Felt, &s.CreatedAt,
	)
	if err != nil {
		return models.WorkoutSession{}, err
	}
	s.PerformedOn = performedOn.Format(dateLayout)
	s.Movements = []models.SessionMovement{}
	return s, nil
}

// List returns sessions matching the filter, newest first. Filtering is done in SQL
// so the CLI and web share one definition. The logged movement lists are attached in
// a single follow-up query to avoid an N+1.
func (r *SessionRepo) List(ctx context.Context, f models.WorkoutSessionFilter) ([]models.WorkoutSession, error) {
	var (
		where []string
		args  []any
	)
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}

	if f.WorkoutID != nil {
		add("s.workout_id = $%d", *f.WorkoutID)
	}
	if f.From != "" {
		from, err := parseDate(f.From)
		if err != nil {
			return nil, err
		}
		add("s.performed_on >= $%d", from)
	}
	if f.To != "" {
		to, err := parseDate(f.To)
		if err != nil {
			return nil, err
		}
		add("s.performed_on <= $%d", to)
	}

	query := sessionSelect
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY s.performed_on DESC, s.created_at DESC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	defer rows.Close()

	sessions := []models.WorkoutSession{}
	byID := map[uuid.UUID]int{}
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning session: %w", err)
		}
		byID[s.ID] = len(sessions)
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(sessions) > 0 {
		if err := r.attachMovements(ctx, sessions, byID); err != nil {
			return nil, err
		}
	}
	return sessions, nil
}

// attachMovements loads every session_movement for the given sessions in one query,
// joined with the movement's name/kind for render, and hangs each onto its session
// ordered by position.
func (r *SessionRepo) attachMovements(ctx context.Context, sessions []models.WorkoutSession, byID map[uuid.UUID]int) error {
	ids := make([]uuid.UUID, 0, len(sessions))
	for _, s := range sessions {
		ids = append(ids, s.ID)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT sm.session_id, sm.id, sm.movement_id, m.name, m.movement_kind, sm.position,
			sm.done, sm.actual_sets, sm.actual_reps, sm.actual_load, sm.notes
		FROM session_movements sm
		JOIN movements m ON m.id = sm.movement_id
		WHERE sm.session_id = ANY($1)
		ORDER BY sm.session_id, sm.position`, ids)
	if err != nil {
		return fmt.Errorf("loading session movements: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var sessionID uuid.UUID
		var sm models.SessionMovement
		if err := rows.Scan(&sessionID, &sm.ID, &sm.MovementID, &sm.MovementName, &sm.MovementKind,
			&sm.Position, &sm.Done, &sm.ActualSets, &sm.ActualReps, &sm.ActualLoad, &sm.Notes); err != nil {
			return fmt.Errorf("scanning session movement: %w", err)
		}
		idx := byID[sessionID]
		sessions[idx].Movements = append(sessions[idx].Movements, sm)
	}
	return rows.Err()
}

func (r *SessionRepo) GetByID(ctx context.Context, id uuid.UUID) (models.WorkoutSession, error) {
	s, err := scanSession(r.pool.QueryRow(ctx, sessionSelect+" WHERE s.id = $1", id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.WorkoutSession{}, fmt.Errorf("session %s: %w", id, ErrNotFound)
		}
		return models.WorkoutSession{}, fmt.Errorf("fetching session: %w", err)
	}
	list := []models.WorkoutSession{s}
	if err := r.attachMovements(ctx, list, map[uuid.UUID]int{s.ID: 0}); err != nil {
		return models.WorkoutSession{}, err
	}
	return list[0], nil
}

// Create starts a session and returns it fully populated. When WorkoutID is set, the
// workout's prescribed movements are copied into the session (start-from-template):
// the prescription (sets/reps/load) seeds actual_* as a starting point, done=false —
// so "did exactly the plan" is a one-tap check-off. Otherwise the supplied Movements
// build an ad-hoc session. The UUID7 id is minted here (time-ordered).
func (r *SessionRepo) Create(ctx context.Context, in models.WorkoutSessionCreate) (models.WorkoutSession, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return models.WorkoutSession{}, fmt.Errorf("generating session id: %w", err)
	}

	performedOn := time.Now()
	if in.PerformedOn != "" {
		performedOn, err = parseDate(in.PerformedOn)
		if err != nil {
			return models.WorkoutSession{}, err
		}
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return models.WorkoutSession{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after a successful commit is a no-op

	_, err = tx.Exec(ctx, `
		INSERT INTO workout_sessions (id, workout_id, performed_on, duration_minutes, overall_notes, felt)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		id, in.WorkoutID, performedOn, in.DurationMinutes, in.OverallNotes, in.Felt)
	if err != nil {
		return models.WorkoutSession{}, mapWriteError("creating session", err)
	}

	if in.WorkoutID != nil {
		// Copy the template's ordered movements, seeding actual_* from the
		// prescription so the session opens pre-filled with the plan.
		_, err = tx.Exec(ctx, `
			INSERT INTO session_movements (session_id, movement_id, position, done, actual_sets, actual_reps, actual_load)
			SELECT $1, wm.movement_id, wm.position, false, wm.sets, wm.reps, wm.load
			FROM workout_movements wm
			WHERE wm.workout_id = $2
			ORDER BY wm.position`,
			id, *in.WorkoutID)
		if err != nil {
			return models.WorkoutSession{}, mapWriteError("copying workout movements", err)
		}
	} else {
		if err := insertSessionMovementsInOrder(ctx, tx, id, in.Movements); err != nil {
			return models.WorkoutSession{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return models.WorkoutSession{}, fmt.Errorf("commit: %w", err)
	}
	return r.GetByID(ctx, id)
}

func (r *SessionRepo) Update(ctx context.Context, id uuid.UUID, in models.WorkoutSessionUpdate) (models.WorkoutSession, error) {
	current, err := r.GetByID(ctx, id)
	if err != nil {
		return models.WorkoutSession{}, err
	}

	performedOn, err := parseDate(current.PerformedOn)
	if err != nil {
		return models.WorkoutSession{}, err
	}
	if in.PerformedOn != nil {
		performedOn, err = parseDate(*in.PerformedOn)
		if err != nil {
			return models.WorkoutSession{}, err
		}
	}
	overallNotes := valueOr(in.OverallNotes, current.OverallNotes)
	duration := pickPtr(in.DurationMinutes, current.DurationMinutes)
	felt := pickPtr(in.Felt, current.Felt)

	_, err = r.pool.Exec(ctx, `
		UPDATE workout_sessions SET
			performed_on = $1, duration_minutes = $2, overall_notes = $3, felt = $4
		WHERE id = $5`,
		performedOn, duration, overallNotes, felt, id)
	if err != nil {
		return models.WorkoutSession{}, mapWriteError("updating session", err)
	}
	return r.GetByID(ctx, id)
}

func (r *SessionRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM workout_sessions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("session %s: %w", id, ErrNotFound)
	}
	return nil
}

// UpdateMovement applies a partial update to one logged entry (identified by entryID
// within sessionID) and returns the refreshed session. A nil field is left unchanged;
// MovementID re-points the entry (the mid-session swap), carrying the untouched
// actuals to the substitute.
func (r *SessionRepo) UpdateMovement(ctx context.Context, sessionID uuid.UUID, entryID int64, in models.SessionMovementUpdate) (models.WorkoutSession, error) {
	current, err := r.getMovementEntry(ctx, sessionID, entryID)
	if err != nil {
		return models.WorkoutSession{}, err
	}

	movementID := current.MovementID
	if in.MovementID != nil {
		movementID = *in.MovementID
	}
	done := valueOr(in.Done, current.Done)
	notes := valueOr(in.Notes, current.Notes)
	actualSets := pickPtr(in.ActualSets, current.ActualSets)
	actualReps := pickPtr(in.ActualReps, current.ActualReps)
	actualLoad := pickPtr(in.ActualLoad, current.ActualLoad)

	_, err = r.pool.Exec(ctx, `
		UPDATE session_movements SET
			movement_id = $1, done = $2, actual_sets = $3, actual_reps = $4, actual_load = $5, notes = $6
		WHERE id = $7 AND session_id = $8`,
		movementID, done, actualSets, actualReps, actualLoad, notes, entryID, sessionID)
	if err != nil {
		return models.WorkoutSession{}, mapWriteError("updating session movement", err)
	}
	return r.GetByID(ctx, sessionID)
}

// getMovementEntry fetches one logged entry scoped to its session, so an entry id
// from a different session is a clean 404 rather than a cross-session edit.
func (r *SessionRepo) getMovementEntry(ctx context.Context, sessionID uuid.UUID, entryID int64) (models.SessionMovement, error) {
	var sm models.SessionMovement
	err := r.pool.QueryRow(ctx, `
		SELECT id, movement_id, position, done, actual_sets, actual_reps, actual_load, notes
		FROM session_movements WHERE id = $1 AND session_id = $2`, entryID, sessionID).Scan(
		&sm.ID, &sm.MovementID, &sm.Position, &sm.Done, &sm.ActualSets, &sm.ActualReps, &sm.ActualLoad, &sm.Notes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.SessionMovement{}, fmt.Errorf("session %s entry %d: %w", sessionID, entryID, ErrNotFound)
		}
		return models.SessionMovement{}, fmt.Errorf("fetching session movement: %w", err)
	}
	return sm, nil
}

// insertSessionMovementsInOrder inserts ad-hoc entries at positions 1..n in array
// order, within the caller's tx. A movement referenced by an unknown id fails the FK
// and surfaces as ErrReferenced.
func insertSessionMovementsInOrder(ctx context.Context, tx pgx.Tx, sessionID uuid.UUID, entries []models.SessionMovementInput) error {
	for i, e := range entries {
		_, err := tx.Exec(ctx, `
			INSERT INTO session_movements (session_id, movement_id, position, done, actual_sets, actual_reps, actual_load, notes)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			sessionID, e.MovementID, i+1, e.Done, e.ActualSets, e.ActualReps, e.ActualLoad, e.Notes)
		if err != nil {
			return mapWriteError("adding session movement", err)
		}
	}
	return nil
}

// parseDate parses a "2006-01-02" wire date, returning ErrInvalid on a bad value so
// the handler maps it to 400 rather than 500.
func parseDate(s string) (time.Time, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q: want YYYY-MM-DD: %w", s, ErrInvalid)
	}
	return t, nil
}
