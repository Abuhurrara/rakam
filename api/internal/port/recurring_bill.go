package port

import (
	"context"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type RecurringBillRepo interface {
	List(ctx context.Context, userID string) ([]domain.RecurringBill, error)
	Get(ctx context.Context, userID, id string) (domain.RecurringBill, error)
	Create(ctx context.Context, b domain.RecurringBill) (domain.RecurringBill, error)
	Update(ctx context.Context, b domain.RecurringBill) (domain.RecurringBill, error)
	Delete(ctx context.Context, userID, id string) error

	// GenerateDue atomically claims every active bill not yet caught up to
	// currentMonth and creates one transaction per month it missed, from
	// just after its last_generated_month through either currentMonth (if
	// currentDay has reached its clamped day_of_month) or currentMonth
	// minus one (otherwise, leaving the current month for a later call).
	// currentMonth and currentDay must already be Karachi-resolved by the
	// caller — this method does no timezone math itself.
	GenerateDue(ctx context.Context, userID string, currentMonth time.Time, currentDay int) ([]domain.Transaction, error)
}
