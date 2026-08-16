package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type BudgetRepo struct {
	pool *pgxpool.Pool
}

func NewBudgetRepo(pool *pgxpool.Pool) *BudgetRepo {
	return &BudgetRepo{pool: pool}
}

// ListWithSpent returns every non-archived expense category for userID, left
// joined to its budget for month (nil if unset) and to expense transactions
// in that category for [month, month+1mo), coalesced to 0 so a category with
// no spending never comes back as NULL.
func (r *BudgetRepo) ListWithSpent(ctx context.Context, userID string, month time.Time) ([]domain.BudgetWithSpent, error) {
	next := month.AddDate(0, 1, 0)

	rows, err := r.pool.Query(ctx, `
		select c.id, c.user_id, c.name, c.kind, c.icon, c.color, c.sort_order, c.is_archived, c.created_at, c.updated_at,
		       b.id, b.month, b.limit_paisa, b.created_at, b.updated_at,
		       coalesce(sum(t.amount_paisa) filter (
		         where t.kind = 'expense' and t.occurred_at >= $2 and t.occurred_at < $3
		       ), 0) as spent_paisa
		from categories c
		left join budgets b on b.category_id = c.id and b.user_id = c.user_id and b.month = $2::date
		left join transactions t on t.category_id = c.id and t.user_id = c.user_id
		where c.user_id = $1 and c.kind = 'expense' and c.is_archived = false
		group by c.id, b.id
		order by c.sort_order
	`, userID, month, next)
	if err != nil {
		return nil, fmt.Errorf("querying budgets: %w", err)
	}
	defer rows.Close()

	var result []domain.BudgetWithSpent
	for rows.Next() {
		var bws domain.BudgetWithSpent
		var budgetID *string
		var budgetMonth *time.Time
		var limitPaisa *domain.Money
		var budgetCreatedAt, budgetUpdatedAt *time.Time

		if err := rows.Scan(
			&bws.Category.ID, &bws.Category.UserID, &bws.Category.Name, &bws.Category.Kind, &bws.Category.Icon, &bws.Category.Color, &bws.Category.SortOrder, &bws.Category.IsArchived, &bws.Category.CreatedAt, &bws.Category.UpdatedAt,
			&budgetID, &budgetMonth, &limitPaisa, &budgetCreatedAt, &budgetUpdatedAt,
			&bws.SpentPaisa,
		); err != nil {
			return nil, fmt.Errorf("scanning budget row: %w", err)
		}

		if budgetID != nil {
			bws.Budget = &domain.Budget{
				ID:         *budgetID,
				UserID:     userID,
				CategoryID: bws.Category.ID,
				Month:      *budgetMonth,
				LimitPaisa: *limitPaisa,
				CreatedAt:  *budgetCreatedAt,
				UpdatedAt:  *budgetUpdatedAt,
			}
		}
		result = append(result, bws)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating budgets: %w", err)
	}
	return result, nil
}

// Upsert inserts or, on a (user_id, category_id, month) conflict, updates
// the existing row's limit — never a select-then-insert.
func (r *BudgetRepo) Upsert(ctx context.Context, b domain.Budget) (domain.Budget, error) {
	err := r.pool.QueryRow(ctx, `
		insert into budgets (user_id, category_id, month, limit_paisa)
		values ($1, $2, $3, $4)
		on conflict (user_id, category_id, month)
		do update set limit_paisa = excluded.limit_paisa, updated_at = now()
		returning id, created_at, updated_at
	`, b.UserID, b.CategoryID, b.Month, b.LimitPaisa).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return domain.Budget{}, fmt.Errorf("upserting budget: %w", err)
	}
	return b, nil
}

func (r *BudgetRepo) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `delete from budgets where id = $1 and user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("deleting budget: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
