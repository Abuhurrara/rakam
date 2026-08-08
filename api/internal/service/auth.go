package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Abuhurrara/rakam/api/internal/auth"
	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/port"
)

type AuthService struct {
	users  port.UserRepo
	secret []byte
}

func NewAuthService(users port.UserRepo, secret []byte) *AuthService {
	return &AuthService{users: users, secret: secret}
}

// Login verifies email and password and, on success, issues a session
// token. Unknown email and wrong password return the identical error so
// a caller can't use the response to enumerate accounts, and a bcrypt
// compare always runs on the failure path so response timing doesn't
// leak which case occurred.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, domain.User, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			auth.RejectWithConstantTime(password)
			return "", domain.User{}, domain.ErrInvalidCredentials
		}
		return "", domain.User{}, fmt.Errorf("looking up user: %w", err)
	}

	if !auth.VerifyPassword(user.PasswordHash, password) {
		return "", domain.User{}, domain.ErrInvalidCredentials
	}

	token, err := auth.IssueToken(s.secret, user.ID)
	if err != nil {
		return "", domain.User{}, fmt.Errorf("issuing token: %w", err)
	}
	return token, user, nil
}

func (s *AuthService) Me(ctx context.Context, userID string) (domain.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return domain.User{}, fmt.Errorf("fetching user: %w", err)
	}
	return user, nil
}
