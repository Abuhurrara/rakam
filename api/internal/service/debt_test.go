package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type fakeDebtRepo struct {
	entries      map[string]domain.DebtEntry
	nextID       int
	transactions []domain.Transaction
}

func newFakeDebtRepo() *fakeDebtRepo {
	return &fakeDebtRepo{entries: make(map[string]domain.DebtEntry)}
}

func (f *fakeDebtRepo) ListByPerson(ctx context.Context, userID, personID string) ([]domain.DebtEntry, error) {
	var result []domain.DebtEntry
	for _, e := range f.entries {
		if e.UserID == userID && e.PersonID == personID {
			result = append(result, e)
		}
	}
	return result, nil
}

func (f *fakeDebtRepo) Get(ctx context.Context, userID, id string) (domain.DebtEntry, error) {
	e, ok := f.entries[id]
	if !ok || e.UserID != userID {
		return domain.DebtEntry{}, domain.ErrNotFound
	}
	return e, nil
}

func (f *fakeDebtRepo) Create(ctx context.Context, e domain.DebtEntry) (domain.DebtEntry, error) {
	f.nextID++
	e.ID = fmt.Sprintf("debt-%d", f.nextID)
	f.entries[e.ID] = e
	return e, nil
}

// settleUnsettled is the single choke point both Settle and
// SettleWithTransaction go through, mirroring postgres.DebtRepo's shared
// "update ... where settled_at is null" query — the already-settled check
// happens here, at call time, never from a value read earlier.
func (f *fakeDebtRepo) settleUnsettled(userID, id string) (domain.DebtEntry, error) {
	e, ok := f.entries[id]
	if !ok || e.UserID != userID {
		return domain.DebtEntry{}, domain.ErrNotFound
	}
	if e.SettledAt != nil {
		return domain.DebtEntry{}, domain.ErrAlreadySettled
	}
	now := time.Now()
	e.SettledAt = &now
	e.UpdatedAt = now
	f.entries[id] = e
	return e, nil
}

func (f *fakeDebtRepo) Settle(ctx context.Context, userID, id string) (domain.DebtEntry, error) {
	return f.settleUnsettled(userID, id)
}

func (f *fakeDebtRepo) SettleWithTransaction(ctx context.Context, userID, id string, categoryID *string) (domain.DebtEntry, domain.Transaction, error) {
	e, err := f.settleUnsettled(userID, id)
	if err != nil {
		return domain.DebtEntry{}, domain.Transaction{}, err
	}
	t := f.insertTransaction(userID, e, categoryID)
	return e, t, nil
}

func (f *fakeDebtRepo) SettleAll(ctx context.Context, userID, personID string, createTransaction bool, categoryID *string) ([]domain.DebtEntry, []domain.Transaction, error) {
	var settled []domain.DebtEntry
	var created []domain.Transaction
	for id, e := range f.entries {
		if e.UserID != userID || e.PersonID != personID || e.SettledAt != nil {
			continue
		}
		updated, err := f.settleUnsettled(userID, id)
		if err != nil {
			return nil, nil, err
		}
		settled = append(settled, updated)
		if createTransaction {
			created = append(created, f.insertTransaction(userID, updated, categoryID))
		}
	}
	return settled, created, nil
}

func (f *fakeDebtRepo) insertTransaction(userID string, e domain.DebtEntry, categoryID *string) domain.Transaction {
	description := e.Description
	t := domain.Transaction{
		ID:          fmt.Sprintf("tx-%d", len(f.transactions)+1),
		UserID:      userID,
		Kind:        domain.KindForDirection(e.Direction),
		AmountPaisa: e.AmountPaisa,
		CategoryID:  categoryID,
		Description: &description,
		OccurredAt:  time.Now(),
	}
	f.transactions = append(f.transactions, t)
	return t
}

func (f *fakeDebtRepo) Delete(ctx context.Context, userID, id string) error {
	e, ok := f.entries[id]
	if !ok || e.UserID != userID {
		return domain.ErrNotFound
	}
	delete(f.entries, id)
	return nil
}

