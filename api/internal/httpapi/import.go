package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/service"
)

type importResponse struct {
	ID            string  `json:"id"`
	Source        string  `json:"source"`
	Kind          string  `json:"kind"`
	AmountPaisa   int64   `json:"amount_paisa"`
	OccurredAt    string  `json:"occurred_at"`
	Description   string  `json:"description"`
	Status        string  `json:"status"`
	TransactionID *string `json:"transaction_id"`
}

func toImportResponse(s domain.ImportSuggestion) importResponse {
	return importResponse{ID: s.ID, Source: s.Source, Kind: string(s.Kind), AmountPaisa: int64(s.AmountPaisa), OccurredAt: s.OccurredAt.Format(time.RFC3339), Description: s.Description, Status: s.Status, TransactionID: s.TransactionID}
}

func decodeImportRequest(w http.ResponseWriter, r *http.Request, dest any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		return fmt.Errorf("%w: invalid JSON body", domain.ErrInvalidImport)
	}
	return nil
}

func handleCreatePairingCode(svc *service.ImportService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		code, expires, err := svc.CreatePairingCode(r.Context(), userID)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(map[string]string{"code": code, "expires_at": expires.Format(time.RFC3339)})
	}
}

func handlePairDevice(svc *service.ImportService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Code string `json:"code"`
			Name string `json:"name"`
		}
		if err := decodeImportRequest(w, r, &req); err != nil {
			writeError(w, err)
			return
		}
		d, token, err := svc.PairDevice(r.Context(), req.Code, req.Name)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(map[string]string{"device_id": d.ID, "token": token})
	}
}

func requireImportDevice(svc *service.ImportService, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := r.Header.Get("Authorization")
		if !strings.HasPrefix(value, "Bearer ") {
			writeError(w, domain.ErrUnauthorized)
			return
		}
		userID, err := svc.AuthenticateDevice(r.Context(), strings.TrimPrefix(value, "Bearer "))
		if err != nil {
			writeError(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDContextKey, userID)))
	})
}

func handleListImportDevices(svc *service.ImportService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		items, err := svc.ListDevices(r.Context(), userID)
		if err != nil {
			writeError(w, err)
			return
		}
		out := make([]map[string]string, 0, len(items))
		for _, d := range items {
			out = append(out, map[string]string{"id": d.ID, "name": d.Name, "created_at": d.CreatedAt.Format(time.RFC3339)})
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(map[string]any{"devices": out})
	}
}

func handleRevokeImportDevice(svc *service.ImportService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		if err := svc.RevokeDevice(r.Context(), userID, r.PathValue("id")); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleIngestSMS(svc *service.ImportService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		var req struct {
			SourceID   string `json:"source_id"`
			Amount     string `json:"amount"`
			OccurredAt string `json:"occurred_at"`
		}
		if err := decodeImportRequest(w, r, &req); err != nil {
			writeError(w, err)
			return
		}
		amount, err := domain.ParseMoney(req.Amount)
		if err != nil {
			writeError(w, err)
			return
		}
		occurred, err := time.Parse(time.RFC3339, req.OccurredAt)
		if err != nil {
			writeError(w, domain.ErrInvalidImport)
			return
		}
		s, err := svc.Ingest(r.Context(), domain.ImportSuggestion{UserID: userID, Source: "meezan_sms", SourceID: req.SourceID, Kind: domain.KindExpense, AmountPaisa: amount, OccurredAt: occurred, Description: "Meezan card purchase"})
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(toImportResponse(s))
	}
}

func handleListImports(svc *service.ImportService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		items, err := svc.List(r.Context(), userID)
		if err != nil {
			writeError(w, err)
			return
		}
		out := make([]importResponse, len(items))
		for i, s := range items {
			out[i] = toImportResponse(s)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(map[string]any{"suggestions": out})
	}
}

func handleIgnoreImport(svc *service.ImportService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		s, err := svc.Ignore(r.Context(), userID, r.PathValue("id"))
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(toImportResponse(s))
	}
}

func handleConfirmImport(svc *service.ImportService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		var req transactionRequest
		if err := decodeImportRequest(w, r, &req); err != nil {
			writeError(w, err)
			return
		}
		t, err := transactionFromRequest(userID, req)
		if err != nil {
			writeError(w, err)
			return
		}
		s, t, err := svc.Confirm(r.Context(), t, r.PathValue("id"))
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"suggestion": toImportResponse(s), "transaction": toTransactionResponse(t)})
	}
}
