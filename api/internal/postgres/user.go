package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

// SoleUserID resolves the single seeded user's ID for the phase-1
// stand-in auth middleware. It goes away once real session auth exists.
func SoleUserID(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var id string
	err := pool.QueryRow(ctx, "select id from users limit 1").Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrNotFound
		}
		return "", fmt.Errorf("querying sole user: %w", err)
	}
	return id, nil
}
