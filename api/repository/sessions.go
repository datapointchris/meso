package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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

// defaultSetKind is what a set is unless it says otherwise. Most sets are working
// sets, so the vocabulary should cost nothing to ignore.
const defaultSetKind = "working"

// sessionSelect is the shared read query. It LEFT JOINs workouts so the template's
// name renders inline; the join is left so a free-form session (workout_id null) or one
// whose template was deleted (SET NULL) still returns.
const sessionSelect = `
	SELECT s.id, s.workout_id, w.name, s.performed_on, s.duration_minutes,
		s.overall_notes, s.felt, s.created_at, s.finished_at
	FROM workout_sessions s
	LEFT JOIN workouts w ON w.id = s.workout_id`

func scanSession(row pgx.Row) (models.WorkoutSession, error) {
	var s models.WorkoutSession
	var performedOn time.Time
	err := row.Scan(
		&s.ID, &s.WorkoutID, &s.WorkoutName, &performedOn,
		&s.DurationMinutes, &s.OverallNotes, &s.Felt, &s.CreatedAt, &s.FinishedAt,
	)
	if err != nil {
		return models.WorkoutSession{}, err
	}
	s.PerformedOn = performedOn.Format(dateLayout)
	s.Movements = []models.SessionMovement{}
	return s, nil
}

// List returns sessions matching the filter, newest first. Filtering is done in SQL
// so the CLI and web share one definition. The logged movements and their sets are
// attached in two follow-up queries to avoid an N+1.
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
	if f.Unfinished {
		where = append(where, "s.finished_at IS NULL")
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
		if err := r.attachSets(ctx, sessions); err != nil {
			return nil, err
		}
	}
	return sessions, nil
}

// attachMovements loads every session_movement for the given sessions in one query,
// joined with the movement's name/kind/load mode for render, and hangs each onto its
// session ordered by position.
func (r *SessionRepo) attachMovements(ctx context.Context, sessions []models.WorkoutSession, byID map[uuid.UUID]int) error {
	ids := make([]uuid.UUID, 0, len(sessions))
	for _, s := range sessions {
		ids = append(ids, s.ID)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT sm.session_id, sm.id, sm.movement_id, m.name, m.movement_kind, m.load_mode, sm.position,
			sm.done, sm.target_sets, sm.target_reps, sm.target_load, sm.notes
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
			&sm.LoadMode, &sm.Position, &sm.Done, &sm.TargetSets, &sm.TargetReps, &sm.TargetLoad,
			&sm.Notes); err != nil {
			return fmt.Errorf("scanning session movement: %w", err)
		}
		sm.Sets = []models.SessionSet{}
		idx := byID[sessionID]
		sessions[idx].Movements = append(sessions[idx].Movements, sm)
	}
	return rows.Err()
}

