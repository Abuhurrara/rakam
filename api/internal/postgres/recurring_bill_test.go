package postgres

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

func testBill(ctx context.Context, t *testing.T, pool *pgxpool.Pool, userID, categoryID string, dayOfMonth int) domain.RecurringBill {
	t.Helper()
	catID := categoryID
	b, err := NewRecurringBillRepo(pool).Create(ctx, domain.RecurringBill{
		UserID: userID, Name: "Rent", AmountPaisa: 500000, CategoryID: &catID, DayOfMonth: dayOfMonth, IsActive: true,
	})
	if err != nil {
		t.Fatalf("creating recurring bill: %v", err)
	}
	return b
}

func countBillTransactions(ctx context.Context, t *testing.T, pool *pgxpool.Pool, billID string) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, `select count(*) from transactions where recurring_bill_id = $1`, billID).Scan(&count); err != nil {
		t.Fatalf("counting bill transactions: %v", err)
	}
	return count
}

// TestRecurringBillRepo_GenerateDue_ConcurrentCallsProduceExactlyOneTransaction
// is the actual proof of race safety: 10 real goroutines call GenerateDue
// concurrently for the same bill. A sequential-calls test cannot demonstrate
// this — only genuine concurrent database access can.
func TestRecurringBillRepo_GenerateDue_ConcurrentCallsProduceExactlyOneTransaction(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	userID := testUser(ctx, t, pool)
	categoryID := testCategory(ctx, t, pool, userID, "Bills", "expense")
	bill := testBill(ctx, t, pool, userID, categoryID, 5)

	repo := NewRecurringBillRepo(pool)
	currentMonth := time.Date(2025, time.August, 1, 0, 0, 0, 0, time.UTC)
	const currentDay = 10 // past the bill's day-5 due date

	const n = 10
	var wg sync.WaitGroup
	results := make([][]domain.Transaction, n)
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = repo.GenerateDue(ctx, userID, currentMonth, currentDay)
		}(i)
	}
	wg.Wait()

	totalCreated := 0
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: GenerateDue() error = %v", i, err)
		}
		totalCreated += len(results[i])
	}
	if totalCreated != 1 {
		t.Errorf("total transactions created across %d concurrent calls = %d; want exactly 1", n, totalCreated)
	}
	if count := countBillTransactions(ctx, t, pool, bill.ID); count != 1 {
		t.Errorf("transactions in database for bill = %d; want exactly 1", count)
	}
}

// TestRecurringBillRepo_GenerateDue_RepeatedCallsAreNoOp verifies calling
// GenerateDue again for the same month (as GET /api/summary does on every
// request) creates nothing the second time.
func TestRecurringBillRepo_GenerateDue_RepeatedCallsAreNoOp(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	userID := testUser(ctx, t, pool)
	categoryID := testCategory(ctx, t, pool, userID, "Bills", "expense")
	bill := testBill(ctx, t, pool, userID, categoryID, 5)

	repo := NewRecurringBillRepo(pool)
	currentMonth := time.Date(2025, time.August, 1, 0, 0, 0, 0, time.UTC)

	first, err := repo.GenerateDue(ctx, userID, currentMonth, 10)
	if err != nil {
		t.Fatalf("first GenerateDue() error = %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first call created %d transactions; want 1", len(first))
	}

	second, err := repo.GenerateDue(ctx, userID, currentMonth, 10)
	if err != nil {
		t.Fatalf("second GenerateDue() error = %v", err)
	}
	if len(second) != 0 {
		t.Errorf("second call (repeated, same month) created %d transactions; want 0", len(second))
	}
	if count := countBillTransactions(ctx, t, pool, bill.ID); count != 1 {
		t.Errorf("transactions in database = %d; want exactly 1 after repeated calls", count)
	}
}

