package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/port"
)

type BudgetService struct {
	budgetRepo port.BudgetRepo
	catRepo    port.CategoryRepo
	loc        *time.Location
}

func NewBudgetService(budgetRepo port.BudgetRepo, catRepo port.CategoryRepo, loc *time.Location) *BudgetService {
	return &BudgetService{budgetRepo: budgetRepo, catRepo: catRepo, loc: loc}
}

func (s *BudgetService) ListForMonth(ctx context.Context, userID, monthStr string) ([]domain.BudgetWithSpent, error) {
	month, _, err := karachiMonthRange(monthStr, s.loc)
	if err != nil {
		return nil, err
	}
	rows, err := s.budgetRepo.ListWithSpent(ctx, userID, month)
	if err != nil {
		return nil, fmt.Errorf("listing budgets: %w", err)
	}
	return rows, nil
}

// Upsert validates the limit and the category before writing, mirroring
// TransactionService.checkCategory: a category that doesn't exist and one
// owned by someone else both come back as ErrNotFound, and a budget can
// only point at an expense category since spent is always summed from
// expense transactions. monthStr is parsed and Karachi-anchored here, the
// same as ListForMonth, since s.loc is only available at this layer.
func (s *BudgetService) Upsert(ctx context.Context, userID, categoryID, monthStr string, limitPaisa domain.Money) (domain.Budget, error) {
	if limitPaisa <= 0 {
		return domain.Budget{}, fmt.Errorf("%w: limit must be greater than zero", domain.ErrInvalidAmount)
	}
	month, _, err := karachiMonthRange(monthStr, s.loc)
	if err != nil {
		return domain.Budget{}, err
	}
	cat, err := s.catRepo.Get(ctx, userID, categoryID)
	if err != nil {
		return domain.Budget{}, fmt.Errorf("looking up category: %w", err)
	}
	if cat.Kind != domain.KindExpense {
		return domain.Budget{}, fmt.Errorf("%w: budget category must be an expense category", domain.ErrInvalidBudget)
	}

	upserted, err := s.budgetRepo.Upsert(ctx, domain.Budget{UserID: userID, CategoryID: categoryID, Month: month, LimitPaisa: limitPaisa})
	if err != nil {
		return domain.Budget{}, fmt.Errorf("upserting budget: %w", err)
	}
	return upserted, nil
}

func (s *BudgetService) Delete(ctx context.Context, userID, id string) error {
	if err := s.budgetRepo.Delete(ctx, userID, id); err != nil {
		return fmt.Errorf("deleting budget: %w", err)
	}
	return nil
}
