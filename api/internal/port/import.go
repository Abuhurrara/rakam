package port

import (
	"context"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type ImportRepo interface {
	CreatePairingCode(ctx context.Context, userID, codeHash string, expiresAt time.Time) error
	RedeemPairingCode(ctx context.Context, codeHash, tokenHash, name string) (domain.ImportDevice, error)
	AuthenticateDevice(ctx context.Context, tokenHash string) (string, error)
	ListDevices(ctx context.Context, userID string) ([]domain.ImportDevice, error)
	RevokeDevice(ctx context.Context, userID, deviceID string) error
	InsertSuggestion(ctx context.Context, suggestion domain.ImportSuggestion) (domain.ImportSuggestion, error)
	ListSuggestions(ctx context.Context, userID string) ([]domain.ImportSuggestion, error)
	IgnoreSuggestion(ctx context.Context, userID, id string) (domain.ImportSuggestion, error)
	ConfirmSuggestion(ctx context.Context, t domain.Transaction, suggestionID string) (domain.ImportSuggestion, domain.Transaction, error)
}