// TestRecurringBillRepo_GenerateDue_DoesNotRegenerateForEarlierMonth is the
// direct check for the WHERE clause using "< currentMonth", not "IS
// DISTINCT FROM": a bill already stamped for a future month must not
// regenerate (or move its stamp backwards) when called for an earlier one.
func TestRecurringBillRepo_GenerateDue_DoesNotRegenerateForEarlierMonth(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	userID := testUser(ctx, t, pool)
	categoryID := testCategory(ctx, t, pool, userID, "Bills", "expense")
	bill := testBill(ctx, t, pool, userID, categoryID, 5)

	repo := NewRecurringBillRepo(pool)
	september := time.Date(2025, time.September, 1, 0, 0, 0, 0, time.UTC)
	if _, err := repo.GenerateDue(ctx, userID, september, 10); err != nil {
		t.Fatalf("stamping September: %v", err)
	}

	august := time.Date(2025, time.August, 1, 0, 0, 0, 0, time.UTC)
	created, err := repo.GenerateDue(ctx, userID, august, 10)
	if err != nil {
		t.Fatalf("calling for earlier month: %v", err)
	}
	if len(created) != 0 {
		t.Errorf("calling GenerateDue for August after already stamped September created %d transactions; want 0", len(created))
	}

	got, err := repo.Get(ctx, userID, bill.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.LastGeneratedMonth == nil || !got.LastGeneratedMonth.Equal(september) {
		t.Errorf("LastGeneratedMonth = %v; want unchanged at September", got.LastGeneratedMonth)
	}
}

// TestRecurringBillRepo_GenerateDue_CatchesUpMissedMonths is fix 2's proof:
// a bill stamped three months back generates one transaction per missed
// month, not just the current one — the money was really owed.
func TestRecurringBillRepo_GenerateDue_CatchesUpMissedMonths(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	userID := testUser(ctx, t, pool)
	categoryID := testCategory(ctx, t, pool, userID, "Bills", "expense")
	bill := testBill(ctx, t, pool, userID, categoryID, 5)

	repo := NewRecurringBillRepo(pool)
	may := time.Date(2025, time.May, 1, 0, 0, 0, 0, time.UTC)
	// Prime last_generated_month directly rather than through a real
	// GenerateDue call, which would itself post a May transaction and
	// throw off the "3 missed months" count below.
	if _, err := pool.Exec(ctx, `update recurring_bills set last_generated_month = $1 where id = $2`, may, bill.ID); err != nil {
		t.Fatalf("priming last_generated_month: %v", err)
	}

	august := time.Date(2025, time.August, 1, 0, 0, 0, 0, time.UTC)
	created, err := repo.GenerateDue(ctx, userID, august, 10) // past day 5
	if err != nil {
		t.Fatalf("GenerateDue() error = %v", err)
	}
	if len(created) != 3 {
		t.Fatalf("created %d transactions catching up May->August; want 3 (June, July, August)", len(created))
	}
	wantMonths := []time.Month{time.June, time.July, time.August}
	for i, tx := range created {
		if tx.OccurredAt.Month() != wantMonths[i] || tx.OccurredAt.Day() != 5 {
			t.Errorf("created[%d].OccurredAt = %v; want day 5 of %s", i, tx.OccurredAt, wantMonths[i])
		}
	}
	if count := countBillTransactions(ctx, t, pool, bill.ID); count != 3 {
		t.Errorf("transactions in database = %d; want 3", count)
	}
}

func TestRecurringBillRepo_GenerateDue_ClampsLeapFebruary(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	userID := testUser(ctx, t, pool)
	categoryID := testCategory(ctx, t, pool, userID, "Bills", "expense")
	testBill(ctx, t, pool, userID, categoryID, 31)

	repo := NewRecurringBillRepo(pool)
	feb2024 := time.Date(2024, time.February, 1, 0, 0, 0, 0, time.UTC)
	created, err := repo.GenerateDue(ctx, userID, feb2024, 29)
	if err != nil {
		t.Fatalf("GenerateDue() error = %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("created %d transactions; want 1", len(created))
	}
	if created[0].OccurredAt.Month() != time.February || created[0].OccurredAt.Day() != 29 {
		t.Errorf("OccurredAt = %v; want Feb 29 2024", created[0].OccurredAt)
	}
}

func TestRecurringBillRepo_GenerateDue_ClampsNonLeapFebruary(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	userID := testUser(ctx, t, pool)
	categoryID := testCategory(ctx, t, pool, userID, "Bills", "expense")
	testBill(ctx, t, pool, userID, categoryID, 31)

	repo := NewRecurringBillRepo(pool)
	feb2023 := time.Date(2023, time.February, 1, 0, 0, 0, 0, time.UTC)
	created, err := repo.GenerateDue(ctx, userID, feb2023, 28)
	if err != nil {
		t.Fatalf("GenerateDue() error = %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("created %d transactions; want 1", len(created))
	}
	if created[0].OccurredAt.Month() != time.February || created[0].OccurredAt.Day() != 28 {
		t.Errorf("OccurredAt = %v; want Feb 28 2023", created[0].OccurredAt)
	}
}
