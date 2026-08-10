package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

// TestPersonRepo_List_BalanceAggregation is the one test that exercises the
// real coalesce(sum(...)) SQL in PersonRepo.List end to end — the service
// layer's fake repos don't reimplement that formula, so this is the only
// place it's actually verified. Skips unless TEST_DATABASE_URL is set, per
// SPEC.md's repository-test convention.
func TestPersonRepo_List_BalanceAggregation(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting to test database: %v", err)
	}
	defer pool.Close()

	email := fmt.Sprintf("person-repo-test-%d@example.com", time.Now().UnixNano())
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

	personRepo := NewPersonRepo(pool)
	debtRepo := NewDebtRepo(pool)

	// Requirement #2: a person with zero debt entries must get balance 0,
	// not NULL.
	zeroPerson, err := personRepo.Create(ctx, domain.Person{UserID: userID, Name: "No Entries"})
	if err != nil {
		t.Fatalf("creating person: %v", err)
	}

	// Requirement #6: mixed directions with partial settlement.
	mixedPerson, err := personRepo.Create(ctx, domain.Person{UserID: userID, Name: "Mixed"})
	if err != nil {
		t.Fatalf("creating person: %v", err)
	}

	now := time.Now()
	if _, err := debtRepo.Create(ctx, domain.DebtEntry{UserID: userID, PersonID: mixedPerson.ID, Direction: domain.DirectionTheyOwe, AmountPaisa: 10000, Description: "unsettled they_owe", IncurredAt: now}); err != nil {
		t.Fatalf("creating debt entry: %v", err)
	}
	if _, err := debtRepo.Create(ctx, domain.DebtEntry{UserID: userID, PersonID: mixedPerson.ID, Direction: domain.DirectionIOwe, AmountPaisa: 3000, Description: "unsettled i_owe", IncurredAt: now}); err != nil {
		t.Fatalf("creating debt entry: %v", err)
	}
	settledEntry, err := debtRepo.Create(ctx, domain.DebtEntry{UserID: userID, PersonID: mixedPerson.ID, Direction: domain.DirectionTheyOwe, AmountPaisa: 5000, Description: "settled they_owe, must be excluded", IncurredAt: now})
	if err != nil {
		t.Fatalf("creating debt entry: %v", err)
	}
	if _, err := debtRepo.Settle(ctx, userID, settledEntry.ID); err != nil {
		t.Fatalf("settling debt entry: %v", err)
	}

	people, err := personRepo.List(ctx, userID)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	balances := make(map[string]domain.Money)
	for _, p := range people {
		balances[p.Person.ID] = p.BalancePaisa
	}

	if got := balances[zeroPerson.ID]; got != 0 {
		t.Errorf("zero-entry person balance = %d; want 0", got)
	}
	if got, want := balances[mixedPerson.ID], domain.Money(7000); got != want {
		t.Errorf("mixed person balance = %d; want %d (10000 they_owe - 3000 i_owe, settled 5000 excluded)", got, want)
	}
}
