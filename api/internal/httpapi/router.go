package httpapi

import (
	"net/http"

	"github.com/Abuhurrara/rakam/api/internal/service"
)

func NewRouter(categorySvc *service.CategoryService, authSvc *service.AuthService, p pinger, jwtSecret []byte) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", handleHealth(p))

	mux.HandleFunc("POST /api/auth/login", handleLogin(authSvc))
	mux.HandleFunc("POST /api/auth/logout", handleLogout())
	mux.Handle("GET /api/auth/me", requireAuth(jwtSecret, handleMe(authSvc)))

	mux.Handle("GET /api/categories", requireAuth(jwtSecret, handleListCategories(categorySvc)))
	mux.Handle("POST /api/categories", requireAuth(jwtSecret, handleCreateCategory(categorySvc)))
	mux.Handle("PATCH /api/categories/{id}", requireAuth(jwtSecret, handleUpdateCategory(categorySvc)))
	mux.Handle("DELETE /api/categories/{id}", requireAuth(jwtSecret, handleArchiveCategory(categorySvc)))

	return recovery(logging(mux))
}
