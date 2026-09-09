package service

import (
	"context"
	"fmt"
	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/port"
)

type ExportService struct{ repo port.ExportRepo }

func NewExportService(repo port.ExportRepo) *ExportService { return &ExportService{repo: repo} }
func (s *ExportService) Snapshot(ctx context.Context, userID string) (domain.LedgerExport, error) {
	if userID == "" {
		return domain.LedgerExport{}, domain.ErrUnauthorized
	}
	snapshot, err := s.repo.Snapshot(ctx, userID)
	if err != nil {
		return domain.LedgerExport{}, fmt.Errorf("exporting ledger: %w", err)
	}
	return snapshot, nil
}
