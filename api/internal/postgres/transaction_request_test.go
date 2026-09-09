package postgres

import (
	"context"
	"errors"
	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/port"
	"sync"
	"testing"
	"time"
)

func TestTransactionRepo_RetryReceipts(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	user := testUser(ctx, t, pool)
	other := testUser(ctx, t, pool)
	repo := NewTransactionRepo(pool)
	input := domain.Transaction{UserID: user, Kind: domain.KindExpense, AmountPaisa: 120000, OccurredAt: time.Date(2026, 9, 1, 2, 0, 0, 0, time.FixedZone("PKT", 18000))}
	const key = "same-request-123456789"
	var wg sync.WaitGroup
	results := make(chan domain.Transaction, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			saved, err := repo.CreateIdempotent(ctx, input, key)
			results <- saved
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	id := ""
	for result := range results {
		if id == "" {
			id = result.ID
		}
		if result.ID != id {
			t.Fatal("duplicate transaction IDs")
		}
	}
	for _, tc := range []struct {
		name     string
		input    domain.Transaction
		conflict bool
	}{
		{"same payload", input, false},
		{"same instant another offset", func() domain.Transaction { v := input; v.OccurredAt = v.OccurredAt.UTC(); return v }(), false},
		{"different amount", func() domain.Transaction { v := input; v.AmountPaisa++; return v }(), true},
		{"another user", func() domain.Transaction { v := input; v.UserID = other; return v }(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := repo.CreateIdempotent(ctx, tc.input, key)
			if errors.Is(err, domain.ErrIdempotencyConflict) != tc.conflict {
				t.Fatalf("error = %v", err)
			}
			if err != nil && !tc.conflict {
				t.Fatal(err)
			}
		})
	}
	if err := repo.Delete(ctx, user, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateIdempotent(ctx, input, key); err != nil {
		t.Fatal(err)
	}
	_, count, _, err := repo.List(ctx, user, port.TransactionFilter{Limit: 50})
	if err != nil || count != 0 {
		t.Fatalf("retry resurrected deleted expense: count %d, err %v", count, err)
	}
}

func TestTransactionRepo_PagedTotalsAndFilters(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	user := testUser(ctx, t, pool)
	other := testUser(ctx, t, pool)
	cat := testCategory(ctx, t, pool, user, "Food", "expense")
	_, err := pool.Exec(ctx, `insert into transactions(user_id,kind,amount_paisa,category_id,description,occurred_at)
 select $1,'expense',100,$2,'meal', '2026-09-01T02:00:00+05:00'::timestamptz from generate_series(1,205)`, user, cat)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewTransactionRepo(pool)
	for _, uid := range []string{user, other} {
		_, err = repo.Create(ctx, domain.Transaction{UserID: uid, Kind: domain.KindIncome, AmountPaisa: 999999, OccurredAt: time.Now()})
		if err != nil {
			t.Fatal(err)
		}
	}
	kind := domain.KindExpense
	q := "meal"
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.FixedZone("PKT", 18000))
	to := from.AddDate(0, 1, 0)
	for _, tc := range []struct {
		name         string
		offset, want int
	}{{"first", 0, 50}, {"last", 200, 5}, {"beyond", 250, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			rows, count, total, err := repo.List(ctx, user, port.TransactionFilter{From: &from, To: &to, Kind: &kind, CategoryID: &cat, Query: &q, Limit: 50, Offset: tc.offset})
			if err != nil || count != 205 || total != 20500 || len(rows) != tc.want {
				t.Fatalf("rows %d count %d total %d error %v", len(rows), count, total, err)
			}
		})
	}
}

func TestTransactionRepo_FailedSaveDoesNotClaimKey(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	user := testUser(ctx, t, pool)
	repo := NewTransactionRepo(pool)
	missing := "00000000-0000-0000-0000-000000000000"
	input := domain.Transaction{UserID: user, Kind: domain.KindExpense, AmountPaisa: 100, OccurredAt: time.Now(), CategoryID: &missing}
	if _, err := repo.CreateIdempotent(ctx, input, "rollback-request-key"); err == nil {
		t.Fatal("expected foreign key failure")
	}
	input.CategoryID = nil
	if _, err := repo.CreateIdempotent(ctx, input, "rollback-request-key"); err != nil {
		t.Fatalf("failed attempt poisoned the key: %v", err)
	}
}
