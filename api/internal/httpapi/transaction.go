package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/service"
)

type transactionRequest struct {
	Kind        string  `json:"kind"`
	Amount      string  `json:"amount"`
	CategoryID  *string `json:"category_id"`
	Description *string `json:"description"`
	OccurredAt  string  `json:"occurred_at"`
}

type transactionResponse struct {
	ID              string  `json:"id"`
	Kind            string  `json:"kind"`
	AmountPaisa     int64   `json:"amount_paisa"`
	CategoryID      *string `json:"category_id"`
	Description     *string `json:"description"`
	OccurredAt      string  `json:"occurred_at"`
	RecurringBillID *string `json:"recurring_bill_id"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type listTransactionsResponse struct {
	Transactions []transactionResponse `json:"transactions"`
	Total        int                   `json:"total"`
	Limit        int                   `json:"limit"`
	Offset       int                   `json:"offset"`
}

func toTransactionResponse(t domain.Transaction) transactionResponse {
	return transactionResponse{
		ID:              t.ID,
		Kind:            string(t.Kind),
		AmountPaisa:     int64(t.AmountPaisa),
		CategoryID:      t.CategoryID,
		Description:     t.Description,
		OccurredAt:      t.OccurredAt.Format(time.RFC3339),
		RecurringBillID: t.RecurringBillID,
		CreatedAt:       t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       t.UpdatedAt.Format(time.RFC3339),
	}
}

// transactionFromRequest decodes and validates the wire-format fields that
// need parsing before they become a domain.Transaction: amount (a decimal
// rupee string, per domain.ParseMoney) and occurred_at (RFC3339).
func transactionFromRequest(userID string, req transactionRequest) (domain.Transaction, error) {
	amount, err := domain.ParseMoney(req.Amount)
	if err != nil {
		return domain.Transaction{}, err
	}
	occurredAt, err := time.Parse(time.RFC3339, req.OccurredAt)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("%w: occurred_at must be RFC3339", domain.ErrInvalidTransaction)
	}
	return domain.Transaction{
		UserID:      userID,
		Kind:        domain.Kind(req.Kind),
		AmountPaisa: amount,
		CategoryID:  req.CategoryID,
		Description: req.Description,
		OccurredAt:  occurredAt,
	}, nil
}

func handleListTransactions(svc *service.TransactionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		q := r.URL.Query()

		params := service.ListParams{}
		if month := q.Get("month"); month != "" {
			params.MonthStr = &month
		}
		if categoryID := q.Get("category_id"); categoryID != "" {
			params.CategoryID = &categoryID
		}
		if search := q.Get("q"); search != "" {
			params.Query = &search
		}
		if limit, err := strconv.Atoi(q.Get("limit")); err == nil {
			params.Limit = limit
		}
		if offset, err := strconv.Atoi(q.Get("offset")); err == nil {
			params.Offset = offset
		}

		result, err := svc.List(r.Context(), userID, params)
		if err != nil {
			writeError(w, err)
			return
		}
		responses := make([]transactionResponse, len(result.Transactions))
		for i, t := range result.Transactions {
			responses[i] = toTransactionResponse(t)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(listTransactionsResponse{
			Transactions: responses,
			Total:        result.Total,
			Limit:        result.Limit,
			Offset:       result.Offset,
		})
	}
}

func handleCreateTransaction(svc *service.TransactionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		var req transactionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrInvalidTransaction))
			return
		}
		t, err := transactionFromRequest(userID, req)
		if err != nil {
			writeError(w, err)
			return
		}
		created, err := svc.Create(r.Context(), t)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(toTransactionResponse(created))
	}
}

func handleUpdateTransaction(svc *service.TransactionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		id := r.PathValue("id")
		var req transactionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrInvalidTransaction))
			return
		}
		t, err := transactionFromRequest(userID, req)
		if err != nil {
			writeError(w, err)
			return
		}
		t.ID = id
		updated, err := svc.Update(r.Context(), t)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(toTransactionResponse(updated))
	}
}

func handleDeleteTransaction(svc *service.TransactionService) http.HandlerFunc {
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