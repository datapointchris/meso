package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	if err := r.attachPreviousActuals(ctx, &list[0]); err != nil {
		return models.WorkoutSession{}, err
	}
	return list[0], nil
}

// attachPreviousActuals hangs the last recorded performance of each movement onto its
// entry — the number to beat on the logging screen. Detail-only: the list endpoint has
// no use for it and would pay the query per session.
//
// Only entries marked done count. A session started from a template seeds actual_* from
// the prescription, so every session — including one opened and abandoned — carries
// numbers; without the done filter a plan never performed would come back as a result to
// beat. One DISTINCT ON query covers every movement in the session rather than one per
// entry.
func (r *SessionRepo) attachPreviousActuals(ctx context.Context, s *models.WorkoutSession) error {
	if len(s.Movements) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(s.Movements))
	for _, m := range s.Movements {
		ids = append(ids, m.MovementID)
	}
	performedOn, err := parseDate(s.PerformedOn)
	if err != nil {
		return err
	}

	// The (performed_on, created_at) tuple orders sessions on the same day, and the
	// strict < excludes this session from being its own previous.
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (sm.movement_id)
			sm.movement_id, prev.performed_on, sm.actual_sets, sm.actual_reps, sm.actual_load
		FROM session_movements sm
		JOIN workout_sessions prev ON prev.id = sm.session_id
		WHERE sm.movement_id = ANY($1)
			AND sm.done
			AND (prev.performed_on, prev.created_at) < ($2, $3)
		ORDER BY sm.movement_id, prev.performed_on DESC, prev.created_at DESC`,
		ids, performedOn, s.CreatedAt)
	if err != nil {
		return fmt.Errorf("loading previous actuals: %w", err)
	}
	defer rows.Close()

	previous := map[int64]*models.PreviousActuals{}
	for rows.Next() {
		var movementID int64
		var performed time.Time
		p := &models.PreviousActuals{}
		if err := rows.Scan(&movementID, &performed, &p.ActualSets, &p.ActualReps, &p.ActualLoad); err != nil {
			return fmt.Errorf("scanning previous actuals: %w", err)
		}
		p.PerformedOn = performed.Format(dateLayout)
		previous[movementID] = p
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i := range s.Movements {
		s.Movements[i].Previous = previous[s.Movements[i].MovementID]
	}
	return nil
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

// AddMovement appends one movement to an existing session and returns the refreshed
// session. This is what makes an ad-hoc session usable: it starts empty and grows as
// the workout is performed, so the entry order is the order things were actually done.
// The new entry lands after every existing one; a concurrent add is not a concern for
// a single-user app logging one session at a time.
func (r *SessionRepo) AddMovement(ctx context.Context, sessionID uuid.UUID, in models.SessionMovementInput) (models.WorkoutSession, error) {
	if _, err := r.GetByID(ctx, sessionID); err != nil {
		return models.WorkoutSession{}, err
	}

	var position int
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(position), 0) + 1 FROM session_movements WHERE session_id = $1`,
		sessionID).Scan(&position)
	if err != nil {
		return models.WorkoutSession{}, fmt.Errorf("finding next session position: %w", err)
	}

	if err := insertSessionMovement(ctx, r.pool, sessionID, position, in); err != nil {
		return models.WorkoutSession{}, err
	}
	return r.GetByID(ctx, sessionID)
}

// RemoveMovement drops one logged entry and returns the refreshed session. The
// remaining positions are left alone: reads order by position, and unlike
// workout_movements there is no UNIQUE(session_id, position) to satisfy, so a gap is
// invisible.
func (r *SessionRepo) RemoveMovement(ctx context.Context, sessionID uuid.UUID, entryID int64) (models.WorkoutSession, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM session_movements WHERE id = $1 AND session_id = $2`, entryID, sessionID)
	if err != nil {
		return models.WorkoutSession{}, fmt.Errorf("removing session movement: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return models.WorkoutSession{}, fmt.Errorf("session %s entry %d: %w", sessionID, entryID, ErrNotFound)
	}
	return r.GetByID(ctx, sessionID)
}

// PromoteToWorkout turns a performed ad-hoc session into a reusable workout template:
// what was actually logged (actual_sets/reps/load) becomes the prescription
// (sets/reps/load), in the order performed. The session is then back-linked to the new
// workout, so it reads as the first instance of the template it produced and its
// movements count toward that workout's history.
//
// Only an ad-hoc session can be promoted — a session already backed by a template would
// silently fork it, which is an edit of that workout, not a new one.
func (r *SessionRepo) PromoteToWorkout(ctx context.Context, sessionID uuid.UUID, in models.SessionPromote) (int64, error) {
	session, err := r.GetByID(ctx, sessionID)
	if err != nil {
		return 0, err
	}
	if session.WorkoutID != nil {
		return 0, fmt.Errorf("session %s already belongs to workout %d: %w", sessionID, *session.WorkoutID, ErrConflict)
	}
	if len(session.Movements) == 0 {
		return 0, fmt.Errorf("session %s has no movements to promote: %w", sessionID, ErrInvalid)
	}

	entries := make([]models.WorkoutMovementInput, 0, len(session.Movements))
	for _, m := range session.Movements {
		entries = append(entries, models.WorkoutMovementInput{
			MovementID: m.MovementID,
			Sets:       m.ActualSets,
			Reps:       m.ActualReps,
			Load:       m.ActualLoad,
			Notes:      m.Notes,
		})
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after a successful commit is a no-op

	workoutID, err := insertWorkout(ctx, tx, models.WorkoutCreate{
		Name:             in.Name,
		Theme:            in.Theme,
		Tags:             in.Tags,
		Notes:            in.Notes,
		EstimatedMinutes: in.EstimatedMinutes,
	})
	if err != nil {
		return 0, err
	}
	if err := insertMovementsInOrder(ctx, tx, workoutID, entries); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE workout_sessions SET workout_id = $1 WHERE id = $2`, workoutID, sessionID); err != nil {
		return 0, mapWriteError("linking session to promoted workout", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return workoutID, nil
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
// order, within the caller's tx.
func insertSessionMovementsInOrder(ctx context.Context, tx pgx.Tx, sessionID uuid.UUID, entries []models.SessionMovementInput) error {
	for i, e := range entries {
		if err := insertSessionMovement(ctx, tx, sessionID, i+1, e); err != nil {
			return err
		}
	}
	return nil
}

// insertSessionMovement inserts one logged entry at an explicit position. A movement
// referenced by an unknown id fails the FK and surfaces as ErrReferenced.
func insertSessionMovement(ctx context.Context, db execer, sessionID uuid.UUID, position int, e models.SessionMovementInput) error {
	_, err := db.Exec(ctx, `
		INSERT INTO session_movements (session_id, movement_id, position, done, actual_sets, actual_reps, actual_load, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		sessionID, e.MovementID, position, e.Done, e.ActualSets, e.ActualReps, e.ActualLoad, e.Notes)
	if err != nil {
		return mapWriteError("adding session movement", err)
	}
	return nil
}

// execer is the write subset shared by *pgxpool.Pool and pgx.Tx, so one insert serves
// both the transactional create path and the single untransacted append.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
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
