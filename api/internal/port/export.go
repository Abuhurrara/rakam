package port

import (
	"context"
	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type ExportRepo interface {
	Snapshot(ctx context.Context, userID string) (domain.LedgerExport, error)
}
