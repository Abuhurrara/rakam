package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

func TestCategoryRepo_ArchiveProtectsActiveBillAndRestoreKeepsRow(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	userID := testUser(ctx, t, pool)
	categoryID := testCategory(ctx, t, pool, userID, "Home", "expense")
	testBill(ctx, t, pool, userID, categoryID, 5)
	repo := NewCategoryRepo(pool)

	if err := repo.Archive(ctx, userID, categoryID); !errors.Is(err, domain.ErrCategoryInUse) {
		t.Fatalf("Archive() error = %v, want ErrCategoryInUse", err)
	}
	if _, err := pool.Exec(ctx, `update recurring_bills set is_active = false where user_id = $1`, userID); err != nil {
		t.Fatalf("deactivating test bill: %v", err)
	}
	if err := repo.Archive(ctx, userID, categoryID); err != nil {
		t.Fatalf("Archive() after bill deactivation: %v", err)
	}
	archived, err := repo.Get(ctx, userID, categoryID)
	if err != nil || !archived.IsArchived {
		t.Fatalf("category after archive = (%+v, %v), want archived row", archived, err)
	}
	restored, err := repo.Restore(ctx, userID, categoryID)
	if err != nil || restored.IsArchived {
		t.Fatalf("Restore() = (%+v, %v), want active row", restored, err)
	}
}

func TestCategoryRepo_SeedDefaultsOnlyForEmptyAccount(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewCategoryRepo(pool)

	emptyUser := testUser(ctx, t, pool)
	seeded, err := repo.SeedDefaultsIfEmpty(ctx, emptyUser)
	if err != nil || !seeded {
		t.Fatalf("SeedDefaultsIfEmpty(empty account) = (%t, %v), want true", seeded, err)
	}
	seeded, err = repo.SeedDefaultsIfEmpty(ctx, emptyUser)
	if err != nil || seeded {
		t.Fatalf("SeedDefaultsIfEmpty(seeded account) = (%t, %v), want false", seeded, err)
	}
	var count int
	if err := pool.QueryRow(ctx, `select count(*) from categories where user_id = $1`, emptyUser).Scan(&count); err != nil {
		t.Fatalf("counting default categories: %v", err)
	}
	if count != len(seedExpenseCategories)+len(seedIncomeCategories) {
		t.Fatalf("empty account got %d categories, want 15", count)
	}

	customUser := testUser(ctx, t, pool)
	testCategory(ctx, t, pool, customUser, "Custom", "expense")
	seeded, err = repo.SeedDefaultsIfEmpty(ctx, customUser)
	if err != nil || seeded {
		t.Fatalf("SeedDefaultsIfEmpty(custom account) = (%t, %v), want false", seeded, err)
	}
	if err := pool.QueryRow(ctx, `select count(*) from categories where user_id = $1`, customUser).Scan(&count); err != nil {
		t.Fatalf("counting custom categories: %v", err)
	}
	if count != 1 {
		t.Fatalf("custom account got %d categories, want exactly 1", count)
	}
}
