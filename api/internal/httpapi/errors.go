package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrInvalidCategory):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrInvalidAmount):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrInvalidTransaction):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrInvalidPerson):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrInvalidDebtEntry):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrAlreadySettled):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrPersonHasDebtEntries):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, domain.ErrInvalidCredentials):
		status = http.StatusUnauthorized
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
