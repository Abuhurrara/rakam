package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type CategoryRepo struct {
	pool *pgxpool.Pool
}

func NewCategoryRepo(pool *pgxpool.Pool) *CategoryRepo {
	return &CategoryRepo{pool: pool}
}

func (r *CategoryRepo) List(ctx context.Context, userID string) ([]domain.Category, error) {
	rows, err := r.pool.Query(ctx, `
		select id, user_id, name, kind, icon, color, sort_order, is_archived, created_at, updated_at
		from categories
		where user_id = $1
		order by sort_order, name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying categories: %w", err)
	}
	defer rows.Close()

	var categories []domain.Category
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Kind, &c.Icon, &c.Color, &c.SortOrder, &c.IsArchived, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning category: %w", err)
		}
		categories = append(categories, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating categories: %w", err)
	}
	return categories, nil
}

func (r *CategoryRepo) Create(ctx context.Context, c domain.Category) (domain.Category, error) {
	err := r.pool.QueryRow(ctx, `
		insert into categories (user_id, name, kind, icon, color, sort_order, is_archived)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning id, created_at, updated_at
	`, c.UserID, c.Name, c.Kind, c.Icon, c.Color, c.SortOrder, c.IsArchived).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return domain.Category{}, fmt.Errorf("inserting category: %w", err)
	}
	return c, nil
}

// Update leaves is_archived untouched — archiving happens only through Archive.
func (r *CategoryRepo) Update(ctx context.Context, c domain.Category) (domain.Category, error) {
	err := r.pool.QueryRow(ctx, `
		update categories
		set name = $1, kind = $2, icon = $3, color = $4, sort_order = $5, updated_at = now()
		where id = $6 and user_id = $7
		returning id, user_id, name, kind, icon, color, sort_order, is_archived, created_at, updated_at
	`, c.Name, c.Kind, c.Icon, c.Color, c.SortOrder, c.ID, c.UserID).Scan(
		&c.ID, &c.UserID, &c.Name, &c.Kind, &c.Icon, &c.Color, &c.SortOrder, &c.IsArchived, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Category{}, domain.ErrNotFound
		}
		return domain.Category{}, fmt.Errorf("updating category: %w", err)
	}
	return c, nil
}

func (r *CategoryRepo) Archive(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `
		update categories set is_archived = true, updated_at = now()
		where id = $1 and user_id = $2
	`, id, userID)
	if err != nil {
		return fmt.Errorf("archiving category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
