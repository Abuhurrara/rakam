package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type fakeRecurringBillRepo struct {
	bills  map[string]domain.RecurringBill
	nextID int

	// generateDueArgs records the arguments GenerateDue was last called
	// with, so tests can assert the service resolved "today"/"current
	// month" through the injected *time.Location rather than UTC.
	generateDueArgs struct {
		userID       string
		currentMonth time.Time
		currentDay   int
	}
}

func newFakeRecurringBillRepo() *fakeRecurringBillRepo {
	return &fakeRecurringBillRepo{bills: make(map[string]domain.RecurringBill)}
}

func (f *fakeRecurringBillRepo) List(ctx context.Context, userID string) ([]domain.RecurringBill, error) {
	var result []domain.RecurringBill
	for _, b := range f.bills {
		if b.UserID == userID {
			result = append(result, b)
		}
	}
	return result, nil
}

func (f *fakeRecurringBillRepo) Get(ctx context.Context, userID, id string) (domain.RecurringBill, error) {
	b, ok := f.bills[id]
	if !ok || b.UserID != userID {
		return domain.RecurringBill{}, domain.ErrNotFound
	}
	return b, nil
}

func (f *fakeRecurringBillRepo) Create(ctx context.Context, b domain.RecurringBill) (domain.RecurringBill, error) {
	f.nextID++
	b.ID = fmt.Sprintf("bill-%d", f.nextID)
	f.bills[b.ID] = b
	return b, nil
}

func (f *fakeRecurringBillRepo) Update(ctx context.Context, b domain.RecurringBill) (domain.RecurringBill, error) {
	existing, ok := f.bills[b.ID]
	if !ok || existing.UserID != b.UserID {
		return domain.RecurringBill{}, domain.ErrNotFound
	}
	b.LastGeneratedMonth = existing.LastGeneratedMonth
	f.bills[b.ID] = b
	return b, nil
}

func (f *fakeRecurringBillRepo) Delete(ctx context.Context, userID, id string) error {
	b, ok := f.bills[id]
	if !ok || b.UserID != userID {
		return domain.ErrNotFound
	}
	delete(f.bills, id)
	return nil
}

func (f *fakeRecurringBillRepo) GenerateDue(ctx context.Context, userID string, currentMonth time.Time, currentDay int) ([]domain.Transaction, error) {
	f.generateDueArgs.userID = userID
	f.generateDueArgs.currentMonth = currentMonth
	f.generateDueArgs.currentDay = currentDay
	return nil, nil
}

func TestRecurringBillService_Create_RejectsZeroOrNegativeAmount(t *testing.T) {
	const userID = "user-1"
	svc := NewRecurringBillService(newFakeRecurringBillRepo(), newFakeCategoryRepo(), testKarachiLoc(t))

	for _, amount := range []domain.Money{0, -100} {
		_, err := svc.Create(context.Background(), domain.RecurringBill{UserID: userID, Name: "Rent", AmountPaisa: amount, DayOfMonth: 1, IsActive: true})
		if !errors.Is(err, domain.ErrInvalidAmount) {
			t.Errorf("Create(amount=%d) error = %v; want ErrInvalidAmount", amount, err)
		}
	}
}

func TestRecurringBillService_Create_RejectsDayOutOfRange(t *testing.T) {
	const userID = "user-1"
	svc := NewRecurringBillService(newFakeRecurringBillRepo(), newFakeCategoryRepo(), testKarachiLoc(t))

	for _, day := range []int{0, 32, -1} {
		_, err := svc.Create(context.Background(), domain.RecurringBill{UserID: userID, Name: "Rent", AmountPaisa: 50000, DayOfMonth: day, IsActive: true})
		if !errors.Is(err, domain.ErrInvalidRecurringBill) {
			t.Errorf("Create(day=%d) error = %v; want ErrInvalidRecurringBill", day, err)
		}
	}
}

func TestRecurringBillService_Create_RejectsIncomeCategory(t *testing.T) {
	const userID = "user-1"
	catRepo := newFakeCategoryRepo()
	cat, _ := catRepo.Create(context.Background(), domain.Category{UserID: userID, Name: "Salary", Kind: domain.KindIncome})
	svc := NewRecurringBillService(newFakeRecurringBillRepo(), catRepo, testKarachiLoc(t))

	catID := cat.ID
	_, err := svc.Create(context.Background(), domain.RecurringBill{UserID: userID, Name: "Rent", AmountPaisa: 50000, CategoryID: &catID, DayOfMonth: 5, IsActive: true})
	if !errors.Is(err, domain.ErrInvalidRecurringBill) {
		t.Errorf("Create() error = %v; want ErrInvalidRecurringBill", err)
	}
}

func TestRecurringBillService_Create_ValidBillPassesThrough(t *testing.T) {
	const userID = "user-1"
	catRepo := newFakeCategoryRepo()
	cat, _ := catRepo.Create(context.Background(), domain.Category{UserID: userID, Name: "Bills", Kind: domain.KindExpense})
	svc := NewRecurringBillService(newFakeRecurringBillRepo(), catRepo, testKarachiLoc(t))

	catID := cat.ID
	created, err := svc.Create(context.Background(), domain.RecurringBill{UserID: userID, Name: "Rent", AmountPaisa: 50000, CategoryID: &catID, DayOfMonth: 5, IsActive: true})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" {
		t.Errorf("Create() did not assign an ID")
	}
}

// TestRecurringBillService_GenerateDue_ResolvesKarachiNow is fix 3's unit
// proof at the service layer: the current month/day passed down to the repo
// must come from the injected Asia/Karachi location, not UTC or server
// local time. It asserts against a value independently computed the same
// way, which would only agree with a UTC-based computation by coincidence
// (never, since Karachi is UTC+5 and this assertion runs in whatever
// timezone the test process happens to run in).
func TestRecurringBillService_GenerateDue_ResolvesKarachiNow(t *testing.T) {
	const userID = "user-1"
	loc := testKarachiLoc(t)
	repo := newFakeRecurringBillRepo()
	svc := NewRecurringBillService(repo, newFakeCategoryRepo(), loc)

	if _, err := svc.GenerateDue(context.Background(), userID); err != nil {
		t.Fatalf("GenerateDue() error = %v", err)
	}

	wantNow := time.Now().In(loc)
	wantMonth := time.Date(wantNow.Year(), wantNow.Month(), 1, 0, 0, 0, 0, loc)

	if repo.generateDueArgs.userID != userID {
		t.Errorf("GenerateDue userID = %q; want %q", repo.generateDueArgs.userID, userID)
	}
	if !repo.generateDueArgs.currentMonth.Equal(wantMonth) {
		t.Errorf("GenerateDue currentMonth = %v; want %v", repo.generateDueArgs.currentMonth, wantMonth)
	}
	if repo.generateDueArgs.currentDay != wantNow.Day() {
		t.Errorf("GenerateDue currentDay = %d; want %d", repo.generateDueArgs.currentDay, wantNow.Day())
	}
	if repo.generateDueArgs.currentMonth.Location().String() != loc.String() {
		t.Errorf("GenerateDue currentMonth location = %v; want %v", repo.generateDueArgs.currentMonth.Location(), loc)
	}
}
