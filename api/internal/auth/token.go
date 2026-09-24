package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

// TokenTTL is the session token lifetime. It is the single source of
// truth for both the JWT expiry and the session cookie's MaxAge.
const TokenTTL = 30 * 24 * time.Hour

type Claims struct {
	jwt.RegisteredClaims
	SessionVersion int `json:"session_version"`
}

func IssueToken(secret []byte, userID string) (string, error) {
	return IssueTokenVersion(secret, userID, 0)
}

func IssueTokenVersion(secret []byte, userID string, version int) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
		},
		SessionVersion: version,
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}
	return token, nil
}

// VerifyToken parses and validates tokenString, returning the user ID
// carried in its subject claim.
//
// The keyfunc rejects any token whose signing method is not HMAC before
// handing back the secret. This is the alg-confusion defense: without
// it, a caller could hand the parser a token whose header claims a
// different algorithm (or "none") and the token's own header would
// dictate how it gets verified.
func VerifyToken(secret []byte, tokenString string) (string, error) {
	userID, _, err := VerifyTokenVersion(secret, tokenString)
	return userID, err
}

func VerifyTokenVersion(secret []byte, tokenString string) (string, int, error) {
	keyFunc := func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	}

	var claims Claims
	_, err := jwt.ParseWithClaims(tokenString, &claims, keyFunc, jwt.WithExpirationRequired())
	if err != nil {
		return "", 0, fmt.Errorf("%w: %v", domain.ErrUnauthorized, err)
	}

	if claims.Subject == "" {
		return "", 0, fmt.Errorf("%w: empty subject claim", domain.ErrUnauthorized)
	}

	return claims.Subject, claims.SessionVersion, nil
}
