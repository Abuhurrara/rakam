package service

import (
	"context"
	"fmt"
	"github.com/Abuhurrara/rakam/api/internal/domain"
	"regexp"
)

var requestKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)

func (s *TransactionService) CreateIdempotent(ctx context.Context, t domain.Transaction, key string) (domain.Transaction, error) {
	// Old installed clients remain usable during a staggered deployment.
	if key == "" {
		return s.Create(ctx, t)
	}
	if !requestKeyPattern.MatchString(key) {
		return domain.Transaction{}, fmt.Errorf("%w: invalid Idempotency-Key", domain.ErrInvalidTransaction)
	}
	if err := validateTransaction(t); err != nil {
		return domain.Transaction{}, err
	}
	if err := s.checkCategory(ctx, t); err != nil {
		return domain.Transaction{}, err
	}
	saved, err := s.txRepo.CreateIdempotent(ctx, t, key)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("saving transaction: %w", err)
	}
	return saved, nil
}
