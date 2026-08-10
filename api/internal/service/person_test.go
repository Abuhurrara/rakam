package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type fakePersonRepo struct {
	people   map[string]domain.Person
	balances map[string]domain.Money
	nextID   int
}

func newFakePersonRepo() *fakePersonRepo {
	return &fakePersonRepo{people: make(map[string]domain.Person), balances: make(map[string]domain.Money)}
}

func (f *fakePersonRepo) List(ctx context.Context, userID string) ([]domain.PersonBalance, error) {
	var result []domain.PersonBalance
	for _, p := range f.people {
		if p.UserID != userID {
			continue
		}
		result = append(result, domain.PersonBalance{Person: p, BalancePaisa: f.balances[p.ID]})
	}
	return result, nil
}

func (f *fakePersonRepo) Get(ctx context.Context, userID, id string) (domain.Person, error) {
	p, ok := f.people[id]
	if !ok || p.UserID != userID {
		return domain.Person{}, domain.ErrNotFound
	}
	return p, nil
}

func (f *fakePersonRepo) Create(ctx context.Context, p domain.Person) (domain.Person, error) {
	f.nextID++
	p.ID = fmt.Sprintf("person-%d", f.nextID)
	f.people[p.ID] = p
	return p, nil
}

func (f *fakePersonRepo) Delete(ctx context.Context, userID, id string) error {
	p, ok := f.people[id]
	if !ok || p.UserID != userID {
		return domain.ErrNotFound
	}
	delete(f.people, id)
	return nil
}

func TestPersonService_Create(t *testing.T) {
	tests := []struct {
		name    string
		input   domain.Person
		wantErr error
	}{
		{name: "valid name", input: domain.Person{UserID: "user-1", Name: "Usman"}},
		{name: "empty name", input: domain.Person{UserID: "user-1", Name: "   "}, wantErr: domain.ErrInvalidPerson},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakePersonRepo()
			svc := NewPersonService(repo, newFakeDebtRepo())

			_, err := svc.Create(context.Background(), tt.input)
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

func TestPersonService_List_ScopesToUser(t *testing.T) {
	repo := newFakePersonRepo()
	svc := NewPersonService(repo, newFakeDebtRepo())
	ctx := context.Background()

	if _, err := svc.Create(ctx, domain.Person{UserID: "user-1", Name: "Usman"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := svc.Create(ctx, domain.Person{UserID: "user-2", Name: "Moiz"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := svc.List(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Person.Name != "Usman" {
		t.Fatalf("got %+v, want only user-1's Usman", got)
	}
}

// TestPersonService_Delete_BlocksUnsettled covers requirement #5: a person
// with a remaining unsettled debt entry must not be deletable, since that
// would make money owed unrecoverable to view.
func TestPersonService_Delete_BlocksUnsettled(t *testing.T) {
	personRepo := newFakePersonRepo()
	debtRepo := newFakeDebtRepo()
	svc := NewPersonService(personRepo, debtRepo)
	ctx := context.Background()

	person, err := svc.Create(ctx, domain.Person{UserID: "user-1", Name: "Usman"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := debtRepo.Create(ctx, domain.DebtEntry{
		UserID: "user-1", PersonID: person.ID, Direction: domain.DirectionTheyOwe,
		AmountPaisa: 10000, Description: "lunch", IncurredAt: time.Now(),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := svc.Delete(ctx, "user-1", person.ID); !errors.Is(err, domain.ErrPersonHasDebtEntries) {
		t.Fatalf("Delete() error = %v; want ErrPersonHasDebtEntries", err)
	}
	if _, ok := personRepo.people[person.ID]; !ok {
		t.Fatal("person was deleted despite having an unsettled entry")
	}
}

// TestPersonService_Delete_BlocksSettled covers the bug found by end-to-end
// testing: debt_entries.person_id has no ON DELETE cascade, so a person
// with only settled history still can't be deleted without either losing
// that history or hitting a raw foreign-key violation. The guard must
// block on any entry, not just unsettled ones.
func TestPersonService_Delete_BlocksSettled(t *testing.T) {
	personRepo := newFakePersonRepo()
	debtRepo := newFakeDebtRepo()
	svc := NewPersonService(personRepo, debtRepo)
	ctx := context.Background()

	person, err := svc.Create(ctx, domain.Person{UserID: "user-1", Name: "Usman"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entry, err := debtRepo.Create(ctx, domain.DebtEntry{
		UserID: "user-1", PersonID: person.ID, Direction: domain.DirectionTheyOwe,
		AmountPaisa: 10000, Description: "lunch", IncurredAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := debtRepo.Settle(ctx, "user-1", entry.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := svc.Delete(ctx, "user-1", person.ID); !errors.Is(err, domain.ErrPersonHasDebtEntries) {
		t.Fatalf("Delete() error = %v; want ErrPersonHasDebtEntries", err)
	}
	if _, ok := personRepo.people[person.ID]; !ok {
		t.Fatal("person was deleted despite having settled debt history")
	}
}

func TestPersonService_Delete_AllowsWhenNoEntries(t *testing.T) {
	personRepo := newFakePersonRepo()
	debtRepo := newFakeDebtRepo()
	svc := NewPersonService(personRepo, debtRepo)
	ctx := context.Background()

	person, err := svc.Create(ctx, domain.Person{UserID: "user-1", Name: "Usman"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := svc.Delete(ctx, "user-1", person.ID); err != nil {
		t.Fatalf("Delete() unexpected error: %v", err)
	}
	if _, ok := personRepo.people[person.ID]; ok {
		t.Fatal("person still present after Delete()")
	}
}
