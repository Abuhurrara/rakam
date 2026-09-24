package httpapi

import (
	"context"
	"errors"
	"github.com/Abuhurrara/rakam/api/internal/auth"
	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/service"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeExportRepo struct{ seen string }

type exportUsers struct{}

func (exportUsers) GetByEmail(context.Context, string) (domain.User, error) {
	return domain.User{}, domain.ErrNotFound
}
func (exportUsers) GetByID(_ context.Context, id string) (domain.User, error) {
	if id == "session-user" {
		return domain.User{ID: id}, nil
	}
	return domain.User{}, errors.New("missing")
}
func (exportUsers) GetSessionVersion(_ context.Context, id string) (int, error) {
	if id == "session-user" {
		return 0, nil
	}
	return 0, errors.New("missing")
}
func (exportUsers) Create(context.Context, string, string, string) (domain.User, error) {
	return domain.User{}, nil
}
func (exportUsers) UpdatePassword(context.Context, string, string) (int, error) { return 0, nil }

func (f *fakeExportRepo) Snapshot(ctx context.Context, userID string) (domain.LedgerExport, error) {
	f.seen = userID
	return domain.LedgerExport{Content: []byte(`{"version":1,"transactions":[]}`)}, nil
}

func TestExportUsesSessionAndDownloadHeaders(t *testing.T) {
	secret := []byte("test-only-secret-with-at-least-32-bytes")
	for _, tc := range []struct {
		name          string
		authenticated bool
		want          int
	}{{"no session", false, 401}, {"authenticated download", true, 200}} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeExportRepo{}
			handler := requireAuth(service.NewAuthService(exportUsers{}, secret), handleExport(service.NewExportService(repo)))
			req := httptest.NewRequest(http.MethodGet, "/api/export?user_id=someone-else", nil)
			if tc.authenticated {
				token, err := auth.IssueToken(secret, "session-user")
				if err != nil {
					t.Fatal(err)
				}
				req.AddCookie(&http.Cookie{Name: cookieName, Value: token})
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status = %d", w.Code)
			}
			if !tc.authenticated {
				if repo.seen != "" {
					t.Fatal("unauthenticated export reached repository")
				}
				return
			}
			if repo.seen != "session-user" {
				t.Fatalf("wrong export user: %s", repo.seen)
			}
			if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Content-Disposition") == "" {
				t.Fatal("missing download/cache headers")
			}
			if w.Body.String() != `{"version":1,"transactions":[]}` {
				t.Fatal("export body changed")
			}
		})
	}
}
