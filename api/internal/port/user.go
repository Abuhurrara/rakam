package port

import (
	"context"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type UserRepo interface {
	GetByEmail(ctx context.Context, email string) (domain.User, error)
	GetByID(ctx context.Context, id string) (domain.User, error)
	GetSessionVersion(ctx context.Context, id string) (int, error)
	Create(ctx context.Context, email, name, passwordHash string) (domain.User, error)
	UpdatePassword(ctx context.Context, id, passwordHash string) (int, error)
}
