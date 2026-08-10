package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/port"
)

type DebtService struct {
	debtRepo   port.DebtRepo
	personRepo port.PersonRepo
	catRepo    port.CategoryRepo
}

func NewDebtService(debtRepo port.DebtRepo, personRepo port.PersonRepo, catRepo port.CategoryRepo) *DebtService {
	return &DebtService{debtRepo: debtRepo, personRepo: personRepo, catRepo: catRepo}
}

func (s *DebtService) ListForPerson(ctx context.Context, userID, personID string) ([]domain.DebtEntry, error) {
	if err := s.checkPerson(ctx, userID, personID); err != nil {
		return nil, err
	}
	entries, err := s.debtRepo.ListByPerson(ctx, userID, personID)
	if err != nil {
		return nil, fmt.Errorf("listing debt entries: %w", err)
	}
	return entries, nil
}

func (s *DebtService) Create(ctx context.Context, e domain.DebtEntry) (domain.DebtEntry, error) {
	if err := validateDebtEntry(e); err != nil {
		return domain.DebtEntry{}, err
	}
	if err := s.checkPerson(ctx, e.UserID, e.PersonID); err != nil {
		return domain.DebtEntry{}, err
	}
	created, err := s.debtRepo.Create(ctx, e)
	if err != nil {
		return domain.DebtEntry{}, fmt.Errorf("creating debt entry: %w", err)
	}
	return created, nil
}

// Settle stamps settled_at on the entry. When createTransaction is true, it
// also records the money moving — atomically, in one DebtRepo call backed
// by a single pgx transaction, because a debt marked settled with no
// matching transaction (or vice versa) is a correctness bug, not a
// cosmetic one. Settling an already-settled entry is rejected, not a
// no-op: a caller that double-taps or retries needs to know nothing
// happened, not silently believe it succeeded.
func (s *DebtService) Settle(ctx context.Context, userID, id string, createTransaction bool, categoryID *string) (domain.DebtEntry, *domain.Transaction, error) {
	if !createTransaction {
		e, err := s.debtRepo.Settle(ctx, userID, id)
		if err != nil {
			return domain.DebtEntry{}, nil, fmt.Errorf("settling debt entry: %w", err)
		}
		return e, nil, nil
	}

	if categoryID != nil {
		// Direction is immutable once a debt entry is created — there's no
		// update endpoint for it — so reading it here to validate the
		// category doesn't reintroduce a settled_at race. The actual
		// settle/already-settled decision is made entirely inside
		// SettleWithTransaction's transaction, not from this read.
		entry, err := s.debtRepo.Get(ctx, userID, id)
		if err != nil {
			return domain.DebtEntry{}, nil, fmt.Errorf("looking up debt entry: %w", err)
		}
		if err := s.checkCategoryKind(ctx, userID, *categoryID, domain.KindForDirection(entry.Direction)); err != nil {
			return domain.DebtEntry{}, nil, err
		}
	}

	settled, created, err := s.debtRepo.SettleWithTransaction(ctx, userID, id, categoryID)
	if err != nil {
		return domain.DebtEntry{}, nil, fmt.Errorf("settling debt entry: %w", err)
	}
	return settled, &created, nil
}

// SettleAll settles every unsettled entry for personID in one database
// transaction — a half-settled batch would be worse than not settling at
// all. Already-settled entries are simply excluded by the update's where
// clause, which is what makes calling settle-all twice a safe no-op rather
// than an error. createTransaction/categoryID behave exactly as they do for
// a single Settle, applied to every entry the batch touches — settling one
// entry and settling several must not produce different kinds of records
// for the same intent.
func (s *DebtService) SettleAll(ctx context.Context, userID, personID string, createTransaction bool, categoryID *string) ([]domain.DebtEntry, []domain.Transaction, error) {
	if err := s.checkPerson(ctx, userID, personID); err != nil {
		return nil, nil, err
	}

	if createTransaction && categoryID != nil {
		if err := s.checkCategoryKindForAllUnsettled(ctx, userID, personID, *categoryID); err != nil {
			return nil, nil, err
		}
	}

	settled, created, err := s.debtRepo.SettleAll(ctx, userID, personID, createTransaction, categoryID)
	if err != nil {
		return nil, nil, fmt.Errorf("settling all debt entries: %w", err)
	}
	return settled, created, nil
}

func (s *DebtService) Delete(ctx context.Context, userID, id string) error {
	if err := s.debtRepo.Delete(ctx, userID, id); err != nil {
		return fmt.Errorf("deleting debt entry: %w", err)
	}
	return nil
}

// checkPerson validates that personID exists and belongs to userID — a
// person that doesn't exist and one owned by someone else look identical
// here, both ErrNotFound. Same rationale as TransactionService.checkCategory.
func (s *DebtService) checkPerson(ctx context.Context, userID, personID string) error {
	if _, err := s.personRepo.Get(ctx, userID, personID); err != nil {
		return fmt.Errorf("looking up person: %w", err)
	}
	return nil
}

func (s *DebtService) checkCategoryKind(ctx context.Context, userID, categoryID string, wantKind domain.Kind) error {
	cat, err := s.catRepo.Get(ctx, userID, categoryID)
	if err != nil {
		return fmt.Errorf("looking up category: %w", err)
	}
	if cat.Kind != wantKind {
		return fmt.Errorf("%w: category kind does not match settlement direction", domain.ErrInvalidDebtEntry)
	}
	return nil
}

// checkCategoryKindForAllUnsettled rejects the whole settle-all call up
// front if the given category's kind wouldn't match every entry it's about
// to settle. A person can have both i_owe and they_owe entries, which
// settle to different transaction kinds, and a single category can't be
// right for both — failing loud here beats silently leaving some created
// transactions uncategorised.
func (s *DebtService) checkCategoryKindForAllUnsettled(ctx context.Context, userID, personID, categoryID string) error {
	entries, err := s.debtRepo.ListByPerson(ctx, userID, personID)
	if err != nil {
		return fmt.Errorf("listing debt entries: %w", err)
	}
	cat, err := s.catRepo.Get(ctx, userID, categoryID)
	if err != nil {
		return fmt.Errorf("looking up category: %w", err)
	}
	for _, e := range entries {
		if e.SettledAt != nil {
			continue
		}
		if domain.KindForDirection(e.Direction) != cat.Kind {
			return fmt.Errorf("%w: category kind does not match every unsettled entry's direction", domain.ErrInvalidDebtEntry)
		}
	}
	return nil
}

func validateDebtEntry(e domain.DebtEntry) error {
	if e.Direction != domain.DirectionIOwe && e.Direction != domain.DirectionTheyOwe {
		return fmt.Errorf("%w: direction must be i_owe or they_owe", domain.ErrInvalidDebtEntry)
	}
	if e.AmountPaisa <= 0 {
		return fmt.Errorf("%w: amount must be greater than zero", domain.ErrInvalidAmount)
	}
	if strings.TrimSpace(e.Description) == "" {
		return fmt.Errorf("%w: description is required", domain.ErrInvalidDebtEntry)
	}
	if e.IncurredAt.IsZero() {
		return fmt.Errorf("%w: incurred_at is required", domain.ErrInvalidDebtEntry)
	}
	return nil
}
