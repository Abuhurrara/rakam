package httpapi

import (
	"net/http"

	"github.com/Abuhurrara/rakam/api/internal/service"
)

func NewRouter(categorySvc *service.CategoryService, transactionSvc *service.TransactionService, authSvc *service.AuthService, p pinger, jwtSecret []byte) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", handleHealth(p))

	mux.HandleFunc("POST /api/auth/login", handleLogin(authSvc))
	mux.HandleFunc("POST /api/auth/logout", handleLogout())
	mux.Handle("GET /api/auth/me", requireAuth(jwtSecret, handleMe(authSvc)))

	mux.Handle("GET /api/categories", requireAuth(jwtSecret, handleListCategories(categorySvc)))
	mux.Handle("POST /api/categories", requireAuth(jwtSecret, handleCreateCategory(categorySvc)))
	mux.Handle("PATCH /api/categories/{id}", requireAuth(jwtSecret, handleUpdateCategory(categorySvc)))
	mux.Handle("DELETE /api/categories/{id}", requireAuth(jwtSecret, handleArchiveCategory(categorySvc)))

	mux.Handle("GET /api/transactions", requireAuth(jwtSecret, handleListTransactions(transactionSvc)))
	mux.Handle("POST /api/transactions", requireAuth(jwtSecret, handleCreateTransaction(transactionSvc)))
	mux.Handle("PATCH /api/transactions/{id}", requireAuth(jwtSecret, handleUpdateTransaction(transactionSvc)))
	mux.Handle("DELETE /api/transactions/{id}", requireAuth(jwtSecret, handleDeleteTransaction(transactionSvc)))

	return recovery(logging(mux))
}
