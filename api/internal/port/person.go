package port

import (
	"context"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type PersonRepo interface {
	List(ctx context.Context, userID string) ([]domain.PersonBalance, error)
	Get(ctx context.Context, userID, id string) (domain.Person, error)
	Create(ctx context.Context, p domain.Person) (domain.Person, error)
	Delete(ctx context.Context, userID, id string) error
}
