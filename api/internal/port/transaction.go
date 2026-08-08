package port

import (
	"context"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

// TransactionFilter narrows TransactionRepo.List. From/To bound occurred_at
// as a half-open interval [From, To); either or both may be nil.
type TransactionFilter struct {
	From       *time.Time
	To         *time.Time
	CategoryID *string
	Query      *string
	Limit      int
	Offset     int
}

type TransactionRepo interface {
	// List returns the page of transactions matching filter along with the
	// total count of matching rows across all pages.
	List(ctx context.Context, userID string, filter TransactionFilter) ([]domain.Transaction, int, error)
	Get(ctx context.Context, userID, id string) (domain.Transaction, error)
	Create(ctx context.Context, t domain.Transaction) (domain.Transaction, error)
	Update(ctx context.Context, t domain.Transaction) (domain.Transaction, error)
	Delete(ctx context.Context, userID, id string) error
}