package port

import (
	"context"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type BudgetRepo interface {
	// ListWithSpent returns every non-archived expense category for userID,
	// each paired with its budget for month (nil if unset) and spent_paisa
	// computed from expense transactions in [month, month+1mo).
	ListWithSpent(ctx context.Context, userID string, month time.Time) ([]domain.BudgetWithSpent, error)
	Upsert(ctx context.Context, b domain.Budget) (domain.Budget, error)
	Delete(ctx context.Context, userID, id string) error
}