// attachSets loads the sets for every entry across every session in one query. The
// third tier of the same batched read as attachMovements: sets are what a session
// actually is now, so a list of ten sessions must not become eleven round trips.
func (r *SessionRepo) attachSets(ctx context.Context, sessions []models.WorkoutSession) error {
	type location struct{ session, movement int }
	at := map[int64]location{}
	ids := []int64{}
	for si := range sessions {
		for mi := range sessions[si].Movements {
			id := sessions[si].Movements[mi].ID
			at[id] = location{session: si, movement: mi}
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT session_movement_id, id, position, reps, load, hold_seconds, set_kind, notes, logged_at
		FROM session_sets
		WHERE session_movement_id = ANY($1)
		ORDER BY session_movement_id, position`, ids)
	if err != nil {
		return fmt.Errorf("loading session sets: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entryID int64
		var s models.SessionSet
		if err := rows.Scan(&entryID, &s.ID, &s.Position, &s.Reps, &s.Load, &s.HoldSeconds,
			&s.SetKind, &s.Notes, &s.LoggedAt); err != nil {
			return fmt.Errorf("scanning session set: %w", err)
		}
		loc := at[entryID]
		m := &sessions[loc.session].Movements[loc.movement]
		m.Sets = append(m.Sets, s)
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
	if err := r.attachSets(ctx, list); err != nil {
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
// An entry qualifies only if it is done and has sets under it. Done alone is not enough:
// it can be ticked with nothing logged, and "last time: nothing" is not a number to beat.
//
// Reps and load come from the last set rather than an average: with warmups in the same
// list the last set is the working one. One DISTINCT ON query covers every movement in
// the session rather than one per entry.
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
			sm.movement_id, prev.performed_on,
			(SELECT count(*) FROM session_sets ss WHERE ss.session_movement_id = sm.id),
			last.reps, last.load
		FROM session_movements sm
		JOIN workout_sessions prev ON prev.id = sm.session_id
		LEFT JOIN LATERAL (
			SELECT ss.reps, ss.load
			FROM session_sets ss
			WHERE ss.session_movement_id = sm.id
			ORDER BY ss.position DESC
			LIMIT 1
		) last ON true
		WHERE sm.movement_id = ANY($1)
			AND sm.done
			AND EXISTS (SELECT 1 FROM session_sets ss WHERE ss.session_movement_id = sm.id)
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
		if err := rows.Scan(&movementID, &performed, &p.Sets, &p.Reps, &p.Load); err != nil {
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
// workout's prescribed movements are copied in as the session's targets
// (start-from-template) with no sets logged — the plan is present but nothing has been
// performed yet. Otherwise the supplied Movements build a free-form session. The UUID7
// id is minted here (time-ordered).
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
		// Copy the template's ordered movements as targets. Nothing is marked done and
		// no sets exist: the session opens showing the plan, not a pre-filled result.
		_, err = tx.Exec(ctx, `
			INSERT INTO session_movements (session_id, movement_id, position, done, target_sets, target_reps, target_load)
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

// Finish marks the end of training and fills in the duration so it never has to be
// typed. Idempotent: finishing again leaves the original timestamp, so a double tap
// cannot rewrite when the session ended.
//
// This says nothing about whether the plan was completed. A session finished with two
// of five movements logged is a finished session.
func (r *SessionRepo) Finish(ctx context.Context, id uuid.UUID) (models.WorkoutSession, error) {
	if _, err := r.GetByID(ctx, id); err != nil {
		return models.WorkoutSession{}, err
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE workout_sessions SET
			finished_at = now(),
			duration_minutes = COALESCE(duration_minutes,
				GREATEST(1, ROUND(EXTRACT(EPOCH FROM (now() - created_at)) / 60))::INT)
		WHERE id = $1 AND finished_at IS NULL`, id)
	if err != nil {
		return models.WorkoutSession{}, mapWriteError("finishing session", err)
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
// MovementID re-points the entry (the mid-session swap), carrying the target and the
// sets already logged to the substitute.
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
	targetSets := pickPtr(in.TargetSets, current.TargetSets)
	targetReps := pickPtr(in.TargetReps, current.TargetReps)
	targetLoad := pickPtr(in.TargetLoad, current.TargetLoad)

	_, err = r.pool.Exec(ctx, `
		UPDATE session_movements SET
			movement_id = $1, done = $2, target_sets = $3, target_reps = $4, target_load = $5, notes = $6
		WHERE id = $7 AND session_id = $8`,
		movementID, done, targetSets, targetReps, targetLoad, notes, entryID, sessionID)
	if err != nil {
		return models.WorkoutSession{}, mapWriteError("updating session movement", err)
	}
	return r.GetByID(ctx, sessionID)
}

// AddMovement appends one movement to an existing session and returns the refreshed
// session. It works whether or not the session came from a template: a session is a
// record of what was done, and doing something the plan did not call for has to be
// recordable or the record is a lie. The new entry lands after every existing one.
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

// RemoveMovement drops one logged entry, and its sets with it, returning the refreshed
// session. The remaining positions are left alone: reads order by position, so a gap is
// invisible, and renumbering would rewrite what order things happened in.
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

// AddSet logs one set against an entry and returns the refreshed session.
//
// A field left unset carries forward from the previous set — same reps, same load —
// falling back to the entry's target when this is the first one. That is the point: the
// common case is another set exactly like the last, and it should cost one tap rather
// than a form. Typing happens only when something actually changed.
func (r *SessionRepo) AddSet(ctx context.Context, sessionID uuid.UUID, entryID int64, in models.SessionSetInput) (models.WorkoutSession, error) {
	entry, err := r.getMovementEntry(ctx, sessionID, entryID)
	if err != nil {
		return models.WorkoutSession{}, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return models.WorkoutSession{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after a successful commit is a no-op

	position := 1
	reps, load, hold := in.Reps, in.Load, in.HoldSeconds

	var last models.SessionSet
	err = tx.QueryRow(ctx, `
		SELECT position, reps, load, hold_seconds
		FROM session_sets WHERE session_movement_id = $1
		ORDER BY position DESC LIMIT 1`, entryID).
		Scan(&last.Position, &last.Reps, &last.Load, &last.HoldSeconds)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		reps = pickPtr(reps, targetRepsAsInt(entry.TargetReps))
		load = pickPtr(load, entry.TargetLoad)
	case err != nil:
		return models.WorkoutSession{}, fmt.Errorf("finding previous set: %w", err)
	default:
		position = last.Position + 1
		reps = pickPtr(reps, last.Reps)
		load = pickPtr(load, last.Load)
		hold = pickPtr(hold, last.HoldSeconds)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO session_sets (session_movement_id, position, reps, load, hold_seconds, set_kind, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		entryID, position, reps, load, hold, valueOrDefault(in.SetKind, defaultSetKind), in.Notes)
	if err != nil {
		return models.WorkoutSession{}, mapWriteError("logging set", err)
	}
	if err := refreshDone(ctx, tx, entryID); err != nil {
		return models.WorkoutSession{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return models.WorkoutSession{}, fmt.Errorf("commit: %w", err)
	}
	return r.GetByID(ctx, sessionID)
}

// UpdateSet applies a partial update to one logged set and returns the refreshed
// session.
func (r *SessionRepo) UpdateSet(ctx context.Context, sessionID uuid.UUID, entryID, setID int64, in models.SessionSetUpdate) (models.WorkoutSession, error) {
	current, err := r.getSet(ctx, sessionID, entryID, setID)
	if err != nil {
		return models.WorkoutSession{}, err
	}

	_, err = r.pool.Exec(ctx, `
		UPDATE session_sets SET reps = $1, load = $2, hold_seconds = $3, set_kind = $4, notes = $5
		WHERE id = $6 AND session_movement_id = $7`,
		pickPtr(in.Reps, current.Reps),
		pickPtr(in.Load, current.Load),
		pickPtr(in.HoldSeconds, current.HoldSeconds),
		valueOr(in.SetKind, current.SetKind),
		valueOr(in.Notes, current.Notes),
		setID, entryID)
	if err != nil {
		return models.WorkoutSession{}, mapWriteError("updating set", err)
	}
	return r.GetByID(ctx, sessionID)
}

// RemoveSet drops one logged set and returns the refreshed session. The entry's done
// flag is left where it is — see refreshDone.
func (r *SessionRepo) RemoveSet(ctx context.Context, sessionID uuid.UUID, entryID, setID int64) (models.WorkoutSession, error) {
	if _, err := r.getSet(ctx, sessionID, entryID, setID); err != nil {
		return models.WorkoutSession{}, err
	}
	if _, err := r.pool.Exec(ctx,
		`DELETE FROM session_sets WHERE id = $1 AND session_movement_id = $2`, setID, entryID); err != nil {
		return models.WorkoutSession{}, fmt.Errorf("removing set: %w", err)
	}
	return r.GetByID(ctx, sessionID)
}

// PromoteToWorkout turns a performed free-form session into a reusable workout
// template: what was actually logged becomes the prescription, in the order performed.
// The session is then back-linked to the new workout, so it reads as the first instance
// of the template it produced and its movements count toward that workout's history.
//
// Only a free-form session can be promoted — a session already backed by a template
// would silently fork it, which is an edit of that workout, not a new one.
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
		sets, reps, load := prescribeFrom(m)
		entries = append(entries, models.WorkoutMovementInput{
			MovementID: m.MovementID,
			Sets:       sets,
			Reps:       reps,
			Load:       load,
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

// prescribeFrom turns what was performed into what to prescribe next time: as many sets
// as were logged, the reps most of them shared, and the load of the last one — the
// working weight, once warmups sit in the same list. An entry with no sets falls back to
// its target, which is the only thing known about it.
func prescribeFrom(m models.SessionMovement) (sets *int, reps *string, load *string) {
	if len(m.Sets) == 0 {
		return m.TargetSets, m.TargetReps, m.TargetLoad
	}

	count := len(m.Sets)
	sets = &count

	seen := map[int]int{}
	commonest, best := 0, 0
	for _, s := range m.Sets {
		if s.Reps == nil {
			continue
		}
		seen[*s.Reps]++
		if n := seen[*s.Reps]; n > best || (n == best && *s.Reps > commonest) {
			commonest, best = *s.Reps, n
		}
	}
	if best > 0 {
		text := strconv.Itoa(commonest)
		reps = &text
	}

	for i := len(m.Sets) - 1; i >= 0; i-- {
		if m.Sets[i].Load != nil {
			load = m.Sets[i].Load
			break
		}
	}
	return sets, reps, load
}

// refreshDone ticks an entry off at the moment its logged sets reach the target — the
// checkbox answering itself, since counting to three is not worth asking someone to do
// mid-set. An entry with no target is done on its first set: nothing was planned, so
// doing it at all is the whole of it.
//
// Ticking is an event at the boundary, not a standing derivation, which is why this
// tests for equality rather than "at least". Logging a fourth set of a planned three
// must not overrule someone who unticked the entry on purpose, and doing more than the
// plan is a fact about the session rather than a reason to re-decide anything.
func refreshDone(ctx context.Context, db execer, entryID int64) error {
	_, err := db.Exec(ctx, `
		UPDATE session_movements sm SET done = true
		WHERE sm.id = $1
			AND NOT sm.done
			AND (SELECT count(*) FROM session_sets ss WHERE ss.session_movement_id = sm.id)
				= COALESCE(sm.target_sets, 1)`, entryID)
	if err != nil {
		return mapWriteError("updating done", err)
	}
	return nil
}

// getMovementEntry fetches one logged entry scoped to its session, so an entry id
// from a different session is a clean 404 rather than a cross-session edit.
func (r *SessionRepo) getMovementEntry(ctx context.Context, sessionID uuid.UUID, entryID int64) (models.SessionMovement, error) {
	var sm models.SessionMovement
	err := r.pool.QueryRow(ctx, `
		SELECT id, movement_id, position, done, target_sets, target_reps, target_load, notes
		FROM session_movements WHERE id = $1 AND session_id = $2`, entryID, sessionID).Scan(
		&sm.ID, &sm.MovementID, &sm.Position, &sm.Done, &sm.TargetSets, &sm.TargetReps, &sm.TargetLoad, &sm.Notes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.SessionMovement{}, fmt.Errorf("session %s entry %d: %w", sessionID, entryID, ErrNotFound)
		}
		return models.SessionMovement{}, fmt.Errorf("fetching session movement: %w", err)
	}
	return sm, nil
}

// getSet fetches one set scoped through its entry to its session, for the same reason
// getMovementEntry scopes to the session: a set id belonging to someone else's entry is
// a 404, not an edit.
func (r *SessionRepo) getSet(ctx context.Context, sessionID uuid.UUID, entryID, setID int64) (models.SessionSet, error) {
	var s models.SessionSet
	err := r.pool.QueryRow(ctx, `
		SELECT ss.id, ss.position, ss.reps, ss.load, ss.hold_seconds, ss.set_kind, ss.notes, ss.logged_at
		FROM session_sets ss
		JOIN session_movements sm ON sm.id = ss.session_movement_id
		WHERE ss.id = $1 AND ss.session_movement_id = $2 AND sm.session_id = $3`,
		setID, entryID, sessionID).Scan(
		&s.ID, &s.Position, &s.Reps, &s.Load, &s.HoldSeconds, &s.SetKind, &s.Notes, &s.LoggedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.SessionSet{}, fmt.Errorf("session %s entry %d set %d: %w", sessionID, entryID, setID, ErrNotFound)
		}
		return models.SessionSet{}, fmt.Errorf("fetching set: %w", err)
	}
	return s, nil
}

// insertSessionMovementsInOrder inserts free-form entries at positions 1..n in array
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
		INSERT INTO session_movements (session_id, movement_id, position, done, target_sets, target_reps, target_load, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		sessionID, e.MovementID, position, e.Done, e.TargetSets, e.TargetReps, e.TargetLoad, e.Notes)
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

// targetRepsAsInt reads a rep count out of a target when it is a plain number, so the
// first set of a "3 × 8" comes pre-filled. A target of "8–10" or "AMRAP" yields nothing
// and the set is logged without a rep count rather than with a guess.
func targetRepsAsInt(target *string) *int {
	if target == nil {
		return nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(*target))
	if err != nil {
		return nil
	}
	return &n
}

// valueOrDefault substitutes a fallback for an empty string, for the NOT NULL text
// columns that carry a vocabulary rather than free prose.
func valueOrDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
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
