package port

import (
	"context"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type CategoryRepo interface {
	List(ctx context.Context, userID string) ([]domain.Category, error)
	Create(ctx context.Context, c domain.Category) (domain.Category, error)
	Update(ctx context.Context, c domain.Category) (domain.Category, error)
	Archive(ctx context.Context, userID, id string) error
}
