package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/service"
)

type budgetResponse struct {
	ID         string `json:"id"`
	CategoryID string `json:"category_id"`
	Month      string `json:"month"`
	LimitPaisa int64  `json:"limit_paisa"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func toBudgetResponse(b domain.Budget) budgetResponse {
	return budgetResponse{
		ID:         b.ID,
		CategoryID: b.CategoryID,
		Month:      b.Month.Format("2006-01"),
		LimitPaisa: int64(b.LimitPaisa),
		CreatedAt:  b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  b.UpdatedAt.Format(time.RFC3339),
	}
}

type budgetWithSpentResponse struct {
	Category   categoryResponse `json:"category"`
	Budget     *budgetResponse  `json:"budget"`
	SpentPaisa int64            `json:"spent_paisa"`
}

func toBudgetWithSpentResponse(b domain.BudgetWithSpent) budgetWithSpentResponse {
	resp := budgetWithSpentResponse{
		Category:   toCategoryResponse(b.Category),
		SpentPaisa: int64(b.SpentPaisa),
	}
	if b.Budget != nil {
		br := toBudgetResponse(*b.Budget)
		resp.Budget = &br
	}
	return resp
}

func handleListBudgets(svc *service.BudgetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		month := r.URL.Query().Get("month")
		if month == "" {
			writeError(w, fmt.Errorf("%w: month is required", domain.ErrInvalidMonth))
			return
		}
		rows, err := svc.ListForMonth(r.Context(), userID, month)
		if err != nil {
			writeError(w, err)
			return
		}
		responses := make([]budgetWithSpentResponse, len(rows))
		for i, row := range rows {
			responses[i] = toBudgetWithSpentResponse(row)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responses)
	}
}

type budgetUpsertRequest struct {
	CategoryID string `json:"category_id"`
	Month      string `json:"month"`
	Limit      string `json:"limit"`
}

func handleUpsertBudget(svc *service.BudgetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		var req budgetUpsertRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrInvalidBudget))
			return
		}
		limit, err := domain.ParseMoney(req.Limit)
		if err != nil {
			writeError(w, err)
			return
		}
		upserted, err := svc.Upsert(r.Context(), userID, req.CategoryID, req.Month, limit)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(toBudgetResponse(upserted))
	}
}

func handleDeleteBudget(svc *service.BudgetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		id := r.PathValue("id")
		if err := svc.Delete(r.Context(), userID, id); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
