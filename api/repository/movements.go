package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"meso/api/models"
)

type MovementRepo struct {
	pool *pgxpool.Pool
}

func NewMovementRepo(pool *pgxpool.Pool) *MovementRepo {
	return &MovementRepo{pool: pool}
}

// movementCols is the ordered column list shared by every movement SELECT so the
// scan below stays in lockstep with the query.
const movementCols = `id, name, movement_kind, favorite, rating, tags, equipment,
	how_to, form_cues, common_faults, default_sets, default_reps, default_hold_seconds,
	sanskrit_name, measurable_rom, source_url, source_name, created_at, updated_at`

func scanMovement(row pgx.Row) (models.Movement, error) {
	var m models.Movement
	err := row.Scan(
		&m.ID, &m.Name, &m.MovementKind, &m.Favorite, &m.Rating, &m.Tags, &m.Equipment,
		&m.HowTo, &m.FormCues, &m.CommonFaults, &m.DefaultSets, &m.DefaultReps, &m.DefaultHoldSeconds,
		&m.SanskritName, &m.MeasurableROM, &m.SourceURL, &m.SourceName, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return models.Movement{}, err
	}
	// Never emit a JSON null for the array columns — the DB stores '{}', but a
	// scan of an empty array can still land as nil, so normalize for the client.
	if m.Tags == nil {
		m.Tags = []string{}
	}
	if m.Equipment == nil {
		m.Equipment = []string{}
	}
	m.Muscles = []models.MovementMuscle{}
	m.Related = []models.RelatedMovement{}
	return m, nil
}

// List returns movements matching the filter, ordered by name. Filtering is done
// in SQL (not client-side) so the list endpoint scales and the CLI/web share one
// definition of each filter. Muscles are attached in a single follow-up query to
// avoid an N+1 per movement.
func (r *MovementRepo) List(ctx context.Context, f models.MovementFilter) ([]models.Movement, error) {
	var (
		where []string
		args  []any
	)
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}

	if f.Kind != "" {
		add("movement_kind = $%d", f.Kind)
	}
	if f.Favorite != nil {
		add("favorite = $%d", *f.Favorite)
	}
	if f.Tag != "" {
		add("$%d = ANY(tags)", f.Tag)
	}
	if f.Equipment != "" {
		add("$%d = ANY(equipment)", f.Equipment)
	}
	if f.Search != "" {
		// Match the name or any tag, case-insensitively.
		args = append(args, "%"+f.Search+"%")
		where = append(where, fmt.Sprintf(
			"(name ILIKE $%d OR array_to_string(tags, ' ') ILIKE $%d)", len(args), len(args)))
	}
	if f.Muscle != "" {
		add("EXISTS (SELECT 1 FROM movement_muscles mm WHERE mm.movement_id = movements.id AND mm.muscle = $%d)", f.Muscle)
	}
	if f.Region != "" {
		add("EXISTS (SELECT 1 FROM movement_muscles mm JOIN muscles mu ON mu.name = mm.muscle "+
			"WHERE mm.movement_id = movements.id AND mu.region = $%d)", f.Region)
	}

	query := `SELECT ` + movementCols + ` FROM movements`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY name"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing movements: %w", err)
	}
	defer rows.Close()

	movements := []models.Movement{}
	byID := map[int64]int{} // movement id -> index in the slice, for muscle attach
	for rows.Next() {
		m, err := scanMovement(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning movement: %w", err)
		}
		byID[m.ID] = len(movements)
		movements = append(movements, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(movements) > 0 {
		if err := r.attachMuscles(ctx, movements, byID); err != nil {
			return nil, err
		}
	}
	return movements, nil
}

// attachMuscles loads every muscle tag for the given movements in one query and
// hangs each onto its movement, keeping ordering stable (region, muscle).
func (r *MovementRepo) attachMuscles(ctx context.Context, movements []models.Movement, byID map[int64]int) error {
	ids := make([]int64, 0, len(movements))
	for _, m := range movements {
		ids = append(ids, m.ID)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT mm.movement_id, mm.muscle, mu.region, mm.role
		FROM movement_muscles mm
		JOIN muscles mu ON mu.name = mm.muscle
		WHERE mm.movement_id = ANY($1)
		ORDER BY mu.region, mm.muscle`, ids)
	if err != nil {
		return fmt.Errorf("loading muscles: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var movementID int64
		var mm models.MovementMuscle
		if err := rows.Scan(&movementID, &mm.Muscle, &mm.Region, &mm.Role); err != nil {
			return fmt.Errorf("scanning muscle: %w", err)
		}
		idx := byID[movementID]
		movements[idx].Muscles = append(movements[idx].Muscles, mm)
	}
	return rows.Err()
}

func (r *MovementRepo) GetByID(ctx context.Context, id int64) (models.Movement, error) {
	m, err := scanMovement(r.pool.QueryRow(ctx, `SELECT `+movementCols+` FROM movements WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Movement{}, fmt.Errorf("movement %d: %w", id, ErrNotFound)
		}
		return models.Movement{}, fmt.Errorf("fetching movement: %w", err)
	}
	// attachMuscles fills in the muscle tags on the slice element in place, so wrap
	// the single movement in a one-element slice and read it back.
	list := []models.Movement{m}
	if err := r.attachMuscles(ctx, list, map[int64]int{m.ID: 0}); err != nil {
		return models.Movement{}, err
	}
	related, err := r.relatedFor(ctx, m.ID)
	if err != nil {
		return models.Movement{}, err
	}
	list[0].Related = related
	return list[0], nil
}

