package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type RecurringBillRepo struct {
	pool *pgxpool.Pool
}

func NewRecurringBillRepo(pool *pgxpool.Pool) *RecurringBillRepo {
	return &RecurringBillRepo{pool: pool}
}

const recurringBillColumns = "id, user_id, name, amount_paisa, category_id, day_of_month, is_active, last_generated_month, created_at, updated_at"

func scanRecurringBill(s scanner) (domain.RecurringBill, error) {
	var b domain.RecurringBill
	err := s.Scan(&b.ID, &b.UserID, &b.Name, &b.AmountPaisa, &b.CategoryID, &b.DayOfMonth, &b.IsActive, &b.LastGeneratedMonth, &b.CreatedAt, &b.UpdatedAt)
	return b, err
}

func (r *RecurringBillRepo) List(ctx context.Context, userID string) ([]domain.RecurringBill, error) {
	rows, err := r.pool.Query(ctx, `
		select `+recurringBillColumns+`
		from recurring_bills
		where user_id = $1
		order by day_of_month, name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying recurring bills: %w", err)
	}
	defer rows.Close()

	var bills []domain.RecurringBill
	for rows.Next() {
		b, err := scanRecurringBill(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning recurring bill: %w", err)
		}
		bills = append(bills, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating recurring bills: %w", err)
	}
	return bills, nil
}

func (r *RecurringBillRepo) Get(ctx context.Context, userID, id string) (domain.RecurringBill, error) {
	b, err := scanRecurringBill(r.pool.QueryRow(ctx, `
		select `+recurringBillColumns+`
		from recurring_bills
		where id = $1 and user_id = $2
	`, id, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RecurringBill{}, domain.ErrNotFound
		}
		return domain.RecurringBill{}, fmt.Errorf("querying recurring bill: %w", err)
	}
	return b, nil
}

func (r *RecurringBillRepo) Create(ctx context.Context, b domain.RecurringBill) (domain.RecurringBill, error) {
	err := r.pool.QueryRow(ctx, `
		insert into recurring_bills (user_id, name, amount_paisa, category_id, day_of_month, is_active)
		values ($1, $2, $3, $4, $5, $6)
		returning id, created_at, updated_at
	`, b.UserID, b.Name, b.AmountPaisa, b.CategoryID, b.DayOfMonth, b.IsActive).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return domain.RecurringBill{}, fmt.Errorf("inserting recurring bill: %w", err)
	}
	return b, nil
}

// Update leaves last_generated_month untouched — it is not part of the
// editable request shape, and stamping it here would let a caller reset or
// fast-forward generation state by accident.
func (r *RecurringBillRepo) Update(ctx context.Context, b domain.RecurringBill) (domain.RecurringBill, error) {
	err := r.pool.QueryRow(ctx, `
		update recurring_bills
		set name = $1, amount_paisa = $2, category_id = $3, day_of_month = $4, is_active = $5, updated_at = now()
		where id = $6 and user_id = $7
		returning `+recurringBillColumns+`
	`, b.Name, b.AmountPaisa, b.CategoryID, b.DayOfMonth, b.IsActive, b.ID, b.UserID).Scan(
		&b.ID, &b.UserID, &b.Name, &b.AmountPaisa, &b.CategoryID, &b.DayOfMonth, &b.IsActive, &b.LastGeneratedMonth, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RecurringBill{}, domain.ErrNotFound
		}
		return domain.RecurringBill{}, fmt.Errorf("updating recurring bill: %w", err)
	}
	return b, nil
}

func (r *RecurringBillRepo) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `delete from recurring_bills where id = $1 and user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("deleting recurring bill: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type dueCandidate struct {
	id                 string
	name               string
	amountPaisa        domain.Money
	categoryID         *string
	dayOfMonth         int
	lastGeneratedMonth *time.Time
}

// GenerateDue is the race-safe core of recurring bill generation. It runs
// entirely inside one pgx.Tx:
//
//  1. Reads every active bill not yet caught up to currentMonth
//     (last_generated_month IS NULL or strictly before it — never a bill
//     already stamped for currentMonth or later, so this can only ever
//     advance a bill's stamp forward, never regenerate an earlier month).
//  2. For each, computes in Go how far it should advance: through
//     currentMonth if currentDay has reached its clamped day_of_month,
//     otherwise only through the last fully-elapsed month, leaving the
//     current month for a later call once its day arrives.
//  3. Claims that bill with a per-row conditional UPDATE keyed on the exact
//     last_generated_month value just read. This is a compare-and-swap, not
//     read-then-blindly-write: Postgres re-evaluates the WHERE clause
//     against the row's current committed state at UPDATE time, so if a
//     concurrent call already claimed this bill first, RowsAffected is 0
//     and this call skips it — it will be picked up on the bill's next
//     GenerateDue call. Only a successful claim proceeds to insert.
//  4. Inserts one transaction per month strictly between the bill's old and
//     new last_generated_month (inclusive of the new one) — this is what
//     catches up months that were missed entirely, e.g. a bill last
//     generated three months ago posts three transactions in one call.
func (r *RecurringBillRepo) GenerateDue(ctx context.Context, userID string, currentMonth time.Time, currentDay int) ([]domain.Transaction, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning generate-due transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		select id, name, amount_paisa, category_id, day_of_month, last_generated_month
		from recurring_bills
		where user_id = $1
		  and is_active = true
		  and (last_generated_month is null or last_generated_month < $2)
	`, userID, currentMonth)
	if err != nil {
		return nil, fmt.Errorf("querying due recurring bills: %w", err)
	}
	var candidates []dueCandidate
	for rows.Next() {
		var c dueCandidate
		if err := rows.Scan(&c.id, &c.name, &c.amountPaisa, &c.categoryID, &c.dayOfMonth, &c.lastGeneratedMonth); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scanning due recurring bill: %w", err)
		}
		candidates = append(candidates, c)
	}
	rowErr := rows.Err()
	rows.Close()
	if rowErr != nil {
		return nil, fmt.Errorf("iterating due recurring bills: %w", rowErr)
	}

	var created []domain.Transaction
	for _, c := range candidates {
		baselineMonth := currentMonth.AddDate(0, -1, 0)
		if c.lastGeneratedMonth != nil {
			baselineMonth = *c.lastGeneratedMonth
		}

		clampedCurrentDueDay := domain.ClampDayOfMonth(c.dayOfMonth, currentMonth.Year(), currentMonth.Month())
		newLast := currentMonth.AddDate(0, -1, 0)
		if currentDay >= clampedCurrentDueDay {
			newLast = currentMonth
		}
		if !newLast.After(baselineMonth) {
			continue
		}

		var tag pgconn.CommandTag
		if c.lastGeneratedMonth == nil {
			tag, err = tx.Exec(ctx, `
				update recurring_bills
				set last_generated_month = $1, updated_at = now()
				where id = $2 and user_id = $3 and last_generated_month is null
			`, newLast, c.id, userID)
		} else {
			tag, err = tx.Exec(ctx, `
				update recurring_bills
				set last_generated_month = $1, updated_at = now()
				where id = $2 and user_id = $3 and last_generated_month = $4
			`, newLast, c.id, userID, *c.lastGeneratedMonth)
		}
		if err != nil {
			return nil, fmt.Errorf("claiming recurring bill: %w", err)
		}
		if tag.RowsAffected() == 0 {
			continue
		}

		for m := baselineMonth.AddDate(0, 1, 0); !m.After(newLast); m = m.AddDate(0, 1, 0) {
			clampedDay := domain.ClampDayOfMonth(c.dayOfMonth, m.Year(), m.Month())
			occurredAt := time.Date(m.Year(), m.Month(), clampedDay, 0, 0, 0, 0, currentMonth.Location())
			billID := c.id
			description := c.name
			t := domain.Transaction{
				UserID:          userID,
				Kind:            domain.KindExpense,
				AmountPaisa:     c.amountPaisa,
				CategoryID:      c.categoryID,
				Description:     &description,
				OccurredAt:      occurredAt,
				RecurringBillID: &billID,
			}
			err := tx.QueryRow(ctx, `
				insert into transactions (user_id, kind, amount_paisa, category_id, description, occurred_at, recurring_bill_id)
				values ($1, $2, $3, $4, $5, $6, $7)
				returning id, created_at, updated_at
			`, t.UserID, t.Kind, t.AmountPaisa, t.CategoryID, t.Description, t.OccurredAt, t.RecurringBillID).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
			if err != nil {
				return nil, fmt.Errorf("inserting recurring bill transaction: %w", err)
			}
			created = append(created, t)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing generate-due transaction: %w", err)
	}
	return created, nil
}