func (f *fakeDebtRepo) HasAny(ctx context.Context, userID, personID string) (bool, error) {
	for _, e := range f.entries {
		if e.UserID == userID && e.PersonID == personID {
			return true, nil
		}
	}
	return false, nil
}

func setupDebtTest(t *testing.T) (*DebtService, *fakeDebtRepo, *fakePersonRepo, *fakeCategoryRepo, domain.Person) {
	t.Helper()
	debtRepo := newFakeDebtRepo()
	personRepo := newFakePersonRepo()
	catRepo := newFakeCategoryRepo()
	svc := NewDebtService(debtRepo, personRepo, catRepo)

	person, err := personRepo.Create(context.Background(), domain.Person{UserID: "user-1", Name: "Usman"})
	if err != nil {
		t.Fatalf("creating person: %v", err)
	}
	return svc, debtRepo, personRepo, catRepo, person
}

func TestDebtService_Create(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		build   func(personID string) domain.DebtEntry
		wantErr error
	}{
		{
			name: "valid they_owe entry",
			build: func(personID string) domain.DebtEntry {
				return domain.DebtEntry{UserID: "user-1", PersonID: personID, Direction: domain.DirectionTheyOwe, AmountPaisa: 5000, Description: "lunch", IncurredAt: now}
			},
		},
		{
			name: "invalid direction",
			build: func(personID string) domain.DebtEntry {
				return domain.DebtEntry{UserID: "user-1", PersonID: personID, Direction: "sideways", AmountPaisa: 5000, Description: "lunch", IncurredAt: now}
			},
			wantErr: domain.ErrInvalidDebtEntry,
		},
		{
			name: "zero amount",
			build: func(personID string) domain.DebtEntry {
				return domain.DebtEntry{UserID: "user-1", PersonID: personID, Direction: domain.DirectionIOwe, AmountPaisa: 0, Description: "lunch", IncurredAt: now}
			},
			wantErr: domain.ErrInvalidAmount,
		},
		{
			name: "empty description",
			build: func(personID string) domain.DebtEntry {
				return domain.DebtEntry{UserID: "user-1", PersonID: personID, Direction: domain.DirectionIOwe, AmountPaisa: 5000, Description: "  ", IncurredAt: now}
			},
			wantErr: domain.ErrInvalidDebtEntry,
		},
		{
			name: "zero incurred_at",
			build: func(personID string) domain.DebtEntry {
				return domain.DebtEntry{UserID: "user-1", PersonID: personID, Direction: domain.DirectionIOwe, AmountPaisa: 5000, Description: "lunch"}
			},
			wantErr: domain.ErrInvalidDebtEntry,
		},
		{
			name: "person belongs to another user",
			build: func(personID string) domain.DebtEntry {
				return domain.DebtEntry{UserID: "user-1", PersonID: "other-users-person", Direction: domain.DirectionIOwe, AmountPaisa: 5000, Description: "lunch", IncurredAt: now}
			},
			wantErr: domain.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _, _, person := setupDebtTest(t)
			_, err := svc.Create(context.Background(), tt.build(person.ID))
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

func TestDebtService_Settle_WithoutTransaction(t *testing.T) {
	svc, debtRepo, _, _, person := setupDebtTest(t)
	ctx := context.Background()

	entry, err := debtRepo.Create(ctx, domain.DebtEntry{UserID: "user-1", PersonID: person.ID, Direction: domain.DirectionTheyOwe, AmountPaisa: 5000, Description: "lunch", IncurredAt: time.Now()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	settled, tx, err := svc.Settle(ctx, "user-1", entry.ID, false, nil)
	if err != nil {
		t.Fatalf("Settle() unexpected error: %v", err)
	}
	if settled.SettledAt == nil {
		t.Fatal("Settle() did not stamp settled_at")
	}
	if tx != nil {
		t.Fatalf("Settle() with createTransaction=false returned a transaction: %+v", tx)
	}
	if len(debtRepo.transactions) != 0 {
		t.Fatalf("expected no transactions created, got %d", len(debtRepo.transactions))
	}
}

// TestDebtService_Settle_AlreadySettled_Rejected covers requirement #4:
// settling an already-settled entry must be rejected, not silently
// succeed a second time.
func TestDebtService_Settle_AlreadySettled_Rejected(t *testing.T) {
	svc, debtRepo, _, _, person := setupDebtTest(t)
	ctx := context.Background()

	entry, err := debtRepo.Create(ctx, domain.DebtEntry{UserID: "user-1", PersonID: person.ID, Direction: domain.DirectionTheyOwe, AmountPaisa: 5000, Description: "lunch", IncurredAt: time.Now()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, err := svc.Settle(ctx, "user-1", entry.ID, false, nil); err != nil {
		t.Fatalf("first Settle() unexpected error: %v", err)
	}

	if _, _, err := svc.Settle(ctx, "user-1", entry.ID, false, nil); !errors.Is(err, domain.ErrAlreadySettled) {
		t.Fatalf("second Settle() error = %v; want ErrAlreadySettled", err)
	}
}

// TestDebtService_Settle_WithTransaction_Twice_CreatesExactlyOne is the
// review-requested test for fix #2: the settled/already-settled decision
// must be made by the atomic update inside SettleWithTransaction, not by an
// earlier read. Calling Settle twice with createTransaction=true simulates
// two settle attempts on the same entry — only the first may create a
// transaction.
func TestDebtService_Settle_WithTransaction_Twice_CreatesExactlyOne(t *testing.T) {
	svc, debtRepo, _, _, person := setupDebtTest(t)
	ctx := context.Background()

	entry, err := debtRepo.Create(ctx, domain.DebtEntry{UserID: "user-1", PersonID: person.ID, Direction: domain.DirectionTheyOwe, AmountPaisa: 5000, Description: "lunch", IncurredAt: time.Now()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, tx1, err := svc.Settle(ctx, "user-1", entry.ID, true, nil)
	if err != nil {
		t.Fatalf("first Settle() unexpected error: %v", err)
	}
	if tx1 == nil {
		t.Fatal("first Settle() did not return a transaction")
	}

	_, tx2, err := svc.Settle(ctx, "user-1", entry.ID, true, nil)
	if !errors.Is(err, domain.ErrAlreadySettled) {
		t.Fatalf("second Settle() error = %v; want ErrAlreadySettled", err)
	}
	if tx2 != nil {
		t.Fatalf("second Settle() returned a transaction: %+v", tx2)
	}
	if len(debtRepo.transactions) != 1 {
		t.Fatalf("expected exactly 1 transaction created across both calls, got %d", len(debtRepo.transactions))
	}
}

func TestDebtService_Settle_WithTransaction_KindMapping(t *testing.T) {
	tests := []struct {
		name      string
		direction domain.DebtDirection
		wantKind  domain.Kind
	}{
		{name: "i_owe settles to an expense", direction: domain.DirectionIOwe, wantKind: domain.KindExpense},
		{name: "they_owe settles to income", direction: domain.DirectionTheyOwe, wantKind: domain.KindIncome},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, debtRepo, _, _, person := setupDebtTest(t)
			ctx := context.Background()

			entry, err := debtRepo.Create(ctx, domain.DebtEntry{UserID: "user-1", PersonID: person.ID, Direction: tt.direction, AmountPaisa: 7500, Description: "lunch", IncurredAt: time.Now()})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			_, tx, err := svc.Settle(ctx, "user-1", entry.ID, true, nil)
			if err != nil {
				t.Fatalf("Settle() unexpected error: %v", err)
			}
			if tx.Kind != tt.wantKind {
				t.Fatalf("created transaction Kind = %v; want %v", tx.Kind, tt.wantKind)
			}
			if tx.AmountPaisa != 7500 {
				t.Fatalf("created transaction AmountPaisa = %v; want 7500", tx.AmountPaisa)
			}
		})
	}
}

func TestDebtService_Settle_CategoryValidation(t *testing.T) {
	svc, debtRepo, _, catRepo, person := setupDebtTest(t)
	ctx := context.Background()

	expenseCat, err := catRepo.Create(ctx, domain.Category{UserID: "user-1", Name: "Food", Kind: domain.KindExpense})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	incomeCat, err := catRepo.Create(ctx, domain.Category{UserID: "user-1", Name: "Salary", Kind: domain.KindIncome})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	otherUsersCat, err := catRepo.Create(ctx, domain.Category{UserID: "user-2", Name: "Food", Kind: domain.KindExpense})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name       string
		categoryID string
		wantErr    error
	}{
		{name: "matching kind (i_owe settles to expense)", categoryID: expenseCat.ID},
		{name: "mismatched kind", categoryID: incomeCat.ID, wantErr: domain.ErrInvalidDebtEntry},
		{name: "another user's category", categoryID: otherUsersCat.ID, wantErr: domain.ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := debtRepo.Create(ctx, domain.DebtEntry{UserID: "user-1", PersonID: person.ID, Direction: domain.DirectionIOwe, AmountPaisa: 5000, Description: "lunch", IncurredAt: time.Now()})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			categoryID := tt.categoryID
			_, tx, err := svc.Settle(ctx, "user-1", entry.ID, true, &categoryID)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Settle() error = %v; want wrapping %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Settle() unexpected error: %v", err)
			}
			if tx.CategoryID == nil || *tx.CategoryID != tt.categoryID {
				t.Fatalf("created transaction CategoryID = %v; want %v", tx.CategoryID, tt.categoryID)
			}
		})
	}
}

// TestDebtService_SettleAll_MixedDirectionsPartialSettlement mirrors
// requirement #6 at the service layer for the settle behavior itself
// (net-balance correctness is verified against real SQL in
// postgres/person_test.go): a person with an unsettled they_owe, an
// unsettled i_owe, and one already-settled entry — settle-all must only
// touch the two unsettled ones.
func TestDebtService_SettleAll_MixedDirectionsPartialSettlement(t *testing.T) {
	svc, debtRepo, _, _, person := setupDebtTest(t)
	ctx := context.Background()

	theyOwe, err := debtRepo.Create(ctx, domain.DebtEntry{UserID: "user-1", PersonID: person.ID, Direction: domain.DirectionTheyOwe, AmountPaisa: 10000, Description: "a", IncurredAt: time.Now()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	iOwe, err := debtRepo.Create(ctx, domain.DebtEntry{UserID: "user-1", PersonID: person.ID, Direction: domain.DirectionIOwe, AmountPaisa: 3000, Description: "b", IncurredAt: time.Now()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	alreadySettled, err := debtRepo.Create(ctx, domain.DebtEntry{UserID: "user-1", PersonID: person.ID, Direction: domain.DirectionTheyOwe, AmountPaisa: 5000, Description: "c", IncurredAt: time.Now()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, err := svc.Settle(ctx, "user-1", alreadySettled.ID, false, nil); err != nil {
		t.Fatalf("pre-settling entry: %v", err)
	}
	preSettledAt := debtRepo.entries[alreadySettled.ID].SettledAt

	settled, created, err := svc.SettleAll(ctx, "user-1", person.ID, false, nil)
	if err != nil {
		t.Fatalf("SettleAll() unexpected error: %v", err)
	}
	if len(settled) != 2 {
		t.Fatalf("SettleAll() settled %d entries; want 2 (already-settled entry must not be re-touched)", len(settled))
	}
	if len(created) != 0 {
		t.Fatalf("SettleAll() with createTransaction=false created %d transactions; want 0", len(created))
	}
	if debtRepo.entries[theyOwe.ID].SettledAt == nil || debtRepo.entries[iOwe.ID].SettledAt == nil {
		t.Fatal("SettleAll() did not settle both previously-unsettled entries")
	}
	if debtRepo.entries[alreadySettled.ID].SettledAt != preSettledAt {
		t.Fatal("SettleAll() re-stamped an already-settled entry's settled_at")
	}
}

func TestDebtService_SettleAll_WithTransaction(t *testing.T) {
	svc, debtRepo, _, _, person := setupDebtTest(t)
	ctx := context.Background()

	if _, err := debtRepo.Create(ctx, domain.DebtEntry{UserID: "user-1", PersonID: person.ID, Direction: domain.DirectionTheyOwe, AmountPaisa: 10000, Description: "a", IncurredAt: time.Now()}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := debtRepo.Create(ctx, domain.DebtEntry{UserID: "user-1", PersonID: person.ID, Direction: domain.DirectionTheyOwe, AmountPaisa: 4000, Description: "b", IncurredAt: time.Now()}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	settled, created, err := svc.SettleAll(ctx, "user-1", person.ID, true, nil)
	if err != nil {
		t.Fatalf("SettleAll() unexpected error: %v", err)
	}
	if len(settled) != 2 || len(created) != 2 {
		t.Fatalf("SettleAll() settled=%d created=%d; want 2 and 2 (one transaction per settled entry)", len(settled), len(created))
	}
	for _, tx := range created {
		if tx.Kind != domain.KindIncome {
			t.Fatalf("created transaction Kind = %v; want income for they_owe", tx.Kind)
		}
	}
}

// TestDebtService_SettleAll_CategoryRejectedForMixedDirections covers fix
// #3 extended to the batch case: a single category can't be right for both
// an i_owe (settles to expense) and a they_owe (settles to income) entry.
func TestDebtService_SettleAll_CategoryRejectedForMixedDirections(t *testing.T) {
	svc, debtRepo, _, catRepo, person := setupDebtTest(t)
	ctx := context.Background()

	if _, err := debtRepo.Create(ctx, domain.DebtEntry{UserID: "user-1", PersonID: person.ID, Direction: domain.DirectionTheyOwe, AmountPaisa: 10000, Description: "a", IncurredAt: time.Now()}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := debtRepo.Create(ctx, domain.DebtEntry{UserID: "user-1", PersonID: person.ID, Direction: domain.DirectionIOwe, AmountPaisa: 3000, Description: "b", IncurredAt: time.Now()}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expenseCat, err := catRepo.Create(ctx, domain.Category{UserID: "user-1", Name: "Food", Kind: domain.KindExpense})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, _, err = svc.SettleAll(ctx, "user-1", person.ID, true, &expenseCat.ID)
	if !errors.Is(err, domain.ErrInvalidDebtEntry) {
		t.Fatalf("SettleAll() error = %v; want ErrInvalidDebtEntry", err)
	}
	if debtRepo.countUnsettled(t, "user-1", person.ID) != 2 {
		t.Fatal("SettleAll() mutated entries despite the category rejection")
	}
}

// countUnsettled is a test-only helper counting unsettled
// entries, used to confirm a rejected SettleAll call touched nothing.
func (f *fakeDebtRepo) countUnsettled(t *testing.T, userID, personID string) int {
	t.Helper()
	count := 0
	for _, e := range f.entries {
		if e.UserID == userID && e.PersonID == personID && e.SettledAt == nil {
			count++
		}
	}
	return count
}

// TestDebtService_SettleAll_ChecksPersonFirst covers fix #4: SettleAll must
// confirm the person exists (and belongs to the user) before touching any
// rows, not after.
func TestDebtService_SettleAll_ChecksPersonFirst(t *testing.T) {
	svc, debtRepo, _, _, person := setupDebtTest(t)
	ctx := context.Background()

	if _, err := debtRepo.Create(ctx, domain.DebtEntry{UserID: "user-1", PersonID: person.ID, Direction: domain.DirectionTheyOwe, AmountPaisa: 10000, Description: "a", IncurredAt: time.Now()}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wrong user for this person: must fail before touching any entries.
	if _, _, err := svc.SettleAll(ctx, "user-2", person.ID, false, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("SettleAll() by wrong user error = %v; want ErrNotFound", err)
	}
	if debtRepo.countUnsettled(t, "user-1", person.ID) != 1 {
		t.Fatal("SettleAll() mutated entries despite failing the person check")
	}
}
