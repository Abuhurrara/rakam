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

type DebtRepo struct {
	pool *pgxpool.Pool
}

func NewDebtRepo(pool *pgxpool.Pool) *DebtRepo {
	return &DebtRepo{pool: pool}
}

// dbtx is satisfied by both *pgxpool.Pool and pgx.Tx, so settleUnsettled and
// settleErr run identically whether called directly against the pool
// (plain Settle) or inside a transaction (SettleWithTransaction, SettleAll).
type dbtx interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type scanner interface {
	Scan(dest ...any) error
}

const debtEntryColumns = "id, user_id, person_id, direction, amount_paisa, description, incurred_at, settled_at, created_at, updated_at"

func scanDebtEntry(s scanner) (domain.DebtEntry, error) {
	var e domain.DebtEntry
	err := s.Scan(&e.ID, &e.UserID, &e.PersonID, &e.Direction, &e.AmountPaisa, &e.Description, &e.IncurredAt, &e.SettledAt, &e.CreatedAt, &e.UpdatedAt)
	return e, err
}

// settleUnsettled stamps settled_at on the entry if and only if it is
// currently unsettled, returning the updated row. Zero rows (pgx.ErrNoRows)
// means the entry either doesn't exist or was already settled — settleErr
// disambiguates. Running this against a pgx.Tx (rather than the pool
// directly) is what makes SettleWithTransaction's settle decision atomic
// with the transaction it inserts: nothing about that decision is read
// before this statement runs.
func settleUnsettled(ctx context.Context, q dbtx, userID, id string) (domain.DebtEntry, error) {
	return scanDebtEntry(q.QueryRow(ctx, `
		update debt_entries
		set settled_at = now(), updated_at = now()
		where id = $1 and user_id = $2 and settled_at is null
		returning `+debtEntryColumns+`
	`, id, userID))
}

func settleErr(ctx context.Context, q dbtx, userID, id string) error {
	var exists bool
	if err := q.QueryRow(ctx, `select exists(select 1 from debt_entries where id = $1 and user_id = $2)`, id, userID).Scan(&exists); err != nil {
		return fmt.Errorf("checking debt entry existence: %w", err)
	}
	if exists {
		return domain.ErrAlreadySettled
	}
	return domain.ErrNotFound
}

func (r *DebtRepo) ListByPerson(ctx context.Context, userID, personID string) ([]domain.DebtEntry, error) {
	rows, err := r.pool.Query(ctx, `
		select `+debtEntryColumns+`
		from debt_entries
		where user_id = $1 and person_id = $2
		order by settled_at is null desc, incurred_at desc
	`, userID, personID)
	if err != nil {
		return nil, fmt.Errorf("querying debt entries: %w", err)
	}
	defer rows.Close()

	var entries []domain.DebtEntry
	for rows.Next() {
		e, err := scanDebtEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning debt entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating debt entries: %w", err)
	}
	return entries, nil
}

func (r *DebtRepo) Get(ctx context.Context, userID, id string) (domain.DebtEntry, error) {
	e, err := scanDebtEntry(r.pool.QueryRow(ctx, `
		select `+debtEntryColumns+`
		from debt_entries
		where id = $1 and user_id = $2
	`, id, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DebtEntry{}, domain.ErrNotFound
		}
		return domain.DebtEntry{}, fmt.Errorf("querying debt entry: %w", err)
	}
	return e, nil
}

func (r *DebtRepo) Create(ctx context.Context, e domain.DebtEntry) (domain.DebtEntry, error) {
	err := r.pool.QueryRow(ctx, `
		insert into debt_entries (user_id, person_id, direction, amount_paisa, description, incurred_at)
		values ($1, $2, $3, $4, $5, $6)
		returning id, created_at, updated_at
	`, e.UserID, e.PersonID, e.Direction, e.AmountPaisa, e.Description, e.IncurredAt).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return domain.DebtEntry{}, fmt.Errorf("inserting debt entry: %w", err)
	}
	return e, nil
}

