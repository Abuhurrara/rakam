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

func TestSMSImportHTTPFlowWithPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	users := postgres.NewUserRepo(pool)
	admin := service.NewUserAdminService(users)
	secret := []byte("import-test-secret-at-least-32-bytes")
	auth := service.NewAuthService(users, secret)
	first, err := admin.Create(ctx, fmt.Sprintf("import-1-%d@example.com", time.Now().UnixNano()), "First", "secure-password-one")
	if err != nil {
		t.Fatal(err)
	}
	second, err := admin.Create(ctx, fmt.Sprintf("import-2-%d@example.com", time.Now().UnixNano()), "Second", "secure-password-two")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `delete from users where id=$1 or id=$2`, first.ID, second.ID) })
	catRepo := postgres.NewCategoryRepo(pool)
	txRepo := postgres.NewTransactionRepo(pool)
	loc := time.FixedZone("PKT", 5*3600)
	txSvc := service.NewTransactionService(txRepo, catRepo, loc)
	imports := service.NewImportService(postgres.NewImportRepo(pool), txSvc)
	personRepo := postgres.NewPersonRepo(pool)
	debtRepo := postgres.NewDebtRepo(pool)
	personSvc := service.NewPersonService(personRepo, debtRepo)
	budgetSvc := service.NewBudgetService(postgres.NewBudgetRepo(pool), catRepo, loc)
	billSvc := service.NewRecurringBillService(postgres.NewRecurringBillRepo(pool), catRepo, loc)
	summarySvc := service.NewSummaryService(txRepo, budgetSvc, billSvc, personSvc, loc)
	router := NewRouter(nil, txSvc, auth, personSvc, nil, budgetSvc, billSvc, summarySvc, nil, imports, pool)
	call := func(method, path string, body any, cookie *http.Cookie, token string) *httptest.ResponseRecorder {
		var data []byte
		if body != nil {
			data, _ = json.Marshal(body)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(data))
		if cookie != nil {
			req.AddCookie(cookie)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	login := func(email, password string) *http.Cookie {
		res := call("POST", "/api/auth/login", map[string]string{"email": email, "password": password}, nil, "")
		if res.Code != 200 {
			t.Fatalf("login: %d %s", res.Code, res.Body.String())
		}
		return res.Result().Cookies()[0]
	}
	firstCookie := login(first.Email, "secure-password-one")
	secondCookie := login(second.Email, "secure-password-two")
	res := call("POST", "/api/imports/pairing-code", nil, firstCookie, "")
	if res.Code != 200 {
		t.Fatalf("pair code: %d %s", res.Code, res.Body.String())
	}
	var pairing struct {
		Code string `json:"code"`
	}
	json.Unmarshal(res.Body.Bytes(), &pairing)
	res = call("POST", "/api/imports/pair", map[string]string{"code": pairing.Code, "name": "Test Android"}, nil, "")
	if res.Code != 200 {
		t.Fatalf("pair device: %d %s", res.Code, res.Body.String())
	}
	var device struct {
		Token string `json:"token"`
	}
	json.Unmarshal(res.Body.Bytes(), &device)
	if device.Token == "" {
		t.Fatal("no device token")
	}
	res = call("GET", "/api/imports/device-check", nil, nil, "")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized device check: %d", res.Code)
	}
	res = call("GET", "/api/imports/device-check", nil, nil, device.Token)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"status":"ready"`) {
		t.Fatalf("device check: %d %s", res.Code, res.Body.String())
	}
	res = call("POST", "/api/imports/pair", map[string]string{"code": pairing.Code, "name": "Replay"}, nil, "")
	if res.Code != 401 {
		t.Fatalf("pair code replay accepted: %d", res.Code)
	}
	occurred := time.Now().In(loc).Format(time.RFC3339)
	alert := map[string]string{"source_id": strings.Repeat("a", 64), "amount": "506.00", "occurred_at": occurred}
	res = call("POST", "/api/imports/sms", alert, nil, "")
	if res.Code != 401 {
		t.Fatalf("unauthorized ingest: %d", res.Code)
	}
	res = call("POST", "/api/imports/sms", alert, nil, device.Token)
	if res.Code != 201 {
		t.Fatalf("ingest: %d %s", res.Code, res.Body.String())
	}
	var suggestion importResponse
	json.Unmarshal(res.Body.Bytes(), &suggestion)
	res = call("POST", "/api/imports/sms", alert, nil, device.Token)
	var duplicate importResponse
	json.Unmarshal(res.Body.Bytes(), &duplicate)
	if duplicate.ID != suggestion.ID {
		t.Fatalf("duplicate produced new row: %s != %s", duplicate.ID, suggestion.ID)
	}
	res = call("GET", "/api/imports", nil, secondCookie, "")
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"suggestions":[]`) {
		t.Fatalf("other account sees imports: %d %s", res.Code, res.Body.String())
	}
	confirm := map[string]any{"kind": "expense", "amount": "506.00", "category_id": nil, "description": "Card purchase", "occurred_at": occurred}
	res = call("POST", "/api/imports/"+suggestion.ID+"/confirm", confirm, secondCookie, "")
	if res.Code != 404 {
		t.Fatalf("cross-account confirm: %d %s", res.Code, res.Body.String())
	}
	res = call("POST", "/api/imports/"+suggestion.ID+"/confirm", confirm, firstCookie, "")
	if res.Code != 200 {
		t.Fatalf("confirm: %d %s", res.Code, res.Body.String())
	}
	res = call("POST", "/api/imports/"+suggestion.ID+"/confirm", confirm, firstCookie, "")
	if res.Code != 409 {
		t.Fatalf("double confirm: %d %s", res.Code, res.Body.String())
	}
	var total int
	if err := pool.QueryRow(ctx, `select count(*) from transactions where user_id=$1`, first.ID).Scan(&total); err != nil || total != 1 {
		t.Fatalf("transaction total %d, err %v", total, err)
	}
	res = call("GET", "/api/summary", nil, firstCookie, "")
	if res.Code != 200 {
		t.Fatalf("summary status: %d %s", res.Code, res.Body.String())
	}
	var summary struct {
		ExpensePaisa int64 `json:"expense_paisa"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &summary); err != nil || summary.ExpensePaisa != 50600 {
		t.Fatalf("Home total after confirm = %d, error %v", summary.ExpensePaisa, err)
	}
}
