package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

func newTestSummaryService(t *testing.T) (*SummaryService, *fakeTransactionRepo, *fakeBudgetRepo, *fakeRecurringBillRepo, *fakePersonRepo, *fakeCategoryRepo) {
	t.Helper()
	loc := testKarachiLoc(t)
	txRepo := newFakeTransactionRepo()
	budgetRepo := newFakeBudgetRepo()
	billRepo := newFakeRecurringBillRepo()
	personRepo := newFakePersonRepo()
	debtRepo := newFakeDebtRepo()
	catRepo := newFakeCategoryRepo()

	budgetSvc := NewBudgetService(budgetRepo, catRepo, loc)
	billSvc := NewRecurringBillService(billRepo, catRepo, loc)
	personSvc := NewPersonService(personRepo, debtRepo)

	svc := NewSummaryService(txRepo, budgetSvc, billSvc, personSvc, loc)
	return svc, txRepo, budgetRepo, billRepo, personRepo, catRepo
}

func TestSummaryService_Get_CallsGenerateDue(t *testing.T) {
	svc, _, _, billRepo, _, _ := newTestSummaryService(t)
	const userID = "user-1"

	if _, err := svc.Get(context.Background(), userID, "2025-08"); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if billRepo.generateDueArgs.userID != userID {
		t.Errorf("GenerateDue was not called with userID %q (got %q) — Get() must trigger generation on every call", userID, billRepo.generateDueArgs.userID)
	}
}

func TestSummaryService_Get_ComputesIncomeExpenseNet(t *testing.T) {
	svc, txRepo, _, _, _, catRepo := newTestSummaryService(t)
	const userID = "user-1"
	loc := testKarachiLoc(t)

	expenseCat, _ := catRepo.Create(context.Background(), domain.Category{UserID: userID, Name: "Food", Kind: domain.KindExpense})
	incomeCat, _ := catRepo.Create(context.Background(), domain.Category{UserID: userID, Name: "Salary", Kind: domain.KindIncome})

	inMonth := time.Date(2025, time.August, 15, 12, 0, 0, 0, loc)
	outOfMonth := time.Date(2025, time.July, 15, 12, 0, 0, 0, loc)

	txRepo.Create(context.Background(), domain.Transaction{UserID: userID, Kind: domain.KindExpense, AmountPaisa: 30000, CategoryID: &expenseCat.ID, OccurredAt: inMonth})
	txRepo.Create(context.Background(), domain.Transaction{UserID: userID, Kind: domain.KindIncome, AmountPaisa: 200000, CategoryID: &incomeCat.ID, OccurredAt: inMonth})
	txRepo.Create(context.Background(), domain.Transaction{UserID: userID, Kind: domain.KindExpense, AmountPaisa: 99999, CategoryID: &expenseCat.ID, OccurredAt: outOfMonth})

	summary, err := svc.Get(context.Background(), userID, "2025-08")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if summary.IncomePaisa != 200000 {
		t.Errorf("IncomePaisa = %d; want 200000", summary.IncomePaisa)
	}
	if summary.ExpensePaisa != 30000 {
		t.Errorf("ExpensePaisa = %d; want 30000 (July transaction excluded)", summary.ExpensePaisa)
	}
	if summary.NetPaisa != 170000 {
		t.Errorf("NetPaisa = %d; want 170000", summary.NetPaisa)
	}
}

func TestSummaryService_Get_MoneyOnTheStreet(t *testing.T) {
	svc, _, _, _, personRepo, _ := newTestSummaryService(t)
	const userID = "user-1"

	theyOweMe, _ := personRepo.Create(context.Background(), domain.Person{UserID: userID, Name: "Usman"})
	personRepo.balances[theyOweMe.ID] = 50000

	iOweThem, _ := personRepo.Create(context.Background(), domain.Person{UserID: userID, Name: "Moiz"})
	personRepo.balances[iOweThem.ID] = -20000

	summary, err := svc.Get(context.Background(), userID, "2025-08")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if summary.OwedToMePaisa != 50000 {
		t.Errorf("OwedToMePaisa = %d; want 50000", summary.OwedToMePaisa)
	}
	if summary.IOwePaisa != 20000 {
		t.Errorf("IOwePaisa = %d; want 20000", summary.IOwePaisa)
	}
	if summary.NetOwedPaisa != 30000 {
		t.Errorf("NetOwedPaisa = %d; want 30000", summary.NetOwedPaisa)
	}
}

func TestSummaryService_Get_RejectsBadMonth(t *testing.T) {
	svc, _, _, _, _, _ := newTestSummaryService(t)
	_, err := svc.Get(context.Background(), "user-1", "not-a-month")
	if !errors.Is(err, domain.ErrInvalidMonth) {
		t.Errorf("Get() error = %v; want ErrInvalidMonth", err)
	}
}
