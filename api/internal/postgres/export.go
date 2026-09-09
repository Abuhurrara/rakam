package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExportRepo struct{ pool *pgxpool.Pool }

func NewExportRepo(pool *pgxpool.Pool) *ExportRepo { return &ExportRepo{pool: pool} }

// A single SQL statement gives every collection the same database snapshot.
// Password hashes, sessions and internal request receipts are never exported.
func (r *ExportRepo) Snapshot(ctx context.Context, userID string) (domain.LedgerExport, error) {
	var result domain.LedgerExport
	err := r.pool.QueryRow(ctx, `select jsonb_build_object(
 'version', 1, 'currency', 'PKR', 'timezone', 'Asia/Karachi', 'exported_at', now(),
 'user', jsonb_build_object('id', u.id, 'email', u.email, 'name', u.name, 'created_at', u.created_at, 'updated_at', u.updated_at),
 'categories', (select coalesce(jsonb_agg(to_jsonb(c) order by c.id), '[]') from categories c where c.user_id = $1),
 'transactions', (select coalesce(jsonb_agg(to_jsonb(t) order by t.occurred_at, t.id), '[]') from transactions t where t.user_id = $1),
 'people', (select coalesce(jsonb_agg(to_jsonb(p) order by p.id), '[]') from people p where p.user_id = $1),
 'debt_entries', (select coalesce(jsonb_agg(to_jsonb(d) order by d.id), '[]') from debt_entries d where d.user_id = $1),
 'budgets', (select coalesce(jsonb_agg(to_jsonb(b) order by b.id), '[]') from budgets b where b.user_id = $1),
 'recurring_bills', (select coalesce(jsonb_agg(to_jsonb(b) order by b.id), '[]') from recurring_bills b where b.user_id = $1),
 'work_logs', (select coalesce(jsonb_agg(to_jsonb(w) order by w.month), '[]') from work_logs w where w.user_id = $1)
 ) from users u where u.id = $1`, userID).Scan(&result.Content)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.LedgerExport{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.LedgerExport{}, fmt.Errorf("reading export snapshot: %w", err)
	}
	return result, nil
}
