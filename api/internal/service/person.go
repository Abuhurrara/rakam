package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/port"
)

type PersonService struct {
	personRepo port.PersonRepo
	debtRepo   port.DebtRepo
}

func NewPersonService(personRepo port.PersonRepo, debtRepo port.DebtRepo) *PersonService {
	return &PersonService{personRepo: personRepo, debtRepo: debtRepo}
}

func (s *PersonService) List(ctx context.Context, userID string) ([]domain.PersonBalance, error) {
	people, err := s.personRepo.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing people: %w", err)
	}
	return people, nil
}

func (s *PersonService) Create(ctx context.Context, p domain.Person) (domain.Person, error) {
	if strings.TrimSpace(p.Name) == "" {
		return domain.Person{}, fmt.Errorf("%w: name is required", domain.ErrInvalidPerson)
	}
	created, err := s.personRepo.Create(ctx, p)
	if err != nil {
		return domain.Person{}, fmt.Errorf("creating person: %w", err)
	}
	return created, nil
}

// Delete blocks removing a person who has any debt entry at all, settled
// or not. debt_entries.person_id has no ON DELETE cascade, and settled
// entries must stay viewable (SPEC.md) — so a person can only be deleted
// once they have no debt history to lose.
func (s *PersonService) Delete(ctx context.Context, userID, id string) error {
	has, err := s.debtRepo.HasAny(ctx, userID, id)
	if err != nil {
		return fmt.Errorf("checking debt entries: %w", err)
	}
	if has {
		return fmt.Errorf("%w", domain.ErrPersonHasDebtEntries)
	}
	if err := s.personRepo.Delete(ctx, userID, id); err != nil {
		return fmt.Errorf("deleting person: %w", err)
	}
	return nil
}