// relatedFor loads the movements this movement points at through relationships,
// each as a compact summary joined with the target's name/kind/favorite. Directional:
// only outgoing edges (movement_id = id) are returned, ordered by kind then name.
func (r *MovementRepo) relatedFor(ctx context.Context, id int64) ([]models.RelatedMovement, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.id, m.name, m.movement_kind, mr.relationship_kind, m.favorite
		FROM movement_relationships mr
		JOIN movements m ON m.id = mr.related_movement_id
		WHERE mr.movement_id = $1
		ORDER BY mr.relationship_kind, m.name`, id)
	if err != nil {
		return nil, fmt.Errorf("loading relationships: %w", err)
	}
	defer rows.Close()

	related := []models.RelatedMovement{}
	for rows.Next() {
		var rm models.RelatedMovement
		if err := rows.Scan(&rm.ID, &rm.Name, &rm.MovementKind, &rm.RelationshipKind, &rm.Favorite); err != nil {
			return nil, fmt.Errorf("scanning relationship: %w", err)
		}
		related = append(related, rm)
	}
	return related, rows.Err()
}

// AddRelationship records a directional relationship from movementID to relatedID
// of the given kind. Re-adding the same triple is a no-op (idempotent). Returns
// ErrNotFound if the source movement is absent, ErrReferenced if the target
// movement or the kind is unknown (FK), and rejects a self-relationship up front.
func (r *MovementRepo) AddRelationship(ctx context.Context, movementID, relatedID int64, kind string) error {
	if movementID == relatedID {
		return fmt.Errorf("a movement cannot relate to itself: %w", ErrInvalid)
	}
	if _, err := r.GetByID(ctx, movementID); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO movement_relationships (movement_id, related_movement_id, relationship_kind)
		VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, movementID, relatedID, kind)
	if err != nil {
		return mapWriteError("adding relationship", err)
	}
	return nil
}

// RemoveRelationship deletes a movement's relationship(s) to relatedID. An empty
// kind removes every relationship between the pair; a specific kind removes just
// that edge. Returns ErrNotFound when no matching edge exists.
func (r *MovementRepo) RemoveRelationship(ctx context.Context, movementID, relatedID int64, kind string) error {
	query := `DELETE FROM movement_relationships WHERE movement_id = $1 AND related_movement_id = $2`
	args := []any{movementID, relatedID}
	if kind != "" {
		query += ` AND relationship_kind = $3`
		args = append(args, kind)
	}
	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("removing relationship: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("relationship %d->%d: %w", movementID, relatedID, ErrNotFound)
	}
	return nil
}

func (r *MovementRepo) Create(ctx context.Context, in models.MovementCreate) (models.Movement, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return models.Movement{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after a successful commit is a no-op

	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO movements (name, movement_kind, favorite, rating, tags, equipment,
			how_to, form_cues, common_faults, default_sets, default_reps, default_hold_seconds,
			sanskrit_name, measurable_rom, source_url, source_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id`,
		in.Name, in.MovementKind, in.Favorite, in.Rating, normalizeArray(in.Tags), normalizeArray(in.Equipment),
		in.HowTo, in.FormCues, in.CommonFaults, in.DefaultSets, in.DefaultReps, in.DefaultHoldSeconds,
		in.SanskritName, in.MeasurableROM, in.SourceURL, in.SourceName).Scan(&id)
	if err != nil {
		return models.Movement{}, mapWriteError("creating movement", err)
	}

	if err := replaceMuscles(ctx, tx, id, in.Muscles); err != nil {
		return models.Movement{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return models.Movement{}, fmt.Errorf("commit: %w", err)
	}
	return r.GetByID(ctx, id)
}

