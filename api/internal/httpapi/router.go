package httpapi

import (
	"net/http"

	"github.com/Abuhurrara/rakam/api/internal/service"
)

func NewRouter(categorySvc *service.CategoryService, p pinger, seedUserID string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", handleHealth(p))

	mux.Handle("GET /api/categories", withSeedUser(seedUserID, handleListCategories(categorySvc)))
	mux.Handle("POST /api/categories", withSeedUser(seedUserID, handleCreateCategory(categorySvc)))
	mux.Handle("PATCH /api/categories/{id}", withSeedUser(seedUserID, handleUpdateCategory(categorySvc)))
	mux.Handle("DELETE /api/categories/{id}", withSeedUser(seedUserID, handleArchiveCategory(categorySvc)))

	return recovery(logging(mux))
}
