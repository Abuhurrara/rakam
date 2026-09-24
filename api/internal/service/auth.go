package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

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
	email = normalizeEmail(email)
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

	token, err := auth.IssueTokenVersion(s.secret, user.ID, user.SessionVersion)
	if err != nil {
		return "", domain.User{}, fmt.Errorf("issuing token: %w", err)
	}
	return token, user, nil
}

func (s *AuthService) ValidateSession(ctx context.Context, token string) (string, error) {
	userID, version, err := auth.VerifyTokenVersion(s.secret, token)
	if err != nil {
		return "", domain.ErrUnauthorized
	}
	versionInStore, err := s.users.GetSessionVersion(ctx, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return "", domain.ErrUnauthorized
	}
	if err != nil {
		return "", fmt.Errorf("validating session user: %w", err)
	}
	if versionInStore != version {
		return "", domain.ErrUnauthorized
	}
	return userID, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) (string, error) {
	if err := validatePassword(newPassword); err != nil {
		return "", err
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("fetching user: %w", err)
	}
	if !auth.VerifyPassword(user.PasswordHash, currentPassword) {
		return "", domain.ErrInvalidCurrentPassword
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return "", fmt.Errorf("hashing new password: %w", err)
	}
	version, err := s.users.UpdatePassword(ctx, userID, hash)
	if err != nil {
		return "", fmt.Errorf("updating password: %w", err)
	}
	token, err := auth.IssueTokenVersion(s.secret, userID, version)
	if err != nil {
		return "", fmt.Errorf("issuing refreshed session: %w", err)
	}
	return token, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validatePassword(password string) error {
	if len(password) < 12 || len(password) > 72 {
		return domain.ErrInvalidPassword
	}
	return nil
}

type UserAdminService struct{ users port.UserRepo }

func NewUserAdminService(users port.UserRepo) *UserAdminService {
	return &UserAdminService{users: users}
}

func (s *UserAdminService) Create(ctx context.Context, email, name, password string) (domain.User, error) {
	email = normalizeEmail(email)
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(email, "@") || strings.TrimSpace(name) == "" {
		return domain.User{}, domain.ErrInvalidUser
	}
	if err := validatePassword(password); err != nil {
		return domain.User{}, err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return domain.User{}, fmt.Errorf("hashing password: %w", err)
	}
	user, err := s.users.Create(ctx, email, strings.TrimSpace(name), hash)
	if err != nil {
		return domain.User{}, fmt.Errorf("creating account: %w", err)
	}
	return user, nil
}

func (s *UserAdminService) ResetPassword(ctx context.Context, email, password string) error {
	email = normalizeEmail(email)
	if err := validatePassword(password); err != nil {
		return err
	}
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("looking up account: %w", err)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}
	if _, err := s.users.UpdatePassword(ctx, user.ID, hash); err != nil {
		return fmt.Errorf("resetting password: %w", err)
	}
	return nil
}

func (s *AuthService) Me(ctx context.Context, userID string) (domain.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return domain.User{}, fmt.Errorf("fetching user: %w", err)
	}
	return user, nil
}
