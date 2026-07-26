package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"meso/api/models"
)

type FeedbackRepo struct {
	pool *pgxpool.Pool
}

func NewFeedbackRepo(pool *pgxpool.Pool) *FeedbackRepo {
	return &FeedbackRepo{pool: pool}
}

// feedbackSelect is the shared read query; scanFeedback stays in lockstep.
const feedbackSelect = `SELECT id, status, body, context_path, created_at, updated_at FROM feedback`

func scanFeedback(row pgx.Row) (models.Feedback, error) {
	var f models.Feedback
	if err := row.Scan(&f.ID, &f.Status, &f.Body, &f.ContextPath, &f.CreatedAt, &f.UpdatedAt); err != nil {
		return models.Feedback{}, err
	}
	return f, nil
}

// List returns feedback matching the filter, newest first. Filtering is in SQL so
// there is one definition of it rather than one per client.
func (r *FeedbackRepo) List(ctx context.Context, f models.FeedbackFilter) ([]models.Feedback, error) {
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
		add("body ILIKE $%d", "%"+f.Search+"%")
	}

	query := feedbackSelect
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing feedback: %w", err)
	}
	defer rows.Close()

	items := []models.Feedback{}
	for rows.Next() {
		item, err := scanFeedback(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning feedback: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *FeedbackRepo) GetByID(ctx context.Context, id uuid.UUID) (models.Feedback, error) {
	f, err := scanFeedback(r.pool.QueryRow(ctx, feedbackSelect+" WHERE id = $1", id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Feedback{}, fmt.Errorf("feedback %s: %w", id, ErrNotFound)
		}
		return models.Feedback{}, fmt.Errorf("fetching feedback: %w", err)
	}
	return f, nil
}

// Create captures feedback. An empty body is ErrInvalid — a blank capture is never
// what was meant, and rejecting it here beats a row nobody can act on.
func (r *FeedbackRepo) Create(ctx context.Context, in models.FeedbackCreate) (models.Feedback, error) {
	body := strings.TrimSpace(in.Body)
	if body == "" {
		return models.Feedback{}, fmt.Errorf("feedback body is required: %w", ErrInvalid)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return models.Feedback{}, fmt.Errorf("generating feedback id: %w", err)
	}

	_, err = r.pool.Exec(ctx,
		`INSERT INTO feedback (id, body, context_path) VALUES ($1, $2, $3)`,
		id, body, in.ContextPath)
	if err != nil {
		return models.Feedback{}, mapWriteError("capturing feedback", err)
	}
	return r.GetByID(ctx, id)
}

// Update applies a partial update, leaving nil fields alone. An unknown id is
// ErrNotFound; a status outside the CHECK set is ErrInvalid.
func (r *FeedbackRepo) Update(ctx context.Context, id uuid.UUID, in models.FeedbackUpdate) (models.Feedback, error) {
	sets := []string{}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		sets = append(sets, fmt.Sprintf(clause, len(args)))
	}
	if in.Body != nil {
		add("body = $%d", *in.Body)
	}
	if in.ContextPath != nil {
		add("context_path = $%d", *in.ContextPath)
	}
	if in.Status != nil {
		add("status = $%d", *in.Status)
	}
	if len(sets) == 0 {
		return r.GetByID(ctx, id)
	}
	sets = append(sets, "updated_at = now()")

	args = append(args, id)
	query := fmt.Sprintf("UPDATE feedback SET %s WHERE id = $%d", strings.Join(sets, ", "), len(args))
	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return models.Feedback{}, mapWriteError("updating feedback", err)
	}
	if tag.RowsAffected() == 0 {
		return models.Feedback{}, fmt.Errorf("feedback %s: %w", id, ErrNotFound)
	}
	return r.GetByID(ctx, id)
}

func (r *FeedbackRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM feedback WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting feedback: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("feedback %s: %w", id, ErrNotFound)
	}
	return nil
}
