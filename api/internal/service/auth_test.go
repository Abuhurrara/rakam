package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Abuhurrara/rakam/api/internal/auth"
	"github.com/Abuhurrara/rakam/api/internal/domain"
)

type fakeUserRepo struct {
	byEmail map[string]domain.User
	byID    map[string]domain.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byEmail: make(map[string]domain.User), byID: make(map[string]domain.User)}
}

func (f *fakeUserRepo) add(u domain.User) {
	f.byEmail[u.Email] = u
	f.byID[u.ID] = u
}

func (f *fakeUserRepo) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserRepo) GetByID(ctx context.Context, id string) (domain.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

const testSecret = "a-secret-that-is-at-least-32-bytes-long"

func newSeededAuthService(t *testing.T) *AuthService {
	t.Helper()
	hash, err := auth.HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("hashing password: %v", err)
	}
	repo := newFakeUserRepo()
	repo.add(domain.User{ID: "user-1", Email: "abu@example.com", PasswordHash: hash, Name: "Abu"})
	return NewAuthService(repo, []byte(testSecret))
}

func TestAuthService_Login(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		wantErr  error
	}{
		{name: "correct credentials", email: "abu@example.com", password: "correct-horse-battery-staple"},
		{name: "wrong password", email: "abu@example.com", password: "wrong-password", wantErr: domain.ErrInvalidCredentials},
		{name: "unknown email", email: "nobody@example.com", password: "correct-horse-battery-staple", wantErr: domain.ErrInvalidCredentials},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newSeededAuthService(t)

			token, user, err := svc.Login(context.Background(), tt.email, tt.password)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got error %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if token == "" {
				t.Fatal("expected a non-empty token")
			}
			if user.Email != tt.email {
				t.Fatalf("got user email %q, want %q", user.Email, tt.email)
			}
		})
	}
}

// TestAuthService_Login_IdenticalErrorForUnknownEmailAndWrongPassword is the
// enumeration defense's actual observable contract: not just that both
// cases satisfy errors.Is against the same sentinel, but that the two
// error strings a caller could inspect are byte-identical.
func TestAuthService_Login_IdenticalErrorForUnknownEmailAndWrongPassword(t *testing.T) {
	svc := newSeededAuthService(t)
	ctx := context.Background()

	_, _, unknownEmailErr := svc.Login(ctx, "nobody@example.com", "correct-horse-battery-staple")
	_, _, wrongPasswordErr := svc.Login(ctx, "abu@example.com", "wrong-password")

	if unknownEmailErr == nil || wrongPasswordErr == nil {
		t.Fatal("expected both login attempts to fail")
	}
	if unknownEmailErr.Error() != wrongPasswordErr.Error() {
		t.Fatalf("error messages differ: unknown email = %q, wrong password = %q", unknownEmailErr.Error(), wrongPasswordErr.Error())
	}
}

func TestAuthService_Me(t *testing.T) {
	svc := newSeededAuthService(t)
	ctx := context.Background()

	user, err := svc.Me(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "abu@example.com" {
		t.Fatalf("got email %q, want %q", user.Email, "abu@example.com")
	}
}

func TestAuthService_Me_UnknownUser(t *testing.T) {
	svc := newSeededAuthService(t)

	_, err := svc.Me(context.Background(), "no-such-user")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got error %v, want %v", err, domain.ErrNotFound)
	}
}
