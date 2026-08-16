package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool connects to TEST_DATABASE_URL, skipping the test if it isn't
// set — the convention every real-Postgres test in this package follows.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connecting to test database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// testUser inserts a throwaway user, cleaned up when the test ends.
func testUser(ctx context.Context, t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	email := fmt.Sprintf("repo-test-%d@example.com", time.Now().UnixNano())
	var userID string
	if err := pool.QueryRow(ctx, `
		insert into users (email, password_hash, name)
		values ($1, 'x', 'Test User')
		returning id
	`, email).Scan(&userID); err != nil {
		t.Fatalf("inserting test user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `delete from users where id = $1`, userID)
	})
	return userID
}

// testCategory inserts a category of the given kind for userID.
func testCategory(ctx context.Context, t *testing.T, pool *pgxpool.Pool, userID, name, kind string) string {
	t.Helper()
	var categoryID string
	if err := pool.QueryRow(ctx, `
		insert into categories (user_id, name, kind, icon, color)
		values ($1, $2, $3, '🏷', '#000000')
		returning id
	`, userID, name, kind).Scan(&categoryID); err != nil {
		t.Fatalf("inserting test category: %v", err)
	}
	return categoryID
}
