package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/port"
)

type TransactionRepo struct {
	pool *pgxpool.Pool
}

func NewTransactionRepo(pool *pgxpool.Pool) *TransactionRepo {
	return &TransactionRepo{pool: pool}
}

// escapeLike escapes ILIKE's wildcard characters in a user-supplied search
// term so a search for e.g. "50% off" or "lunch_break" doesn't match more
// than the literal text. Parameterization alone stops SQL injection but not
// this — % and _ stay meaningful inside the pattern even when passed as a
// bound argument.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// buildTransactionFilter returns the "where ..." clause (starting with the
// user scope) and its positional args, shared between the count query and
// the page query so they always agree on which rows match.
func buildTransactionFilter(userID string, filter port.TransactionFilter) (string, []any) {
	var b strings.Builder
	args := []any{userID}
	b.WriteString("where user_id = $1")

	if filter.From != nil {
		args = append(args, *filter.From)
		fmt.Fprintf(&b, " and occurred_at >= $%d", len(args))
	}
	if filter.To != nil {
		args = append(args, *filter.To)
		fmt.Fprintf(&b, " and occurred_at < $%d", len(args))
	}
	if filter.CategoryID != nil {
		args = append(args, *filter.CategoryID)
		fmt.Fprintf(&b, " and category_id = $%d", len(args))
	}
	if filter.Query != nil {
		args = append(args, "%"+escapeLike(*filter.Query)+"%")
		fmt.Fprintf(&b, ` and description ilike $%d escape '\'`, len(args))
	}

	return b.String(), args
}

func (r *TransactionRepo) List(ctx context.Context, userID string, filter port.TransactionFilter) ([]domain.Transaction, int, error) {
	whereClause, args := buildTransactionFilter(userID, filter)

	var total int
	if err := r.pool.QueryRow(ctx, "select count(*) from transactions "+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting transactions: %w", err)
	}

	pageArgs := append(append([]any{}, args...), filter.Limit, filter.Offset)
	query := fmt.Sprintf(`
		select id, user_id, kind, amount_paisa, category_id, description, occurred_at, recurring_bill_id, created_at, updated_at
		from transactions
		%s
		order by occurred_at desc, id desc
		limit $%d offset $%d
	`, whereClause, len(pageArgs)-1, len(pageArgs))

	rows, err := r.pool.Query(ctx, query, pageArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying transactions: %w", err)
	}
	defer rows.Close()

	var transactions []domain.Transaction
	for rows.Next() {
		var t domain.Transaction
		if err := rows.Scan(&t.ID, &t.UserID, &t.Kind, &t.AmountPaisa, &t.CategoryID, &t.Description, &t.OccurredAt, &t.RecurringBillID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scanning transaction: %w", err)
		}
		transactions = append(transactions, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating transactions: %w", err)
	}
	return transactions, total, nil
}

func (r *TransactionRepo) Get(ctx context.Context, userID, id string) (domain.Transaction, error) {
	var t domain.Transaction
	err := r.pool.QueryRow(ctx, `
		select id, user_id, kind, amount_paisa, category_id, description, occurred_at, recurring_bill_id, created_at, updated_at
		from transactions
		where id = $1 and user_id = $2
	`, id, userID).Scan(&t.ID, &t.UserID, &t.Kind, &t.AmountPaisa, &t.CategoryID, &t.Description, &t.OccurredAt, &t.RecurringBillID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Transaction{}, domain.ErrNotFound
		}
		return domain.Transaction{}, fmt.Errorf("querying transaction: %w", err)
	}
	return t, nil
}

func (r *TransactionRepo) Create(ctx context.Context, t domain.Transaction) (domain.Transaction, error) {
	err := r.pool.QueryRow(ctx, `
		insert into transactions (user_id, kind, amount_paisa, category_id, description, occurred_at, recurring_bill_id)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning id, created_at, updated_at
	`, t.UserID, t.Kind, t.AmountPaisa, t.CategoryID, t.Description, t.OccurredAt, t.RecurringBillID).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("inserting transaction: %w", err)
	}
	return t, nil
}

// Update leaves recurring_bill_id untouched — it is not part of the editable
// request shape, and stamping it here would sever the link phase 5's
// idempotent bill generation relies on.
func (r *TransactionRepo) Update(ctx context.Context, t domain.Transaction) (domain.Transaction, error) {
	err := r.pool.QueryRow(ctx, `
		update transactions
		set kind = $1, amount_paisa = $2, category_id = $3, description = $4, occurred_at = $5, updated_at = now()
		where id = $6 and user_id = $7
		returning id, user_id, kind, amount_paisa, category_id, description, occurred_at, recurring_bill_id, created_at, updated_at
	`, t.Kind, t.AmountPaisa, t.CategoryID, t.Description, t.OccurredAt, t.ID, t.UserID).Scan(
		&t.ID, &t.UserID, &t.Kind, &t.AmountPaisa, &t.CategoryID, &t.Description, &t.OccurredAt, &t.RecurringBillID, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Transaction{}, domain.ErrNotFound
		}
		return domain.Transaction{}, fmt.Errorf("updating transaction: %w", err)
	}
	return t, nil
}

func (r *TransactionRepo) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `
		delete from transactions where id = $1 and user_id = $2
	`, id, userID)
	if err != nil {
		return fmt.Errorf("deleting transaction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
