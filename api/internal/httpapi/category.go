package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/service"
)

type categoryRequest struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Icon      string `json:"icon"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
}

type categoryResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Icon       string `json:"icon"`
	Color      string `json:"color"`
	SortOrder  int    `json:"sort_order"`
	IsArchived bool   `json:"is_archived"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func toCategoryResponse(c domain.Category) categoryResponse {
	return categoryResponse{
		ID:         c.ID,
		Name:       c.Name,
		Kind:       string(c.Kind),
		Icon:       c.Icon,
		Color:      c.Color,
		SortOrder:  c.SortOrder,
		IsArchived: c.IsArchived,
		CreatedAt:  c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  c.UpdatedAt.Format(time.RFC3339),
	}
}

func handleListCategories(svc *service.CategoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		categories, err := svc.List(r.Context(), userID)
		if err != nil {
			writeError(w, err)
			return
		}
		responses := make([]categoryResponse, len(categories))
		for i, c := range categories {
			responses[i] = toCategoryResponse(c)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responses)
	}
}

func handleCreateCategory(svc *service.CategoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		var req categoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrInvalidCategory))
			return
		}
		c := domain.Category{
			UserID:    userID,
			Name:      req.Name,
			Kind:      domain.Kind(req.Kind),
			Icon:      req.Icon,
			Color:     req.Color,
			SortOrder: req.SortOrder,
		}
		created, err := svc.Create(r.Context(), c)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(toCategoryResponse(created))
	}
}

func handleUpdateCategory(svc *service.CategoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		id := r.PathValue("id")
		var req categoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, fmt.Errorf("%w: invalid JSON body", domain.ErrInvalidCategory))
			return
		}
		c := domain.Category{
			ID:        id,
			UserID:    userID,
			Name:      req.Name,
			Kind:      domain.Kind(req.Kind),
			Icon:      req.Icon,
			Color:     req.Color,
			SortOrder: req.SortOrder,
		}
		updated, err := svc.Update(r.Context(), c)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(toCategoryResponse(updated))
	}
}

func handleArchiveCategory(svc *service.CategoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		id := r.PathValue("id")
		if err := svc.Archive(r.Context(), userID, id); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
