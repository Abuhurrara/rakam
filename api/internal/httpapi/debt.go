package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/service"
)

type debtEntryRequest struct {
	Direction   string `json:"direction"`
	Amount      string `json:"amount"`
	Description string `json:"description"`
	IncurredAt  string `json:"incurred_at"`
}

type debtEntryResponse struct {
	ID          string  `json:"id"`
	PersonID    string  `json:"person_id"`
	Direction   string  `json:"direction"`
	AmountPaisa int64   `json:"amount_paisa"`
	Description string  `json:"description"`
	IncurredAt  string  `json:"incurred_at"`
	SettledAt   *string `json:"settled_at"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func toDebtEntryResponse(e domain.DebtEntry) debtEntryResponse {
	var settledAt *string
	if e.SettledAt != nil {
		s := e.SettledAt.Format(time.RFC3339)
		settledAt = &s
	}
	return debtEntryResponse{
		ID:          e.ID,
		PersonID:    e.PersonID,
		Direction:   string(e.Direction),
		AmountPaisa: int64(e.AmountPaisa),
		Description: e.Description,
		IncurredAt:  e.IncurredAt.Format(time.RFC3339),
		SettledAt:   settledAt,
		CreatedAt:   e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   e.UpdatedAt.Format(time.RFC3339),
	}
}

func handleListDebtEntries(svc *service.DebtService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		personID := r.PathValue("id")
		entries, err := svc.ListForPerson(r.Context(), userID, personID)
		if err != nil {
			writeError(w, err)
			return
		}
		responses := make([]debtEntryResponse, len(entries))
		for i, e := range entries {
			responses[i] = toDebtEntryResponse(e)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responses)
	}
}

func handleCreateDebtEntry(svc *service.DebtService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		personID := r.PathValue("id")
		var req debtEntryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrInvalidDebtEntry))
			return
		}
		amount, err := domain.ParseMoney(req.Amount)
		if err != nil {
			writeError(w, err)
			return
		}
		incurredAt, err := time.Parse(time.RFC3339, req.IncurredAt)
		if err != nil {
			writeError(w, fmt.Errorf("%w: incurred_at must be RFC3339", domain.ErrInvalidDebtEntry))
			return
		}
		created, err := svc.Create(r.Context(), domain.DebtEntry{
			UserID:      userID,
			PersonID:    personID,
			Direction:   domain.DebtDirection(req.Direction),
			AmountPaisa: amount,
			Description: req.Description,
			IncurredAt:  incurredAt,
		})
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(toDebtEntryResponse(created))
	}
}

// settleRequest is shared by both the single-entry and settle-all bodies —
// they take the same two optional fields, and a settle with no flags at all
// (the common case) is a valid empty body.
type settleRequest struct {
	CreateTransaction bool    `json:"create_transaction"`
	CategoryID        *string `json:"category_id"`
}

func decodeSettleRequest(r *http.Request) (settleRequest, error) {
	var req settleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return settleRequest{}, fmt.Errorf("%w: invalid JSON body", domain.ErrInvalidDebtEntry)
	}
	return req, nil
}

type settleResponse struct {
	DebtEntry   debtEntryResponse    `json:"debt_entry"`
	Transaction *transactionResponse `json:"transaction"`
}

func handleSettleDebtEntry(svc *service.DebtService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		id := r.PathValue("id")
		req, err := decodeSettleRequest(r)
		if err != nil {
			writeError(w, err)
			return
		}
		entry, tx, err := svc.Settle(r.Context(), userID, id, req.CreateTransaction, req.CategoryID)
		if err != nil {
			writeError(w, err)
			return
		}
		resp := settleResponse{DebtEntry: toDebtEntryResponse(entry)}
		if tx != nil {
			txResp := toTransactionResponse(*tx)
			resp.Transaction = &txResp
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

type settleAllResponse struct {
	DebtEntries  []debtEntryResponse   `json:"debt_entries"`
	Transactions []transactionResponse `json:"transactions"`
}

func handleSettleAllDebtEntries(svc *service.DebtService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		personID := r.PathValue("id")
		req, err := decodeSettleRequest(r)
		if err != nil {
			writeError(w, err)
			return
		}
		entries, txs, err := svc.SettleAll(r.Context(), userID, personID, req.CreateTransaction, req.CategoryID)
		if err != nil {
			writeError(w, err)
			return
		}
		entryResponses := make([]debtEntryResponse, len(entries))
		for i, e := range entries {
			entryResponses[i] = toDebtEntryResponse(e)
		}
		txResponses := make([]transactionResponse, len(txs))
		for i, t := range txs {
			txResponses[i] = toTransactionResponse(t)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(settleAllResponse{DebtEntries: entryResponses, Transactions: txResponses})
	}
}

func handleDeleteDebtEntry(svc *service.DebtService) http.HandlerFunc {
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