func (r *MovementRepo) Update(ctx context.Context, id int64, in models.MovementUpdate) (models.Movement, error) {
	current, err := r.GetByID(ctx, id)
	if err != nil {
		return models.Movement{}, err
	}

	name := valueOr(in.Name, current.Name)
	kind := valueOr(in.MovementKind, current.MovementKind)
	favorite := valueOr(in.Favorite, current.Favorite)
	measurableROM := valueOr(in.MeasurableROM, current.MeasurableROM)
	howTo := valueOr(in.HowTo, current.HowTo)
	formCues := valueOr(in.FormCues, current.FormCues)
	commonFaults := valueOr(in.CommonFaults, current.CommonFaults)

	rating := current.Rating
	if in.Rating != nil {
		rating = in.Rating
	}
	defaultSets := pickPtr(in.DefaultSets, current.DefaultSets)
	defaultReps := pickPtr(in.DefaultReps, current.DefaultReps)
	defaultHold := pickPtr(in.DefaultHoldSeconds, current.DefaultHoldSeconds)
	sanskrit := pickPtr(in.SanskritName, current.SanskritName)
	sourceURL := pickPtr(in.SourceURL, current.SourceURL)
	sourceName := pickPtr(in.SourceName, current.SourceName)

	tags := current.Tags
	if in.Tags != nil {
		tags = *in.Tags
	}
	equipment := current.Equipment
	if in.Equipment != nil {
		equipment = *in.Equipment
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return models.Movement{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, `
		UPDATE movements SET
			name = $1, movement_kind = $2, favorite = $3, rating = $4, tags = $5, equipment = $6,
			how_to = $7, form_cues = $8, common_faults = $9, default_sets = $10, default_reps = $11,
			default_hold_seconds = $12, sanskrit_name = $13, measurable_rom = $14, source_url = $15,
			source_name = $16, updated_at = now()
		WHERE id = $17`,
		name, kind, favorite, rating, normalizeArray(tags), normalizeArray(equipment),
		howTo, formCues, commonFaults, defaultSets, defaultReps, defaultHold, sanskrit,
		measurableROM, sourceURL, sourceName, id)
	if err != nil {
		return models.Movement{}, mapWriteError("updating movement", err)
	}

	if in.Muscles != nil {
		if err := replaceMuscles(ctx, tx, id, *in.Muscles); err != nil {
			return models.Movement{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return models.Movement{}, fmt.Errorf("commit: %w", err)
	}
	return r.GetByID(ctx, id)
}

// Upsert inserts a movement or overwrites every column of an existing one matched
// by name — the idempotent write path for cmd/seed and the CLI import pass. Muscle
// tags are replaced wholesale. Run repeatedly, it converges each row to the
// supplied values.
func (r *MovementRepo) Upsert(ctx context.Context, in models.MovementCreate) (models.Movement, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return models.Movement{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO movements (name, movement_kind, favorite, rating, tags, equipment,
			how_to, form_cues, common_faults, default_sets, default_reps, default_hold_seconds,
			sanskrit_name, measurable_rom, source_url, source_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (name) DO UPDATE SET
			movement_kind = EXCLUDED.movement_kind, favorite = EXCLUDED.favorite, rating = EXCLUDED.rating,
			tags = EXCLUDED.tags, equipment = EXCLUDED.equipment, how_to = EXCLUDED.how_to,
			form_cues = EXCLUDED.form_cues, common_faults = EXCLUDED.common_faults,
			default_sets = EXCLUDED.default_sets, default_reps = EXCLUDED.default_reps,
			default_hold_seconds = EXCLUDED.default_hold_seconds, sanskrit_name = EXCLUDED.sanskrit_name,
			measurable_rom = EXCLUDED.measurable_rom, source_url = EXCLUDED.source_url,
			source_name = EXCLUDED.source_name, updated_at = now()
		RETURNING id`,
		in.Name, in.MovementKind, in.Favorite, in.Rating, normalizeArray(in.Tags), normalizeArray(in.Equipment),
		in.HowTo, in.FormCues, in.CommonFaults, in.DefaultSets, in.DefaultReps, in.DefaultHoldSeconds,
		in.SanskritName, in.MeasurableROM, in.SourceURL, in.SourceName).Scan(&id)
	if err != nil {
		return models.Movement{}, mapWriteError("upserting movement", err)
	}
	if err := replaceMuscles(ctx, tx, id, in.Muscles); err != nil {
		return models.Movement{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return models.Movement{}, fmt.Errorf("commit: %w", err)
	}
	return r.GetByID(ctx, id)
}

func (r *MovementRepo) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM movements WHERE id = $1`, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return fmt.Errorf("movement %d: %w", id, ErrReferenced)
		}
		return fmt.Errorf("deleting movement: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("movement %d: %w", id, ErrNotFound)
	}
	return nil
}

// ListMuscles returns the muscle lookup, ordered by region then name — the
// vocabulary the UI offers when tagging a movement.
func (r *MovementRepo) ListMuscles(ctx context.Context) ([]models.Muscle, error) {
	rows, err := r.pool.Query(ctx, `SELECT name, region FROM muscles ORDER BY region, name`)
	if err != nil {
		return nil, fmt.Errorf("listing muscles: %w", err)
	}
	defer rows.Close()

	muscles := []models.Muscle{}
	for rows.Next() {
		var m models.Muscle
		if err := rows.Scan(&m.Name, &m.Region); err != nil {
			return nil, fmt.Errorf("scanning muscle: %w", err)
		}
		muscles = append(muscles, m)
	}
	return muscles, rows.Err()
}

// replaceMuscles clears a movement's muscle tags and inserts the supplied set,
// inside the caller's transaction. A movement with no tags is valid (an empty
// slice clears them).
func replaceMuscles(ctx context.Context, tx pgx.Tx, movementID int64, muscles []models.MovementMuscleInput) error {
	if _, err := tx.Exec(ctx, `DELETE FROM movement_muscles WHERE movement_id = $1`, movementID); err != nil {
		return fmt.Errorf("clearing muscles: %w", err)
	}
	for _, mm := range muscles {
		_, err := tx.Exec(ctx,
			`INSERT INTO movement_muscles (movement_id, muscle, role) VALUES ($1, $2, $3)
			 ON CONFLICT DO NOTHING`,
			movementID, mm.Muscle, mm.Role)
		if err != nil {
			return mapWriteError("attaching muscle", err)
		}
	}
	return nil
}

// mapWriteError translates the Postgres constraint violations we care about into
// the repository's sentinel errors: 23505 (unique) -> ErrConflict, 23503 (FK, e.g.
// an unknown movement_kind or muscle) -> ErrReferenced. Everything else is wrapped.
func mapWriteError(op string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return fmt.Errorf("%s: %w", op, ErrConflict)
		case "23503":
			return fmt.Errorf("%s: %w", op, ErrReferenced)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

// normalizeArray maps a nil slice to an empty one so it stores as '{}' rather than
// NULL, keeping the NOT NULL array columns happy and reads branch-free.
func normalizeArray(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// valueOr returns *p when set, else the fallback — for non-null update fields.
func valueOr[T any](p *T, fallback T) T {
	if p != nil {
		return *p
	}
	return fallback
}

// pickPtr returns the incoming pointer when the field was supplied, else the
// current value — for nullable update fields where nil means "leave unchanged".
func pickPtr[T any](incoming, current *T) *T {
	if incoming != nil {
		return incoming
	}
	return current
}
