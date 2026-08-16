package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	_ "time/tzdata"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type fakeBudgetRepo struct {
	budgets map[string]domain.Budget
	nextID  int
}

func newFakeBudgetRepo() *fakeBudgetRepo {
	return &fakeBudgetRepo{budgets: make(map[string]domain.Budget)}
}

func (f *fakeBudgetRepo) ListWithSpent(ctx context.Context, userID string, month time.Time) ([]domain.BudgetWithSpent, error) {
	var result []domain.BudgetWithSpent
	for _, b := range f.budgets {
		if b.UserID != userID || !b.Month.Equal(month) {
			continue
		}
		budget := b
		result = append(result, domain.BudgetWithSpent{Budget: &budget})
	}
	return result, nil
}

func (f *fakeBudgetRepo) Upsert(ctx context.Context, b domain.Budget) (domain.Budget, error) {
	for id, existing := range f.budgets {
		if existing.UserID == b.UserID && existing.CategoryID == b.CategoryID && existing.Month.Equal(b.Month) {
			b.ID = id
			b.CreatedAt = existing.CreatedAt
			f.budgets[id] = b
			return b, nil
		}
	}
	f.nextID++
	b.ID = fmt.Sprintf("budget-%d", f.nextID)
	f.budgets[b.ID] = b
	return b, nil
}

func (f *fakeBudgetRepo) Delete(ctx context.Context, userID, id string) error {
	b, ok := f.budgets[id]
	if !ok || b.UserID != userID {
		return domain.ErrNotFound
	}
	delete(f.budgets, id)
	return nil
}

func testKarachiLoc(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Karachi")
	if err != nil {
		t.Fatalf("loading Asia/Karachi: %v", err)
	}
	return loc
}

func TestBudgetService_Upsert_RejectsZeroOrNegativeLimit(t *testing.T) {
	const userID = "user-1"
	catRepo := newFakeCategoryRepo()
	cat, _ := catRepo.Create(context.Background(), domain.Category{UserID: userID, Name: "Food", Kind: domain.KindExpense})
	svc := NewBudgetService(newFakeBudgetRepo(), catRepo, testKarachiLoc(t))

	for _, limit := range []domain.Money{0, -100} {
		_, err := svc.Upsert(context.Background(), userID, cat.ID, "2025-08", limit)
		if !errors.Is(err, domain.ErrInvalidAmount) {
			t.Errorf("Upsert(limit=%d) error = %v; want ErrInvalidAmount", limit, err)
		}
	}
}

func TestBudgetService_Upsert_RejectsIncomeCategory(t *testing.T) {
	const userID = "user-1"
	catRepo := newFakeCategoryRepo()
	cat, _ := catRepo.Create(context.Background(), domain.Category{UserID: userID, Name: "Salary", Kind: domain.KindIncome})
	svc := NewBudgetService(newFakeBudgetRepo(), catRepo, testKarachiLoc(t))

	_, err := svc.Upsert(context.Background(), userID, cat.ID, "2025-08", 50000)
	if !errors.Is(err, domain.ErrInvalidBudget) {
		t.Errorf("Upsert() error = %v; want ErrInvalidBudget", err)
	}
}

func TestBudgetService_Upsert_ValidBudgetPassesThrough(t *testing.T) {
	const userID = "user-1"
	catRepo := newFakeCategoryRepo()
	cat, _ := catRepo.Create(context.Background(), domain.Category{UserID: userID, Name: "Food", Kind: domain.KindExpense})
	svc := NewBudgetService(newFakeBudgetRepo(), catRepo, testKarachiLoc(t))

	created, err := svc.Upsert(context.Background(), userID, cat.ID, "2025-08", 50000)
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if created.ID == "" {
		t.Errorf("Upsert() did not assign an ID")
	}
}

func TestBudgetService_ListForMonth_RejectsBadMonthString(t *testing.T) {
	svc := NewBudgetService(newFakeBudgetRepo(), newFakeCategoryRepo(), testKarachiLoc(t))
	_, err := svc.ListForMonth(context.Background(), "user-1", "not-a-month")
	if !errors.Is(err, domain.ErrInvalidMonth) {
		t.Errorf("ListForMonth() error = %v; want ErrInvalidMonth", err)
	}
}
