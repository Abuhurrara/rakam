package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Abuhurrara/rakam/api/internal/auth"
)

func TestRequireAuth(t *testing.T) {
	secret := []byte("a-secret-that-is-at-least-32-bytes-long")
	otherSecret := []byte("a-different-secret-of-at-least-32-bytes")

	validToken, err := auth.IssueToken(secret, "user-1")
	if err != nil {
		t.Fatalf("issuing token: %v", err)
	}
	wrongSecretToken, err := auth.IssueToken(otherSecret, "user-1")
	if err != nil {
		t.Fatalf("issuing token: %v", err)
	}

	tests := []struct {
		name       string
		cookie     *http.Cookie
		wantStatus int
		wantCalled bool
	}{
		{name: "no cookie", wantStatus: http.StatusUnauthorized, wantCalled: false},
		{name: "garbage cookie value", cookie: &http.Cookie{Name: cookieName, Value: "not-a-jwt"}, wantStatus: http.StatusUnauthorized, wantCalled: false},
		{name: "wrong secret", cookie: &http.Cookie{Name: cookieName, Value: wrongSecretToken}, wantStatus: http.StatusUnauthorized, wantCalled: false},
		{name: "valid cookie", cookie: &http.Cookie{Name: cookieName, Value: validToken}, wantStatus: http.StatusOK, wantCalled: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var called bool
			var gotUserID string
			spy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				gotUserID, _ = UserIDFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			handler := requireAuth(secret, spy)

			req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("got status %d, want %d", rec.Code, tt.wantStatus)
			}
			if called != tt.wantCalled {
				t.Fatalf("got called=%v, want %v", called, tt.wantCalled)
			}
			if tt.wantCalled && gotUserID != "user-1" {
				t.Fatalf("got userID %q, want %q", gotUserID, "user-1")
			}
		})
	}
}
