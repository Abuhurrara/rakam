package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

// CreateIdempotent commits the receipt and expense together. A competing insert
// waits on the unique key, then reads the committed receipt in the same request.
func (r *TransactionRepo) CreateIdempotent(ctx context.Context, t domain.Transaction, key string) (domain.Transaction, error) {
	canonical := t
	canonical.OccurredAt = t.OccurredAt.UTC()
	payload, err := json.Marshal(canonical)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("encoding save request: %w", err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(payload))
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("beginning save: %w", err)
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `insert into transaction_requests (user_id, request_key, request_hash)
 values ($1, $2, $3) on conflict (user_id, request_key) do nothing`, t.UserID, key, hash)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("claiming save: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var previousHash string
		var response []byte
		if err := tx.QueryRow(ctx, `select request_hash, response from transaction_requests where user_id = $1 and request_key = $2`, t.UserID, key).Scan(&previousHash, &response); err != nil {
			return domain.Transaction{}, fmt.Errorf("reading save receipt: %w", err)
		}
		if previousHash != hash {
			return domain.Transaction{}, domain.ErrIdempotencyConflict
		}
		var saved domain.Transaction
		if err := json.Unmarshal(response, &saved); err != nil {
			return domain.Transaction{}, fmt.Errorf("decoding save receipt: %w", err)
		}
		return saved, nil
	}
	err = tx.QueryRow(ctx, `insert into transactions (user_id, kind, amount_paisa, category_id, description, occurred_at, recurring_bill_id)
 values ($1, $2, $3, $4, $5, $6, $7) returning id, created_at, updated_at`,
		t.UserID, t.Kind, t.AmountPaisa, t.CategoryID, t.Description, t.OccurredAt, t.RecurringBillID).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("inserting expense: %w", err)
	}
	response, err := json.Marshal(t)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("encoding save receipt: %w", err)
	}
	if _, err := tx.Exec(ctx, `update transaction_requests set response = $3 where user_id = $1 and request_key = $2`, t.UserID, key, response); err != nil {
		return domain.Transaction{}, fmt.Errorf("recording save receipt: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Transaction{}, fmt.Errorf("committing save: %w", err)
	}
	return t, nil
}
