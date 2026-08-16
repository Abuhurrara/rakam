package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/service"
)

type billRequest struct {
	Name       string  `json:"name"`
	Amount     string  `json:"amount"`
	CategoryID *string `json:"category_id"`
	DayOfMonth int     `json:"day_of_month"`
	IsActive   bool    `json:"is_active"`
}

type billResponse struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	AmountPaisa        int64   `json:"amount_paisa"`
	CategoryID         *string `json:"category_id"`
	DayOfMonth         int     `json:"day_of_month"`
	IsActive           bool    `json:"is_active"`
	LastGeneratedMonth *string `json:"last_generated_month"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
}

func toBillResponse(b domain.RecurringBill) billResponse {
	var lastGenerated *string
	if b.LastGeneratedMonth != nil {
		s := b.LastGeneratedMonth.Format("2006-01")
		lastGenerated = &s
	}
	return billResponse{
		ID:                 b.ID,
		Name:               b.Name,
		AmountPaisa:        int64(b.AmountPaisa),
		CategoryID:         b.CategoryID,
		DayOfMonth:         b.DayOfMonth,
		IsActive:           b.IsActive,
		LastGeneratedMonth: lastGenerated,
		CreatedAt:          b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          b.UpdatedAt.Format(time.RFC3339),
	}
}

func billFromRequest(userID string, req billRequest) (domain.RecurringBill, error) {
	amount, err := domain.ParseMoney(req.Amount)
	if err != nil {
		return domain.RecurringBill{}, err
	}
	return domain.RecurringBill{
		UserID:      userID,
		Name:        req.Name,
		AmountPaisa: amount,
		CategoryID:  req.CategoryID,
		DayOfMonth:  req.DayOfMonth,
		IsActive:    req.IsActive,
	}, nil
}

func handleListBills(svc *service.RecurringBillService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		bills, err := svc.List(r.Context(), userID)
		if err != nil {
			writeError(w, err)
			return
		}
		responses := make([]billResponse, len(bills))
		for i, b := range bills {
			responses[i] = toBillResponse(b)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responses)
	}
}

func handleCreateBill(svc *service.RecurringBillService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		var req billRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrInvalidRecurringBill))
			return
		}
		b, err := billFromRequest(userID, req)
		if err != nil {
			writeError(w, err)
			return
		}
		created, err := svc.Create(r.Context(), b)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(toBillResponse(created))
	}
}

func handleUpdateBill(svc *service.RecurringBillService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		id := r.PathValue("id")
		var req billRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrInvalidRecurringBill))
			return
		}
		b, err := billFromRequest(userID, req)
		if err != nil {
			writeError(w, err)
			return
		}
		b.ID = id
		updated, err := svc.Update(r.Context(), b)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(toBillResponse(updated))
	}
}

func handleDeleteBill(svc *service.RecurringBillService) http.HandlerFunc {
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
