package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

func TestImportRepositoryFlow(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	repo := NewImportRepo(pool)
	user1 := testUser(ctx, t, pool)
	user2 := testUser(ctx, t, pool)
	if err := repo.CreatePairingCode(ctx, user1, "code-hash", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	device, err := repo.RedeemPairingCode(ctx, "code-hash", "token-hash", "Android")
	if err != nil || device.UserID != user1 {
		t.Fatalf("pair: %+v %v", device, err)
	}
	if _, err := repo.RedeemPairingCode(ctx, "code-hash", "other-token", "Again"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("reused code: %v", err)
	}
	if id, err := repo.AuthenticateDevice(ctx, "token-hash"); err != nil || id != user1 {
		t.Fatalf("auth: %s %v", id, err)
	}
	s := domain.ImportSuggestion{UserID: user1, Source: "meezan_sms", SourceID: "source", Kind: domain.KindExpense, AmountPaisa: 50600, OccurredAt: time.Now(), Description: "Meezan card purchase"}
	first, err := repo.InsertSuggestion(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.InsertSuggestion(ctx, s)
	if err != nil || second.ID != first.ID {
		t.Fatalf("duplicate: %+v %v", second, err)
	}
	if list, err := repo.ListSuggestions(ctx, user2); err != nil || len(list) != 0 {
		t.Fatalf("other user list: %+v %v", list, err)
	}
	entry := domain.Transaction{UserID: user1, Kind: domain.KindExpense, AmountPaisa: 50600, OccurredAt: time.Now()}
	if _, _, err := repo.ConfirmSuggestion(ctx, entry, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.ConfirmSuggestion(ctx, entry, first.ID); !errors.Is(err, domain.ErrImportAlreadyReviewed) {
		t.Fatalf("double confirm: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `select count(*) from transactions where user_id=$1`, user1).Scan(&count); err != nil || count != 1 {
		t.Fatalf("transaction count=%d err=%v", count, err)
	}
	if err := repo.RevokeDevice(ctx, user2, device.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-user revoke: %v", err)
	}
	if err := repo.RevokeDevice(ctx, user1, device.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AuthenticateDevice(ctx, "token-hash"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("revoked device: %v", err)
	}
}
