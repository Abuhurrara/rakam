package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/port"
)

type RecurringBillService struct {
	billRepo port.RecurringBillRepo
	catRepo  port.CategoryRepo
	loc      *time.Location
}

func NewRecurringBillService(billRepo port.RecurringBillRepo, catRepo port.CategoryRepo, loc *time.Location) *RecurringBillService {
	return &RecurringBillService{billRepo: billRepo, catRepo: catRepo, loc: loc}
}

func (s *RecurringBillService) List(ctx context.Context, userID string) ([]domain.RecurringBill, error) {
	bills, err := s.billRepo.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing recurring bills: %w", err)
	}
	return bills, nil
}

func (s *RecurringBillService) Get(ctx context.Context, userID, id string) (domain.RecurringBill, error) {
	b, err := s.billRepo.Get(ctx, userID, id)
	if err != nil {
		return domain.RecurringBill{}, fmt.Errorf("getting recurring bill: %w", err)
	}
	return b, nil
}

func (s *RecurringBillService) Create(ctx context.Context, b domain.RecurringBill) (domain.RecurringBill, error) {
	if err := s.validate(ctx, b); err != nil {
		return domain.RecurringBill{}, err
	}
	created, err := s.billRepo.Create(ctx, b)
	if err != nil {
		return domain.RecurringBill{}, fmt.Errorf("creating recurring bill: %w", err)
	}
	return created, nil
}

func (s *RecurringBillService) Update(ctx context.Context, b domain.RecurringBill) (domain.RecurringBill, error) {
	if err := s.validate(ctx, b); err != nil {
		return domain.RecurringBill{}, err
	}
	updated, err := s.billRepo.Update(ctx, b)
	if err != nil {
		return domain.RecurringBill{}, fmt.Errorf("updating recurring bill: %w", err)
	}
	return updated, nil
}

func (s *RecurringBillService) Delete(ctx context.Context, userID, id string) error {
	if err := s.billRepo.Delete(ctx, userID, id); err != nil {
		return fmt.Errorf("deleting recurring bill: %w", err)
	}
	return nil
}

// GenerateDue derives "today" and "the current month" from s.loc — never
// UTC, never server-local time — exactly once, here, and passes them down
// as already-resolved values. Everything below this point (the repo, the
// SQL) works only with those values; no timezone decision is made anywhere
// else in the generation path.
func (s *RecurringBillService) GenerateDue(ctx context.Context, userID string) ([]domain.Transaction, error) {
	now := time.Now().In(s.loc)
	currentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.loc)
	created, err := s.billRepo.GenerateDue(ctx, userID, currentMonth, now.Day())
	if err != nil {
		return nil, fmt.Errorf("generating due recurring bills: %w", err)
	}
	return created, nil
}

func (s *RecurringBillService) validate(ctx context.Context, b domain.RecurringBill) error {
	if b.AmountPaisa <= 0 {
		return fmt.Errorf("%w: amount must be greater than zero", domain.ErrInvalidAmount)
	}
	if b.DayOfMonth < 1 || b.DayOfMonth > 31 {
		return fmt.Errorf("%w: day_of_month must be between 1 and 31", domain.ErrInvalidRecurringBill)
	}
	if b.Name == "" {
		return fmt.Errorf("%w: name is required", domain.ErrInvalidRecurringBill)
	}
	if b.CategoryID == nil {
		return nil
	}
	cat, err := s.catRepo.Get(ctx, b.UserID, *b.CategoryID)
	if err != nil {
		return fmt.Errorf("looking up category: %w", err)
	}
	if cat.Kind != domain.KindExpense {
		return fmt.Errorf("%w: bill category must be an expense category", domain.ErrInvalidRecurringBill)
	}
	return nil
}
