package postgres

import (
	"context"
	"strings"
	"testing"
)

func TestUserRepo_CreateStartsEmptyAndPasswordUpdateRevokesSessions(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewUserRepo(pool)
	email := "account-test-" + strings.ReplaceAll(t.Name(), "/", "-") + "@example.com"
	user, err := repo.Create(ctx, strings.ToLower(email), "Account Test", "bcrypt-hash")
	if err != nil {
		t.Fatalf("creating account: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `delete from users where id = $1`, user.ID) })
	if user.SessionVersion != 0 {
		t.Fatalf("initial session version = %d, want 0", user.SessionVersion)
	}

	var categories, people, transactions, budgets, bills int
	for _, check := range []struct {
		table string
		into  *int
	}{{"categories", &categories}, {"people", &people}, {"transactions", &transactions}, {"budgets", &budgets}, {"recurring_bills", &bills}} {
		query := "select count(*) from " + check.table + " where user_id = $1"
		if err := pool.QueryRow(ctx, query, user.ID).Scan(check.into); err != nil {
			t.Fatalf("counting %s for new account: %v", check.table, err)
		}
		if *check.into != 0 {
			t.Fatalf("new account has %d %s rows, want none", *check.into, check.table)
		}
	}

	if _, err := repo.Create(ctx, strings.ToUpper(email), "Duplicate", "another-hash"); err == nil {
		t.Fatal("case-insensitive duplicate email was accepted")
	}
	version, err := repo.UpdatePassword(ctx, user.ID, "new-bcrypt-hash")
	if err != nil {
		t.Fatalf("updating password: %v", err)
	}
	if version != 1 {
		t.Fatalf("session version after reset = %d, want 1", version)
	}
	got, err := repo.GetByEmail(ctx, strings.ToLower(email))
	if err != nil {
		t.Fatalf("reloading account: %v", err)
	}
	if got.PasswordHash != "new-bcrypt-hash" || got.SessionVersion != 1 {
		t.Fatalf("password update not persisted: hash=%q version=%d", got.PasswordHash, got.SessionVersion)
	}
}
