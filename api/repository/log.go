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

type LogRepo struct {
	pool *pgxpool.Pool
}

func NewLogRepo(pool *pgxpool.Pool) *LogRepo {
	return &LogRepo{pool: pool}
}

// logSelect is the shared read query; scanLogEntry stays in lockstep with its columns.
const logSelect = `SELECT id, entry_date, body, tags, mood, created_at, updated_at FROM fitness_log_entries`

func scanLogEntry(row pgx.Row) (models.FitnessLogEntry, error) {
	var e models.FitnessLogEntry
	var entryDate time.Time
	if err := row.Scan(&e.ID, &entryDate, &e.Body, &e.Tags, &e.Mood, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return models.FitnessLogEntry{}, err
	}
	e.EntryDate = entryDate.Format(dateLayout)
	// Never emit a JSON null for tags — the column stores '{}', but a scan of an
	// empty array can still land as nil, so normalize for the client.
	if e.Tags == nil {
		e.Tags = []string{}
	}
	return e, nil
}

// List returns entries matching the filter, newest first (ties broken by created_at
// so two entries on the same date keep a stable order). Filtering is in SQL so the
// CLI and web share one definition.
func (r *LogRepo) List(ctx context.Context, f models.FitnessLogEntryFilter) ([]models.FitnessLogEntry, error) {
	var (
		where []string
		args  []any
	)
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}

	if f.From != "" {
		from, err := parseDate(f.From)
		if err != nil {
			return nil, err
		}
		add("entry_date >= $%d", from)
	}
	if f.To != "" {
		to, err := parseDate(f.To)
		if err != nil {
			return nil, err
		}
		add("entry_date <= $%d", to)
	}
	if f.Tag != "" {
		add("$%d = ANY(tags)", f.Tag)
	}

	query := logSelect
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY entry_date DESC, created_at DESC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing log entries: %w", err)
	}
	defer rows.Close()

	entries := []models.FitnessLogEntry{}
	for rows.Next() {
		e, err := scanLogEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning log entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *LogRepo) GetByID(ctx context.Context, id uuid.UUID) (models.FitnessLogEntry, error) {
	e, err := scanLogEntry(r.pool.QueryRow(ctx, logSelect+" WHERE id = $1", id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.FitnessLogEntry{}, fmt.Errorf("log entry %s: %w", id, ErrNotFound)
		}
		return models.FitnessLogEntry{}, fmt.Errorf("fetching log entry: %w", err)
	}
	return e, nil
}

// Create appends an entry. entry_date defaults to today when omitted. The UUID7 id is
// minted here (time-ordered), matching workout_sessions.
func (r *LogRepo) Create(ctx context.Context, in models.FitnessLogEntryCreate) (models.FitnessLogEntry, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return models.FitnessLogEntry{}, fmt.Errorf("generating log entry id: %w", err)
	}

	entryDate := time.Now()
	if in.EntryDate != "" {
		entryDate, err = parseDate(in.EntryDate)
		if err != nil {
			return models.FitnessLogEntry{}, err
		}
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO fitness_log_entries (id, entry_date, body, tags, mood)
		VALUES ($1, $2, $3, $4, $5)`,
		id, entryDate, in.Body, normalizeArray(in.Tags), in.Mood)
	if err != nil {
		return models.FitnessLogEntry{}, mapWriteError("creating log entry", err)
	}
	return r.GetByID(ctx, id)
}

// Update applies a partial update: a nil field is left unchanged. Tags is replaced
// wholesale when a non-nil slice is supplied (an empty slice clears them).
func (r *LogRepo) Update(ctx context.Context, id uuid.UUID, in models.FitnessLogEntryUpdate) (models.FitnessLogEntry, error) {
	current, err := r.GetByID(ctx, id)
	if err != nil {
		return models.FitnessLogEntry{}, err
	}

	entryDate, err := parseDate(current.EntryDate)
	if err != nil {
		return models.FitnessLogEntry{}, err
	}
	if in.EntryDate != nil {
		entryDate, err = parseDate(*in.EntryDate)
		if err != nil {
			return models.FitnessLogEntry{}, err
		}
	}
	body := valueOr(in.Body, current.Body)
	mood := pickPtr(in.Mood, current.Mood)
	tags := current.Tags
	if in.Tags != nil {
		tags = *in.Tags
	}

	_, err = r.pool.Exec(ctx, `
		UPDATE fitness_log_entries SET
			entry_date = $1, body = $2, tags = $3, mood = $4, updated_at = now()
		WHERE id = $5`,
		entryDate, body, normalizeArray(tags), mood, id)
	if err != nil {
		return models.FitnessLogEntry{}, mapWriteError("updating log entry", err)
	}
	return r.GetByID(ctx, id)
}

func (r *LogRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM fitness_log_entries WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting log entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("log entry %s: %w", id, ErrNotFound)
	}
	return nil
}
