package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

// TestBudgetRepo_ListWithSpent_CoalescesZeroSpend is fix 5's proof: a
// budgeted category with zero transactions must come back with
// SpentPaisa 0, not a scan error from a NULL sum.
func TestBudgetRepo_ListWithSpent_CoalescesZeroSpend(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	userID := testUser(ctx, t, pool)
	categoryID := testCategory(ctx, t, pool, userID, "Groceries", "expense")

	budgetRepo := NewBudgetRepo(pool)
	month := time.Date(2025, time.August, 1, 0, 0, 0, 0, time.UTC)
	if _, err := budgetRepo.Upsert(ctx, domain.Budget{UserID: userID, CategoryID: categoryID, Month: month, LimitPaisa: 500000}); err != nil {
		t.Fatalf("upserting budget: %v", err)
	}

	rows, err := budgetRepo.ListWithSpent(ctx, userID, month)
	if err != nil {
		t.Fatalf("ListWithSpent() error = %v", err)
	}
	for _, r := range rows {
		if r.Category.ID != categoryID {
			continue
		}
		if r.SpentPaisa != 0 {
			t.Errorf("SpentPaisa = %d; want 0 for a category with no transactions", r.SpentPaisa)
		}
		if r.Budget == nil || r.Budget.LimitPaisa != 500000 {
			t.Errorf("Budget = %+v; want limit 500000", r.Budget)
		}
		return
	}
	t.Fatalf("budgeted category %s not found in ListWithSpent result", categoryID)
}

// TestBudgetRepo_ListWithSpent_FiltersExpenseKindOnly is fix 4's proof: an
// income transaction filed under an expense category must not count toward
// spent.
func TestBudgetRepo_ListWithSpent_FiltersExpenseKindOnly(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	userID := testUser(ctx, t, pool)
	categoryID := testCategory(ctx, t, pool, userID, "Freelance Expenses", "expense")

	month := time.Date(2025, time.August, 1, 0, 0, 0, 0, time.UTC)
	occurredAt := time.Date(2025, time.August, 15, 12, 0, 0, 0, time.UTC)

	if _, err := pool.Exec(ctx, `
		insert into transactions (user_id, kind, amount_paisa, category_id, occurred_at)
		values ($1, 'income', 100000, $2, $3)
	`, userID, categoryID, occurredAt); err != nil {
		t.Fatalf("inserting income transaction: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into transactions (user_id, kind, amount_paisa, category_id, occurred_at)
		values ($1, 'expense', 30000, $2, $3)
	`, userID, categoryID, occurredAt); err != nil {
		t.Fatalf("inserting expense transaction: %v", err)
	}

	budgetRepo := NewBudgetRepo(pool)
	rows, err := budgetRepo.ListWithSpent(ctx, userID, month)
	if err != nil {
		t.Fatalf("ListWithSpent() error = %v", err)
	}
	for _, r := range rows {
		if r.Category.ID != categoryID {
			continue
		}
		if r.SpentPaisa != 30000 {
			t.Errorf("SpentPaisa = %d; want 30000 (expense only, income excluded)", r.SpentPaisa)
		}
		return
	}
	t.Fatalf("category %s not found in ListWithSpent result", categoryID)
}

// TestBudgetRepo_Upsert_ConflictUpdatesLimit is fix 6's proof: a second
// upsert for the same (user_id, category_id, month) updates the existing
// row's limit in place rather than a select-then-insert producing a
// duplicate or a constraint violation.
func TestBudgetRepo_Upsert_ConflictUpdatesLimit(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	userID := testUser(ctx, t, pool)
	categoryID := testCategory(ctx, t, pool, userID, "Transport", "expense")

	budgetRepo := NewBudgetRepo(pool)
	month := time.Date(2025, time.August, 1, 0, 0, 0, 0, time.UTC)

	first, err := budgetRepo.Upsert(ctx, domain.Budget{UserID: userID, CategoryID: categoryID, Month: month, LimitPaisa: 200000})
	if err != nil {
		t.Fatalf("first Upsert() error = %v", err)
	}

	second, err := budgetRepo.Upsert(ctx, domain.Budget{UserID: userID, CategoryID: categoryID, Month: month, LimitPaisa: 350000})
	if err != nil {
		t.Fatalf("second Upsert() error = %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("second Upsert() created a new row (ID %s) instead of updating %s", second.ID, first.ID)
	}
	if second.LimitPaisa != 350000 {
		t.Errorf("LimitPaisa after conflicting upsert = %d; want 350000", second.LimitPaisa)
	}

	var count int
	if err := pool.QueryRow(ctx, `select count(*) from budgets where user_id = $1 and category_id = $2 and month = $3`, userID, categoryID, month).Scan(&count); err != nil {
		t.Fatalf("counting budgets: %v", err)
	}
	if count != 1 {
		t.Errorf("budgets in database = %d; want exactly 1 after conflicting upsert", count)
	}
}
