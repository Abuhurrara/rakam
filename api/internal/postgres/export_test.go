package postgres

import (
	"context"
	"encoding/json"
	"github.com/Abuhurrara/rakam/api/internal/domain"
	"testing"
	"time"
)

func TestExportSnapshot(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	user := testUser(ctx, t, pool)
	other := testUser(ctx, t, pool)
	cat := testCategory(ctx, t, pool, user, "Archived", "expense")
	if _, err := pool.Exec(ctx, `update categories set is_archived=true where user_id=$1 and id=$2`, user, cat); err != nil {
		t.Fatal(err)
	}
	for _, uid := range []string{user, other} {
		_, err := NewTransactionRepo(pool).Create(ctx, domain.Transaction{UserID: uid, Kind: domain.KindExpense, AmountPaisa: 9007199254740993, OccurredAt: time.Now()})
		if err != nil {
			t.Fatal(err)
		}
	}
	result, err := NewExportRepo(pool).Snapshot(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]json.RawMessage
	if err = json.Unmarshal(result.Content, &data); err != nil {
		t.Fatal(err)
	}
	var account map[string]json.RawMessage
	json.Unmarshal(data["user"], &account)
	if _, ok := account["password_hash"]; ok {
		t.Fatal("export contains password hash")
	}
	var rows []struct {
		UserID string `json:"user_id"`
		Amount int64  `json:"amount_paisa"`
	}
	json.Unmarshal(data["transactions"], &rows)
	if len(rows) != 1 || rows[0].UserID != user || rows[0].Amount != 9007199254740993 {
		t.Fatalf("wrong scope or precision: %+v", rows)
	}
	var cats []struct {
		Archived bool `json:"is_archived"`
	}
	json.Unmarshal(data["categories"], &cats)
	if len(cats) != 1 || !cats[0].Archived {
		t.Fatal("archived category missing")
	}
	for _, name := range []string{"people", "debt_entries", "budgets", "recurring_bills", "work_logs"} {
		if string(data[name]) != "[]" {
			t.Fatalf("%s is not empty array: %s", name, data[name])
		}
	}
}
