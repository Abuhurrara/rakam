package port

import (
	"context"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type DebtRepo interface {
	ListByPerson(ctx context.Context, userID, personID string) ([]domain.DebtEntry, error)
	Get(ctx context.Context, userID, id string) (domain.DebtEntry, error)
	Create(ctx context.Context, e domain.DebtEntry) (domain.DebtEntry, error)

	// Settle stamps settled_at without creating a transaction. It fails
	// with domain.ErrAlreadySettled, not a silent no-op, if the entry is
	// already settled.
	Settle(ctx context.Context, userID, id string) (domain.DebtEntry, error)

	// SettleWithTransaction settles the entry and inserts a transaction
	// record for it inside a single database transaction — both writes
	// succeed or neither does. categoryID is optional. The settled/
	// already-settled decision is made entirely inside that transaction,
	// from the row the settling update itself returns.
	SettleWithTransaction(ctx context.Context, userID, id string, categoryID *string) (domain.DebtEntry, domain.Transaction, error)

	// SettleAll settles every currently-unsettled entry for personID in one
	// database transaction. When createTransaction is true, it inserts one
	// transaction per settled entry inside that same transaction.
	SettleAll(ctx context.Context, userID, personID string, createTransaction bool, categoryID *string) ([]domain.DebtEntry, []domain.Transaction, error)

	Delete(ctx context.Context, userID, id string) error

	// HasAny reports whether personID has any debt entry at all, settled or
	// not — used to guard deleting a person, since debt_entries.person_id
	// has no ON DELETE cascade and settled history must stay viewable.
	HasAny(ctx context.Context, userID, personID string) (bool, error)
}
