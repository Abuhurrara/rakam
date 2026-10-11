package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type fakeImportRepo struct {
	codes       map[string]string
	devices     map[string]string
	suggestions map[string]domain.ImportSuggestion
	confirmed   int
}

func newFakeImportRepo() *fakeImportRepo {
	return &fakeImportRepo{codes: map[string]string{}, devices: map[string]string{}, suggestions: map[string]domain.ImportSuggestion{}}
}
func (f *fakeImportRepo) CreatePairingCode(_ context.Context, userID, hash string, _ time.Time) error {
	for oldHash, owner := range f.codes {
		if owner == userID {
			delete(f.codes, oldHash)
		}
	}
	f.codes[hash] = userID
	return nil
}
func (f *fakeImportRepo) RedeemPairingCode(_ context.Context, hash, tokenHash, name string) (domain.ImportDevice, error) {
	userID := f.codes[hash]
	if userID == "" {
		return domain.ImportDevice{}, domain.ErrInvalidCredentials
	}
	delete(f.codes, hash)
	f.devices[tokenHash] = userID
	return domain.ImportDevice{ID: "device", UserID: userID, Name: name}, nil
}
func (f *fakeImportRepo) AuthenticateDevice(_ context.Context, hash string) (string, error) {
	id := f.devices[hash]
	if id == "" {
		return "", domain.ErrUnauthorized
	}
	return id, nil
}
func (f *fakeImportRepo) ListDevices(_ context.Context, userID string) ([]domain.ImportDevice, error) {
	return []domain.ImportDevice{}, nil
}
func (f *fakeImportRepo) RevokeDevice(_ context.Context, userID, deviceID string) error { return nil }
func (f *fakeImportRepo) InsertSuggestion(_ context.Context, s domain.ImportSuggestion) (domain.ImportSuggestion, error) {
	key := s.UserID + ":" + s.SourceID
	if old, ok := f.suggestions[key]; ok {
		return old, nil
	}
	s.ID = key
	s.Status = "pending"
	f.suggestions[key] = s
	return s, nil
}
func (f *fakeImportRepo) ListSuggestions(_ context.Context, userID string) ([]domain.ImportSuggestion, error) {
	var out []domain.ImportSuggestion
	for _, s := range f.suggestions {
		if s.UserID == userID && s.Status == "pending" {
			out = append(out, s)
		}
	}
	return out, nil
}
func (f *fakeImportRepo) IgnoreSuggestion(_ context.Context, userID, id string) (domain.ImportSuggestion, error) {
	s, ok := f.suggestions[id]
	if !ok || s.UserID != userID || s.Status != "pending" {
		return domain.ImportSuggestion{}, domain.ErrNotFound
	}
	s.Status = "ignored"
	f.suggestions[id] = s
	return s, nil
}
func (f *fakeImportRepo) ConfirmSuggestion(_ context.Context, t domain.Transaction, id string) (domain.ImportSuggestion, domain.Transaction, error) {
	s, ok := f.suggestions[id]
	if !ok || s.UserID != t.UserID {
		return domain.ImportSuggestion{}, domain.Transaction{}, domain.ErrNotFound
	}
	if s.Status != "pending" {
		return domain.ImportSuggestion{}, domain.Transaction{}, domain.ErrImportAlreadyReviewed
	}
	s.Status = "confirmed"
	f.suggestions[id] = s
	f.confirmed++
	t.ID = "transaction"
	return s, t, nil
}

func TestImportPairingOneTimeAndScoped(t *testing.T) {
	ctx := context.Background()
	repo := newFakeImportRepo()
	svc := NewImportService(repo, nil)
	code, expires, err := svc.CreatePairingCode(ctx, "user-1")
	if err != nil || len(code) != 24 || time.Until(expires) <= 0 {
		t.Fatalf("invalid pairing code: %q %v %v", code, expires, err)
	}
	device, token, err := svc.PairDevice(ctx, code, "My phone")
	if err != nil || device.UserID != "user-1" || len(token) != 43 {
		t.Fatalf("pair: %+v %q %v", device, token, err)
	}
	if _, _, err := svc.PairDevice(ctx, code, "Again"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("reused code: %v", err)
	}
	if user, err := svc.AuthenticateDevice(ctx, token); err != nil || user != "user-1" {
		t.Fatalf("device auth: %q %v", user, err)
	}
	if _, err := svc.AuthenticateDevice(ctx, "garbage"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("bad device token: %v", err)
	}
}

func TestImportIngestValidationAndDeduplication(t *testing.T) {
	ctx := context.Background()
	repo := newFakeImportRepo()
	svc := NewImportService(repo, nil)
	valid := domain.ImportSuggestion{UserID: "user-1", Source: "meezan_sms", SourceID: importHash("alert"), Kind: domain.KindExpense, AmountPaisa: 50600, OccurredAt: time.Now(), Description: ""}
	tests := []struct {
		name   string
		change func(*domain.ImportSuggestion)
		bad    bool
	}{
		{"valid", func(*domain.ImportSuggestion) {}, false},
		{"bad source", func(s *domain.ImportSuggestion) { s.Source = "email" }, true},
		{"income", func(s *domain.ImportSuggestion) { s.Kind = domain.KindIncome }, true},
		{"bad fingerprint", func(s *domain.ImportSuggestion) { s.SourceID = "hello" }, true},
		{"zero amount", func(s *domain.ImportSuggestion) { s.AmountPaisa = 0 }, true},
		{"future date", func(s *domain.ImportSuggestion) { s.OccurredAt = time.Now().Add(48 * time.Hour) }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := valid
			tt.change(&input)
			_, err := svc.Ingest(ctx, input)
			if tt.bad != errors.Is(err, domain.ErrInvalidImport) {
				t.Fatalf("got %v, bad=%v", err, tt.bad)
			}
		})
	}
	first, err := svc.Ingest(ctx, valid)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Ingest(ctx, valid)
	if err != nil || first.ID != second.ID {
		t.Fatalf("duplicate created another suggestion: %+v %+v %v", first, second, err)
	}
	other := valid
	other.UserID = "user-2"
	third, err := svc.Ingest(ctx, other)
	if err != nil || third.ID == first.ID {
		t.Fatalf("user scoping: %+v %+v %v", first, third, err)
	}
}

func TestImportConfirmDoesNotDoublePost(t *testing.T) {
	ctx := context.Background()
	repo := newFakeImportRepo()
	cats := newFakeCategoryRepo()
	svc := NewImportService(repo, NewTransactionService(nil, cats, time.UTC))
	s, err := svc.Ingest(ctx, domain.ImportSuggestion{UserID: "user-1", Source: "meezan_sms", SourceID: importHash("alert"), Kind: domain.KindExpense, AmountPaisa: 50600, OccurredAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	entry := domain.Transaction{UserID: "user-1", Kind: domain.KindExpense, AmountPaisa: 50600, OccurredAt: time.Now()}
	if _, _, err := svc.Confirm(ctx, entry, s.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Confirm(ctx, entry, s.ID); !errors.Is(err, domain.ErrImportAlreadyReviewed) {
		t.Fatalf("second confirm: %v", err)
	}
	if repo.confirmed != 1 {
		t.Fatalf("got %d posts, want 1", repo.confirmed)
	}
	entry.UserID = "user-2"
	if _, _, err := svc.Confirm(ctx, entry, s.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-user confirm: %v", err)
	}
}
