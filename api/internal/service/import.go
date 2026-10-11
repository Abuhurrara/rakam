package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/port"
)

var sourceIDPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type ImportService struct {
	repo         port.ImportRepo
	transactions *TransactionService
}

func NewImportService(repo port.ImportRepo, transactions *TransactionService) *ImportService {
	return &ImportService{repo: repo, transactions: transactions}
}

func importHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func randomURLToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *ImportService) CreatePairingCode(ctx context.Context, userID string) (string, time.Time, error) {
	code, err := randomURLToken(18)
	if err != nil {
		return "", time.Time{}, err
	}
	expires := time.Now().Add(10 * time.Minute)
	if err := s.repo.CreatePairingCode(ctx, userID, importHash(code), expires); err != nil {
		return "", time.Time{}, fmt.Errorf("saving pairing code: %w", err)
	}
	return code, expires, nil
}

func (s *ImportService) PairDevice(ctx context.Context, code, name string) (domain.ImportDevice, string, error) {
	name = strings.TrimSpace(name)
	if len(name) < 1 || len(name) > 80 || len(code) != 24 {
		return domain.ImportDevice{}, "", domain.ErrInvalidImport
	}
	token, err := randomURLToken(32)
	if err != nil {
		return domain.ImportDevice{}, "", err
	}
	d, err := s.repo.RedeemPairingCode(ctx, importHash(code), importHash(token), name)
	if err != nil {
		return domain.ImportDevice{}, "", fmt.Errorf("pairing import device: %w", err)
	}
	return d, token, nil
}

func (s *ImportService) AuthenticateDevice(ctx context.Context, token string) (string, error) {
	if len(token) != 43 {
		return "", domain.ErrUnauthorized
	}
	return s.repo.AuthenticateDevice(ctx, importHash(token))
}

func (s *ImportService) ListDevices(ctx context.Context, userID string) ([]domain.ImportDevice, error) {
	return s.repo.ListDevices(ctx, userID)
}

func (s *ImportService) RevokeDevice(ctx context.Context, userID, deviceID string) error {
	return s.repo.RevokeDevice(ctx, userID, deviceID)
}

func (s *ImportService) Ingest(ctx context.Context, suggestion domain.ImportSuggestion) (domain.ImportSuggestion, error) {
	if suggestion.Source != "meezan_sms" || suggestion.Kind != domain.KindExpense || !sourceIDPattern.MatchString(suggestion.SourceID) || suggestion.AmountPaisa <= 0 || suggestion.OccurredAt.IsZero() || suggestion.OccurredAt.After(time.Now().Add(24*time.Hour)) || suggestion.OccurredAt.Before(time.Now().AddDate(-3, 0, 0)) || len(suggestion.Description) > 200 {
		return domain.ImportSuggestion{}, domain.ErrInvalidImport
	}
	suggestion.Description = strings.TrimSpace(suggestion.Description)
	if suggestion.Description == "" {
		suggestion.Description = "Meezan card purchase"
	}
	return s.repo.InsertSuggestion(ctx, suggestion)
}

func (s *ImportService) List(ctx context.Context, userID string) ([]domain.ImportSuggestion, error) {
	return s.repo.ListSuggestions(ctx, userID)
}

func (s *ImportService) Ignore(ctx context.Context, userID, id string) (domain.ImportSuggestion, error) {
	return s.repo.IgnoreSuggestion(ctx, userID, id)
}

func (s *ImportService) Confirm(ctx context.Context, t domain.Transaction, suggestionID string) (domain.ImportSuggestion, domain.Transaction, error) {
	if t.Kind != domain.KindExpense {
		return domain.ImportSuggestion{}, domain.Transaction{}, domain.ErrInvalidImport
	}
	if err := validateTransaction(t); err != nil {
		return domain.ImportSuggestion{}, domain.Transaction{}, err
	}
	if err := s.transactions.checkCategory(ctx, t); err != nil {
		return domain.ImportSuggestion{}, domain.Transaction{}, err
	}
	return s.repo.ConfirmSuggestion(ctx, t, suggestionID)
}
