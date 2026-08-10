package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/service"
)

type personRequest struct {
	Name  string  `json:"name"`
	Phone *string `json:"phone"`
	Notes *string `json:"notes"`
}

type personResponse struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Phone        *string `json:"phone"`
	Notes        *string `json:"notes"`
	BalancePaisa int64   `json:"balance_paisa"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

func toPersonResponse(pb domain.PersonBalance) personResponse {
	return personResponse{
		ID:           pb.Person.ID,
		Name:         pb.Person.Name,
		Phone:        pb.Person.Phone,
		Notes:        pb.Person.Notes,
		BalancePaisa: int64(pb.BalancePaisa),
		CreatedAt:    pb.Person.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    pb.Person.UpdatedAt.Format(time.RFC3339),
	}
}

func handleListPeople(svc *service.PersonService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		people, err := svc.List(r.Context(), userID)
		if err != nil {
			writeError(w, err)
			return
		}
		responses := make([]personResponse, len(people))
		for i, p := range people {
			responses[i] = toPersonResponse(p)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responses)
	}
}

func handleCreatePerson(svc *service.PersonService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		var req personRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrInvalidPerson))
			return
		}
		created, err := svc.Create(r.Context(), domain.Person{
			UserID: userID,
			Name:   req.Name,
			Phone:  req.Phone,
			Notes:  req.Notes,
		})
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(toPersonResponse(domain.PersonBalance{Person: created}))
	}
}

func handleDeletePerson(svc *service.PersonService) http.HandlerFunc {
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
