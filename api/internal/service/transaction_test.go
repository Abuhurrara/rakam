package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	// Hermetic tzdata: LoadLocation("Asia/Karachi") must not depend on the
	// host having zoneinfo installed, matching the embedding done in
	// cmd/api/main.go for the same reason.
	_ "time/tzdata"

	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/port"
)

type fakeTransactionRepo struct {
	transactions map[string]domain.Transaction
	nextID       int
	lastFilter   port.TransactionFilter
}

func newFakeTransactionRepo() *fakeTransactionRepo {
	return &fakeTransactionRepo{transactions: make(map[string]domain.Transaction)}
}

// List mirrors the real repo's filtering and its occurred_at desc, id desc
// tiebreak order closely enough to exercise the service's pagination and
// month-boundary logic honestly.
func (f *fakeTransactionRepo) List(ctx context.Context, userID string, filter port.TransactionFilter) ([]domain.Transaction, int, error) {
	f.lastFilter = filter

	var matched []domain.Transaction
	for _, t := range f.transactions {
		if t.UserID != userID {
			continue
		}
		if filter.From != nil && t.OccurredAt.Before(*filter.From) {
			continue
		}
		if filter.To != nil && !t.OccurredAt.Before(*filter.To) {
			continue
		}
		if filter.CategoryID != nil && (t.CategoryID == nil || *t.CategoryID != *filter.CategoryID) {
			continue
		}
		if filter.Query != nil {
			desc := ""
			if t.Description != nil {
				desc = *t.Description
			}
			if !strings.Contains(strings.ToLower(desc), strings.ToLower(*filter.Query)) {
				continue
			}
		}
		matched = append(matched, t)
	}

	sort.Slice(matched, func(i, j int) bool {
		if !matched[i].OccurredAt.Equal(matched[j].OccurredAt) {
			return matched[i].OccurredAt.After(matched[j].OccurredAt)
		}
		return matched[i].ID > matched[j].ID
	})

	total := len(matched)
	start := min(filter.Offset, total)
	end := min(start+filter.Limit, total)
	return matched[start:end], total, nil
}

func (f *fakeTransactionRepo) Get(ctx context.Context, userID, id string) (domain.Transaction, error) {
	t, ok := f.transactions[id]
	if !ok || t.UserID != userID {
		return domain.Transaction{}, domain.ErrNotFound
	}
	return t, nil
}

func (f *fakeTransactionRepo) Create(ctx context.Context, t domain.Transaction) (domain.Transaction, error) {
	f.nextID++
	t.ID = fmt.Sprintf("tx-%d", f.nextID)
	f.transactions[t.ID] = t
	return t, nil
}

// Update never touches RecurringBillID, matching postgres.TransactionRepo.
func (f *fakeTransactionRepo) Update(ctx context.Context, t domain.Transaction) (domain.Transaction, error) {
	existing, ok := f.transactions[t.ID]
	if !ok || existing.UserID != t.UserID {
		return domain.Transaction{}, domain.ErrNotFound
	}
	t.RecurringBillID = existing.RecurringBillID
	t.CreatedAt = existing.CreatedAt
	f.transactions[t.ID] = t
	return t, nil
}

func (f *fakeTransactionRepo) Delete(ctx context.Context, userID, id string) error {
	existing, ok := f.transactions[id]
	if !ok || existing.UserID != userID {
		return domain.ErrNotFound
	}
	delete(f.transactions, id)
	return nil
}

func (f *fakeTransactionRepo) SumByKind(ctx context.Context, userID string, from, to time.Time) (income, expense domain.Money, err error) {
	for _, t := range f.transactions {
		if t.UserID != userID || t.OccurredAt.Before(from) || !t.OccurredAt.Before(to) {
			continue
		}
		switch t.Kind {
		case domain.KindIncome:
			income += t.AmountPaisa
		case domain.KindExpense:
			expense += t.AmountPaisa
		}
	}
	return income, expense, nil
}

