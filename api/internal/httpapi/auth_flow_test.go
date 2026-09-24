package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Abuhurrara/rakam/api/internal/postgres"
	"github.com/Abuhurrara/rakam/api/internal/service"
)

func TestPrivateAccountHTTPFlowWithPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting to test database: %v", err)
	}
	defer pool.Close()

	users := postgres.NewUserRepo(pool)
	admin := service.NewUserAdminService(users)
	authSvc := service.NewAuthService(users, []byte("test-secret-with-at-least-32-bytes-long"))
	email := fmt.Sprintf("account-flow-%d@example.com", time.Now().UnixNano())
	initialPassword := "first-secure-password"
	changedPassword := "second-secure-password"
	resetPassword := "third-secure-password"
	user, err := admin.Create(ctx, email, "Flow Test", initialPassword)
	if err != nil {
		t.Fatalf("creating empty account: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `delete from users where id = $1`, user.ID) })

	var categories, people, transactions, budgets, bills int
	for _, check := range []struct {
		table string
		count *int
	}{{"categories", &categories}, {"people", &people}, {"transactions", &transactions}, {"budgets", &budgets}, {"recurring_bills", &bills}} {
		query := "select count(*) from " + check.table + " where user_id = $1"
		if err := pool.QueryRow(ctx, query, user.ID).Scan(check.count); err != nil {
			t.Fatalf("counting new account's %s: %v", check.table, err)
		}
		if *check.count != 0 {
			t.Fatalf("new account has %d %s rows, want none", *check.count, check.table)
		}
	}

	login := handleLogin(authSvc)
	loginWith := func(password string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(loginRequest{Email: "  " + strings.ToUpper(email) + " ", Password: password})
		recorder := httptest.NewRecorder()
		login.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body)))
		return recorder
	}
	me := requireAuth(authSvc, handleMe(authSvc))
	checkMe := func(cookie *http.Cookie, wantStatus int) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		req.AddCookie(cookie)
		me.ServeHTTP(recorder, req)
		if recorder.Code != wantStatus {
			t.Fatalf("GET /api/auth/me status = %d, want %d: %s", recorder.Code, wantStatus, recorder.Body.String())
		}
		return recorder
	}

	loginResponse := loginWith(initialPassword)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("initial login status = %d: %s", loginResponse.Code, loginResponse.Body.String())
	}
	if cookies := loginResponse.Result().Cookies(); len(cookies) == 0 {
		t.Fatal("successful login did not issue a session cookie")
	}
	oldCookie := loginResponse.Result().Cookies()[0]
	checkMe(oldCookie, http.StatusOK)

	change := requireAuth(authSvc, handleChangePassword(authSvc))
	changeBody, _ := json.Marshal(changePasswordRequest{CurrentPassword: initialPassword, NewPassword: changedPassword})
	changeRequest := httptest.NewRequest(http.MethodPut, "/api/auth/password", bytes.NewReader(changeBody))
	changeRequest.AddCookie(oldCookie)
	changeResponse := httptest.NewRecorder()
	change.ServeHTTP(changeResponse, changeRequest)
	if changeResponse.Code != http.StatusNoContent {
		t.Fatalf("password change status = %d: %s", changeResponse.Code, changeResponse.Body.String())
	}
	if cookies := changeResponse.Result().Cookies(); len(cookies) == 0 {
		t.Fatal("successful password change did not refresh the session cookie")
	}
	changedCookie := changeResponse.Result().Cookies()[0]
	checkMe(oldCookie, http.StatusUnauthorized)
	checkMe(changedCookie, http.StatusOK)
	if loginResponse := loginWith(changedPassword); loginResponse.Code != http.StatusOK {
		t.Fatalf("login with changed password status = %d", loginResponse.Code)
	}

	if err := admin.ResetPassword(ctx, email, resetPassword); err != nil {
		t.Fatalf("owner password reset: %v", err)
	}
	checkMe(changedCookie, http.StatusUnauthorized)
	if response := loginWith(changedPassword); response.Code != http.StatusUnauthorized {
		t.Fatalf("old password after reset status = %d, want 401", response.Code)
	}
	if response := loginWith(resetPassword); response.Code != http.StatusOK {
		t.Fatalf("login after owner reset status = %d: %s", response.Code, response.Body.String())
	}
}
