package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

func (r *CategoryRepo) Get(ctx context.Context, userID, id string) (domain.Category, error) {
	var c domain.Category
	err := r.pool.QueryRow(ctx, `
		select id, user_id, name, kind, icon, color, sort_order, is_archived, created_at, updated_at
		from categories
		where id = $1 and user_id = $2
	`, id, userID).Scan(&c.ID, &c.UserID, &c.Name, &c.Kind, &c.Icon, &c.Color, &c.SortOrder, &c.IsArchived, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Category{}, domain.ErrNotFound
		}
		return domain.Category{}, fmt.Errorf("querying category: %w", err)
	}
	return c, nil
}

func (r *CategoryRepo) Create(ctx context.Context, c domain.Category) (domain.Category, error) {
	err := r.pool.QueryRow(ctx, `
		insert into categories (user_id, name, kind, icon, color, sort_order, is_archived)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning id, created_at, updated_at
	`, c.UserID, c.Name, c.Kind, c.Icon, c.Color, c.SortOrder, c.IsArchived).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if isDuplicateCategoryError(err) {
			return domain.Category{}, domain.ErrDuplicateCategory
		}
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
		if isDuplicateCategoryError(err) {
			return domain.Category{}, domain.ErrDuplicateCategory
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Category{}, domain.ErrNotFound
		}
		return domain.Category{}, fmt.Errorf("updating category: %w", err)
	}
	return c, nil
}

func isDuplicateCategoryError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "categories_user_id_name_kind_key"
}

func (r *CategoryRepo) Archive(ctx context.Context, userID, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning category archive: %w", err)
	}
	defer tx.Rollback(ctx)

	var categoryID string
	if err := tx.QueryRow(ctx, `select id from categories where id = $1 and user_id = $2 for update`, id, userID).Scan(&categoryID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("locking category for archive: %w", err)
	}
	var usedByActiveBill bool
	if err := tx.QueryRow(ctx, `
		select exists (
			select 1 from recurring_bills
			where user_id = $1 and category_id = $2 and is_active = true
		)
	`, userID, id).Scan(&usedByActiveBill); err != nil {
		return fmt.Errorf("checking active recurring bills: %w", err)
	}
	if usedByActiveBill {
		return domain.ErrCategoryInUse
	}
	if _, err := tx.Exec(ctx, `update categories set is_archived = true, updated_at = now() where id = $1 and user_id = $2`, id, userID); err != nil {
		return fmt.Errorf("archiving category: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing category archive: %w", err)
	}
	return nil
}

func (r *CategoryRepo) Restore(ctx context.Context, userID, id string) (domain.Category, error) {
	var c domain.Category
	err := r.pool.QueryRow(ctx, `
		update categories
		set is_archived = false, updated_at = now()
		where id = $1 and user_id = $2
		returning id, user_id, name, kind, icon, color, sort_order, is_archived, created_at, updated_at
	`, id, userID).Scan(&c.ID, &c.UserID, &c.Name, &c.Kind, &c.Icon, &c.Color, &c.SortOrder, &c.IsArchived, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Category{}, domain.ErrNotFound
		}
		return domain.Category{}, fmt.Errorf("restoring category: %w", err)
	}
	return c, nil
}

// SeedDefaultsIfEmpty adds standard categories only when the account has no
// categories. It is intended for the trusted account-provisioning CLI.
func (r *CategoryRepo) SeedDefaultsIfEmpty(ctx context.Context, userID string) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("beginning default category backfill: %w", err)
	}
	defer tx.Rollback(ctx)

	var id string
	if err := tx.QueryRow(ctx, `select id from users where id = $1 for update`, userID).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, domain.ErrNotFound
		}
		return false, fmt.Errorf("locking account for category backfill: %w", err)
	}
	var hasCategories bool
	if err := tx.QueryRow(ctx, `select exists(select 1 from categories where user_id = $1)`, userID).Scan(&hasCategories); err != nil {
		return false, fmt.Errorf("checking account categories: %w", err)
	}
	if hasCategories {
		if err := tx.Commit(ctx); err != nil {
			return false, fmt.Errorf("committing skipped category backfill: %w", err)
		}
		return false, nil
	}
	if err := seedDefaultCategories(ctx, tx, userID); err != nil {
		return false, fmt.Errorf("adding default categories: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("committing default category backfill: %w", err)
	}
	return true, nil
}