func TestTransactionService_Create(t *testing.T) {
	const (
		userID      = "user-1"
		otherUserID = "user-2"
	)

	catRepo := newFakeCategoryRepo()
	expenseCat, _ := catRepo.Create(context.Background(), domain.Category{UserID: userID, Name: "Food", Kind: domain.KindExpense})
	incomeCat, _ := catRepo.Create(context.Background(), domain.Category{UserID: userID, Name: "Salary", Kind: domain.KindIncome})
	otherUsersCat, _ := catRepo.Create(context.Background(), domain.Category{UserID: otherUserID, Name: "Food", Kind: domain.KindExpense})

	now := time.Now()

	tests := []struct {
		name    string
		tx      domain.Transaction
		wantErr error
	}{
		{
			name: "valid expense with matching category",
			tx:   domain.Transaction{UserID: userID, Kind: domain.KindExpense, AmountPaisa: 10000, CategoryID: &expenseCat.ID, OccurredAt: now},
		},
		{
			name:    "category kind mismatch",
			tx:      domain.Transaction{UserID: userID, Kind: domain.KindExpense, AmountPaisa: 10000, CategoryID: &incomeCat.ID, OccurredAt: now},
			wantErr: domain.ErrInvalidTransaction,
		},
		{
			name:    "category belongs to another user",
			tx:      domain.Transaction{UserID: userID, Kind: domain.KindExpense, AmountPaisa: 10000, CategoryID: &otherUsersCat.ID, OccurredAt: now},
			wantErr: domain.ErrNotFound,
		},
		{
			name:    "invalid kind",
			tx:      domain.Transaction{UserID: userID, Kind: "transfer", AmountPaisa: 10000, OccurredAt: now},
			wantErr: domain.ErrInvalidTransaction,
		},
		{
			name:    "zero amount",
			tx:      domain.Transaction{UserID: userID, Kind: domain.KindExpense, AmountPaisa: 0, OccurredAt: now},
			wantErr: domain.ErrInvalidAmount,
		},
		{
			name:    "occurred_at two days in the future",
			tx:      domain.Transaction{UserID: userID, Kind: domain.KindExpense, AmountPaisa: 10000, OccurredAt: now.Add(48 * time.Hour)},
			wantErr: domain.ErrInvalidTransaction,
		},
		{
			name: "occurred_at twelve hours in the future is fine",
			tx:   domain.Transaction{UserID: userID, Kind: domain.KindExpense, AmountPaisa: 10000, OccurredAt: now.Add(12 * time.Hour)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txRepo := newFakeTransactionRepo()
			svc := NewTransactionService(txRepo, catRepo, time.UTC)

			_, err := svc.Create(context.Background(), tt.tx)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Create() error = %v; want wrapping %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Create() unexpected error: %v", err)
			}
		})
	}
}

func TestTransactionService_List_KarachiMonthBoundary(t *testing.T) {
	karachi, err := time.LoadLocation("Asia/Karachi")
	if err != nil {
		t.Fatalf("loading Asia/Karachi: %v", err)
	}

	txRepo := newFakeTransactionRepo()
	catRepo := newFakeCategoryRepo()
	svc := NewTransactionService(txRepo, catRepo, karachi)

	// 2026-08-01T02:00:00+05:00 — 2am on the 1st, Karachi wall clock.
	occurredAt := time.Date(2026, time.August, 1, 2, 0, 0, 0, karachi)
	txRepo.transactions["tx-boundary"] = domain.Transaction{
		ID: "tx-boundary", UserID: "user-1", Kind: domain.KindExpense,
		AmountPaisa: 5000, OccurredAt: occurredAt,
	}

	august := "2026-08"
	result, err := svc.List(context.Background(), "user-1", ListParams{MonthStr: &august})
	if err != nil {
		t.Fatalf("List(2026-08) unexpected error: %v", err)
	}
	if result.Total != 1 || len(result.Transactions) != 1 {
		t.Fatalf("List(2026-08) = %d results, total %d; want 1 and 1", len(result.Transactions), result.Total)
	}

	july := "2026-07"
	result, err = svc.List(context.Background(), "user-1", ListParams{MonthStr: &july})
	if err != nil {
		t.Fatalf("List(2026-07) unexpected error: %v", err)
	}
	if result.Total != 0 || len(result.Transactions) != 0 {
		t.Fatalf("List(2026-07) = %d results, total %d; want 0 and 0 — the 2am Karachi transaction leaked into July", len(result.Transactions), result.Total)
	}
}

func TestTransactionService_List_PaginationClamping(t *testing.T) {
	txRepo := newFakeTransactionRepo()
	catRepo := newFakeCategoryRepo()
	svc := NewTransactionService(txRepo, catRepo, time.UTC)

	tests := []struct {
		name       string
		limit      int
		offset     int
		wantLimit  int
		wantOffset int
	}{
		{name: "unset limit defaults to 50", limit: 0, offset: 0, wantLimit: 50, wantOffset: 0},
		{name: "limit above hard max clamps to 200", limit: 500, offset: 0, wantLimit: 200, wantOffset: 0},
		{name: "negative offset clamps to 0", limit: 10, offset: -5, wantLimit: 10, wantOffset: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.List(context.Background(), "user-1", ListParams{Limit: tt.limit, Offset: tt.offset})
			if err != nil {
				t.Fatalf("List() unexpected error: %v", err)
			}
			if txRepo.lastFilter.Limit != tt.wantLimit || txRepo.lastFilter.Offset != tt.wantOffset {
				t.Fatalf("repo saw Limit=%d Offset=%d; want Limit=%d Offset=%d",
					txRepo.lastFilter.Limit, txRepo.lastFilter.Offset, tt.wantLimit, tt.wantOffset)
			}
		})
	}
}

