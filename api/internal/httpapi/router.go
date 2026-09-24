package httpapi

import (
	"net/http"

	"github.com/Abuhurrara/rakam/api/internal/service"
)

func NewRouter(categorySvc *service.CategoryService, transactionSvc *service.TransactionService, authSvc *service.AuthService, personSvc *service.PersonService, debtSvc *service.DebtService, budgetSvc *service.BudgetService, billSvc *service.RecurringBillService, summarySvc *service.SummaryService, exportSvc *service.ExportService, p pinger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", handleHealth(p))

	mux.HandleFunc("POST /api/auth/login", handleLogin(authSvc))
	mux.HandleFunc("POST /api/auth/logout", handleLogout())
	mux.Handle("GET /api/auth/me", requireAuth(authSvc, handleMe(authSvc)))
	mux.Handle("PUT /api/auth/password", requireAuth(authSvc, handleChangePassword(authSvc)))

	mux.Handle("GET /api/categories", requireAuth(authSvc, handleListCategories(categorySvc)))
	mux.Handle("POST /api/categories", requireAuth(authSvc, handleCreateCategory(categorySvc)))
	mux.Handle("PATCH /api/categories/{id}", requireAuth(authSvc, handleUpdateCategory(categorySvc)))
	mux.Handle("DELETE /api/categories/{id}", requireAuth(authSvc, handleArchiveCategory(categorySvc)))

	mux.Handle("GET /api/transactions", requireAuth(authSvc, handleListTransactions(transactionSvc)))
	mux.Handle("POST /api/transactions", requireAuth(authSvc, handleCreateTransaction(transactionSvc)))
	mux.Handle("PATCH /api/transactions/{id}", requireAuth(authSvc, handleUpdateTransaction(transactionSvc)))
	mux.Handle("DELETE /api/transactions/{id}", requireAuth(authSvc, handleDeleteTransaction(transactionSvc)))

	mux.Handle("GET /api/people", requireAuth(authSvc, handleListPeople(personSvc)))
	mux.Handle("POST /api/people", requireAuth(authSvc, handleCreatePerson(personSvc)))
	mux.Handle("DELETE /api/people/{id}", requireAuth(authSvc, handleDeletePerson(personSvc)))
	mux.Handle("GET /api/people/{id}/entries", requireAuth(authSvc, handleListDebtEntries(debtSvc)))
	mux.Handle("POST /api/people/{id}/entries", requireAuth(authSvc, handleCreateDebtEntry(debtSvc)))
	mux.Handle("POST /api/people/{id}/settle-all", requireAuth(authSvc, handleSettleAllDebtEntries(debtSvc)))
	mux.Handle("POST /api/debt-entries/{id}/settle", requireAuth(authSvc, handleSettleDebtEntry(debtSvc)))
	mux.Handle("DELETE /api/debt-entries/{id}", requireAuth(authSvc, handleDeleteDebtEntry(debtSvc)))

	mux.Handle("GET /api/budgets", requireAuth(authSvc, handleListBudgets(budgetSvc)))
	mux.Handle("PUT /api/budgets", requireAuth(authSvc, handleUpsertBudget(budgetSvc)))
	mux.Handle("DELETE /api/budgets/{id}", requireAuth(authSvc, handleDeleteBudget(budgetSvc)))

	mux.Handle("GET /api/bills", requireAuth(authSvc, handleListBills(billSvc)))
	mux.Handle("POST /api/bills", requireAuth(authSvc, handleCreateBill(billSvc)))
	mux.Handle("PATCH /api/bills/{id}", requireAuth(authSvc, handleUpdateBill(billSvc)))
	mux.Handle("DELETE /api/bills/{id}", requireAuth(authSvc, handleDeleteBill(billSvc)))

	mux.Handle("GET /api/export", requireAuth(authSvc, handleExport(exportSvc)))

	mux.Handle("GET /api/summary", requireAuth(authSvc, handleSummary(summarySvc)))

	return recovery(logging(mux))
}