func (r *DebtRepo) Settle(ctx context.Context, userID, id string) (domain.DebtEntry, error) {
	e, err := settleUnsettled(ctx, r.pool, userID, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DebtEntry{}, settleErr(ctx, r.pool, userID, id)
		}
		return domain.DebtEntry{}, fmt.Errorf("settling debt entry: %w", err)
	}
	return e, nil
}

// insertSettlementTransaction builds and inserts the transaction record for
// a just-settled entry, using only data the settling update itself
// returned — never a value read before that update ran.
func insertSettlementTransaction(ctx context.Context, q dbtx, userID string, e domain.DebtEntry, categoryID *string) (domain.Transaction, error) {
	description := e.Description
	t := domain.Transaction{
		UserID:      userID,
		Kind:        domain.KindForDirection(e.Direction),
		AmountPaisa: e.AmountPaisa,
		CategoryID:  categoryID,
		Description: &description,
		OccurredAt:  time.Now(),
	}
	err := q.QueryRow(ctx, `
		insert into transactions (user_id, kind, amount_paisa, category_id, description, occurred_at)
		values ($1, $2, $3, $4, $5, $6)
		returning id, created_at, updated_at
	`, t.UserID, t.Kind, t.AmountPaisa, t.CategoryID, t.Description, t.OccurredAt).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("inserting settlement transaction: %w", err)
	}
	return t, nil
}

func (r *DebtRepo) SettleWithTransaction(ctx context.Context, userID, id string, categoryID *string) (domain.DebtEntry, domain.Transaction, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.DebtEntry{}, domain.Transaction{}, fmt.Errorf("beginning settle transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	entry, err := settleUnsettled(ctx, tx, userID, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.DebtEntry{}, domain.Transaction{}, settleErr(ctx, tx, userID, id)
		}
		return domain.DebtEntry{}, domain.Transaction{}, fmt.Errorf("settling debt entry: %w", err)
	}

	created, err := insertSettlementTransaction(ctx, tx, userID, entry, categoryID)
	if err != nil {
		return domain.DebtEntry{}, domain.Transaction{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.DebtEntry{}, domain.Transaction{}, fmt.Errorf("committing settle transaction: %w", err)
	}
	return entry, created, nil
}

func (r *DebtRepo) SettleAll(ctx context.Context, userID, personID string, createTransaction bool, categoryID *string) ([]domain.DebtEntry, []domain.Transaction, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("beginning settle-all transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		update debt_entries
		set settled_at = now(), updated_at = now()
		where user_id = $1 and person_id = $2 and settled_at is null
		returning `+debtEntryColumns+`
	`, userID, personID)
	if err != nil {
		return nil, nil, fmt.Errorf("settling debt entries: %w", err)
	}
	var entries []domain.DebtEntry
	for rows.Next() {
		e, err := scanDebtEntry(rows)
		if err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("scanning settled debt entry: %w", err)
		}
		entries = append(entries, e)
	}
	rowErr := rows.Err()
	rows.Close()
	if rowErr != nil {
		return nil, nil, fmt.Errorf("iterating settled debt entries: %w", rowErr)
	}

	var created []domain.Transaction
	if createTransaction {
		for _, e := range entries {
			t, err := insertSettlementTransaction(ctx, tx, userID, e, categoryID)
			if err != nil {
				return nil, nil, err
			}
			created = append(created, t)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("committing settle-all transaction: %w", err)
	}
	return entries, created, nil
}

func (r *DebtRepo) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `delete from debt_entries where id = $1 and user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("deleting debt entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *DebtRepo) HasAny(ctx context.Context, userID, personID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		select exists(select 1 from debt_entries where user_id = $1 and person_id = $2)
	`, userID, personID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking debt entries: %w", err)
	}
	return exists, nil
}