// TestTransactionService_List_PaginationTiebreak covers rows sharing an
// identical occurred_at: without a stable secondary sort key, offset
// pagination over such rows can duplicate or skip them between pages.
func TestTransactionService_List_PaginationTiebreak(t *testing.T) {
	txRepo := newFakeTransactionRepo()
	catRepo := newFakeCategoryRepo()
	svc := NewTransactionService(txRepo, catRepo, time.UTC)

	same := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	txRepo.transactions["tx-1"] = domain.Transaction{ID: "tx-1", UserID: "user-1", Kind: domain.KindExpense, AmountPaisa: 100, OccurredAt: same}
	txRepo.transactions["tx-2"] = domain.Transaction{ID: "tx-2", UserID: "user-1", Kind: domain.KindExpense, AmountPaisa: 200, OccurredAt: same}

	result1, err := svc.List(context.Background(), "user-1", ListParams{Limit: 1, Offset: 0})
	if err != nil || result1.Total != 2 || len(result1.Transactions) != 1 {
		t.Fatalf("page1: results=%d total=%d err=%v; want 1 result, total 2, no error", len(result1.Transactions), result1.Total, err)
	}
	result2, err := svc.List(context.Background(), "user-1", ListParams{Limit: 1, Offset: 1})
	if err != nil || result2.Total != 2 || len(result2.Transactions) != 1 {
		t.Fatalf("page2: results=%d total=%d err=%v; want 1 result, total 2, no error", len(result2.Transactions), result2.Total, err)
	}

	page1ID, page2ID := result1.Transactions[0].ID, result2.Transactions[0].ID
	if page1ID == page2ID {
		t.Fatalf("page1 and page2 both returned %s; pagination duplicated a row across pages", page1ID)
	}
	seen := map[string]bool{page1ID: true, page2ID: true}
	if !seen["tx-1"] || !seen["tx-2"] {
		t.Fatalf("pages returned %v; want tx-1 and tx-2 covered exactly once between them", seen)
	}
}

func TestTransactionService_Update_PreservesRecurringBillID(t *testing.T) {
	txRepo := newFakeTransactionRepo()
	catRepo := newFakeCategoryRepo()
	svc := NewTransactionService(txRepo, catRepo, time.UTC)

	billID := "bill-1"
	txRepo.transactions["tx-1"] = domain.Transaction{
		ID: "tx-1", UserID: "user-1", Kind: domain.KindExpense,
		AmountPaisa: 5000, OccurredAt: time.Now(), RecurringBillID: &billID,
	}

	// Simulates a handler-built update request, which has no field for
	// recurring_bill_id and so leaves it at its zero value (nil).
	updated, err := svc.Update(context.Background(), domain.Transaction{
		ID: "tx-1", UserID: "user-1", Kind: domain.KindExpense,
		AmountPaisa: 7500, OccurredAt: time.Now(), Description: new("edited"),
	})
	if err != nil {
		t.Fatalf("Update() unexpected error: %v", err)
	}
	if updated.RecurringBillID == nil || *updated.RecurringBillID != billID {
		t.Fatalf("Update() RecurringBillID = %v; want %q preserved", updated.RecurringBillID, billID)
	}
}

func TestTransactionService_Update_NotFoundForOtherUser(t *testing.T) {
	txRepo := newFakeTransactionRepo()
	catRepo := newFakeCategoryRepo()
	svc := NewTransactionService(txRepo, catRepo, time.UTC)

	txRepo.transactions["tx-1"] = domain.Transaction{ID: "tx-1", UserID: "user-1", Kind: domain.KindExpense, AmountPaisa: 5000, OccurredAt: time.Now()}

	_, err := svc.Update(context.Background(), domain.Transaction{ID: "tx-1", UserID: "user-2", Kind: domain.KindExpense, AmountPaisa: 5000, OccurredAt: time.Now()})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Update() by wrong user error = %v; want ErrNotFound", err)
	}
}

func TestTransactionService_Delete_NotFoundForOtherUser(t *testing.T) {
	txRepo := newFakeTransactionRepo()
	catRepo := newFakeCategoryRepo()
	svc := NewTransactionService(txRepo, catRepo, time.UTC)

	txRepo.transactions["tx-1"] = domain.Transaction{ID: "tx-1", UserID: "user-1", Kind: domain.KindExpense, AmountPaisa: 5000, OccurredAt: time.Now()}

	err := svc.Delete(context.Background(), "user-2", "tx-1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Delete() by wrong user error = %v; want ErrNotFound", err)
	}
}