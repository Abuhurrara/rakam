package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

func TestVerifyToken_ValidTokenAccepted(t *testing.T) {
	secret := []byte("a-secret-that-is-at-least-32-bytes-long")

	token, err := IssueToken(secret, "user-123")
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}

	userID, err := VerifyToken(secret, token)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	if userID != "user-123" {
		t.Fatalf("VerifyToken() userID = %q, want %q", userID, "user-123")
	}
}

func TestVerifyToken_RejectsInvalidTokens(t *testing.T) {
	secret := []byte("a-secret-that-is-at-least-32-bytes-long")
	otherSecret := []byte("a-different-secret-of-at-least-32-bytes")

	sign := func(method jwt.SigningMethod, claims jwt.Claims, key any) string {
		s, err := jwt.NewWithClaims(method, claims).SignedString(key)
		if err != nil {
			t.Fatalf("signing test token: %v", err)
		}
		return s
	}

	now := time.Now()

	tests := []struct {
		name  string
		token string
	}{
		{
			name: "expired token",
			token: sign(jwt.SigningMethodHS256, Claims{RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "user-123",
				IssuedAt:  jwt.NewNumericDate(now.Add(-2 * TokenTTL)),
				ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
			}}, secret),
		},
		{
			name: "wrong secret",
			token: sign(jwt.SigningMethodHS256, Claims{RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "user-123",
				ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
			}}, otherSecret),
		},
		{
			name: "alg none",
			token: sign(jwt.SigningMethodNone, Claims{RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "user-123",
				ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
			}}, jwt.UnsafeAllowNoneSignatureType),
		},
		{
			name: "empty subject",
			token: sign(jwt.SigningMethodHS256, Claims{RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "",
				ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
			}}, secret),
		},
		{
			name: "missing expiry",
			token: sign(jwt.SigningMethodHS256, Claims{RegisteredClaims: jwt.RegisteredClaims{
				Subject: "user-123",
			}}, secret),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := VerifyToken(secret, tt.token)
			if !errors.Is(err, domain.ErrUnauthorized) {
				t.Fatalf("VerifyToken() error = %v, want errors.Is(err, domain.ErrUnauthorized)", err)
			}
		})
	}
}
