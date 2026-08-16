package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/port"
)

const (
	defaultTransactionLimit = 50
	maxTransactionLimit     = 200
)

type TransactionService struct {
	txRepo  port.TransactionRepo
	catRepo port.CategoryRepo
	loc     *time.Location
}

func NewTransactionService(txRepo port.TransactionRepo, catRepo port.CategoryRepo, loc *time.Location) *TransactionService {
	return &TransactionService{txRepo: txRepo, catRepo: catRepo, loc: loc}
}

// ListParams is the caller-supplied (unvalidated, unclamped) view of a
// transaction list request. MonthStr, when set, is "2006-01".
type ListParams struct {
	MonthStr   *string
	CategoryID *string
	Query      *string
	Limit      int
	Offset     int
}

// ListResult carries the page of transactions plus the pagination values
// actually applied (after clamping) — the caller needs those to render
// e.g. "showing 1-50 of 214", not just the raw request values.
type ListResult struct {
	Transactions []domain.Transaction
	Total        int
	Limit        int
	Offset       int
}

func (s *TransactionService) List(ctx context.Context, userID string, p ListParams) (ListResult, error) {
	filter := port.TransactionFilter{
		CategoryID: p.CategoryID,
		Query:      p.Query,
		Limit:      clampLimit(p.Limit),
		Offset:     clampOffset(p.Offset),
	}

	if p.MonthStr != nil {
		first, next, err := karachiMonthRange(*p.MonthStr, s.loc)
		if err != nil {
			return ListResult{}, err
		}
		filter.From = &first
		filter.To = &next
	}

	transactions, total, err := s.txRepo.List(ctx, userID, filter)
	if err != nil {
		return ListResult{}, fmt.Errorf("listing transactions: %w", err)
	}
	return ListResult{Transactions: transactions, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return defaultTransactionLimit
	}
	if limit > maxTransactionLimit {
		return maxTransactionLimit
	}
	return limit
}

func clampOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func (s *TransactionService) Create(ctx context.Context, t domain.Transaction) (domain.Transaction, error) {
	if err := validateTransaction(t); err != nil {
		return domain.Transaction{}, err
	}
	if err := s.checkCategory(ctx, t); err != nil {
		return domain.Transaction{}, err
	}

	created, err := s.txRepo.Create(ctx, t)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("creating transaction: %w", err)
	}
	return created, nil
}

func (s *TransactionService) Update(ctx context.Context, t domain.Transaction) (domain.Transaction, error) {
	if err := validateTransaction(t); err != nil {
		return domain.Transaction{}, err
	}
	if err := s.checkCategory(ctx, t); err != nil {
		return domain.Transaction{}, err
	}

	updated, err := s.txRepo.Update(ctx, t)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("updating transaction: %w", err)
	}
	return updated, nil
}

func (s *TransactionService) Delete(ctx context.Context, userID, id string) error {
	if err := s.txRepo.Delete(ctx, userID, id); err != nil {
		return fmt.Errorf("deleting transaction: %w", err)
	}
	return nil
}

func validateTransaction(t domain.Transaction) error {
	if t.Kind != domain.KindExpense && t.Kind != domain.KindIncome {
		return fmt.Errorf("%w: kind must be expense or income", domain.ErrInvalidTransaction)
	}
	if t.AmountPaisa <= 0 {
		return fmt.Errorf("%w: amount must be greater than zero", domain.ErrInvalidAmount)
	}
	if t.OccurredAt.IsZero() {
		return fmt.Errorf("%w: occurred_at is required", domain.ErrInvalidTransaction)
	}
	if t.OccurredAt.After(time.Now().Add(24 * time.Hour)) {
		return fmt.Errorf("%w: occurred_at cannot be more than a day in the future", domain.ErrInvalidTransaction)
	}
	return nil
}

// checkCategory validates that a set category_id belongs to the same user
// (a category that doesn't exist and one owned by someone else look
// identical here, both ErrNotFound) and that its kind matches the
// transaction's — an expense can't point at an income category.
func (s *TransactionService) checkCategory(ctx context.Context, t domain.Transaction) error {
	if t.CategoryID == nil {
		return nil
	}
	cat, err := s.catRepo.Get(ctx, t.UserID, *t.CategoryID)
	if err != nil {
		return fmt.Errorf("looking up category: %w", err)
	}
	if cat.Kind != t.Kind {
		return fmt.Errorf("%w: category kind does not match transaction kind", domain.ErrInvalidTransaction)
	}
	return nil
}