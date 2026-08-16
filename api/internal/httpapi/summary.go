package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/Abuhurrara/rakam/api/internal/service"
)

type upcomingBillResponse struct {
	Bill  billResponse `json:"bill"`
	DueAt string       `json:"due_at"`
}

type summaryResponse struct {
	Month              string                 `json:"month"`
	IncomePaisa        int64                  `json:"income_paisa"`
	ExpensePaisa       int64                  `json:"expense_paisa"`
	NetPaisa           int64                  `json:"net_paisa"`
	DaysRemaining      int                    `json:"days_remaining"`
	BudgetLimitPaisa   int64                  `json:"budget_limit_paisa"`
	BudgetSpentPaisa   int64                  `json:"budget_spent_paisa"`
	OwedToMePaisa      int64                  `json:"owed_to_me_paisa"`
	IOwePaisa          int64                  `json:"i_owe_paisa"`
	NetOwedPaisa       int64                  `json:"net_owed_paisa"`
	UpcomingBills      []upcomingBillResponse `json:"upcoming_bills"`
	RecentTransactions []transactionResponse  `json:"recent_transactions"`
}

func toSummaryResponse(s service.Summary) summaryResponse {
	upcoming := make([]upcomingBillResponse, len(s.UpcomingBills))
	for i, u := range s.UpcomingBills {
		upcoming[i] = upcomingBillResponse{
			Bill:  toBillResponse(u.Bill),
			DueAt: u.DueAt.Format("2006-01-02"),
		}
	}
	recent := make([]transactionResponse, len(s.RecentTransactions))
	for i, t := range s.RecentTransactions {
		recent[i] = toTransactionResponse(t)
	}
	return summaryResponse{
		Month:              s.Month.Format("2006-01"),
		IncomePaisa:        int64(s.IncomePaisa),
		ExpensePaisa:       int64(s.ExpensePaisa),
		NetPaisa:           int64(s.NetPaisa),
		DaysRemaining:      s.DaysRemaining,
		BudgetLimitPaisa:   int64(s.BudgetLimitPaisa),
		BudgetSpentPaisa:   int64(s.BudgetSpentPaisa),
		OwedToMePaisa:      int64(s.OwedToMePaisa),
		IOwePaisa:          int64(s.IOwePaisa),
		NetOwedPaisa:       int64(s.NetOwedPaisa),
		UpcomingBills:      upcoming,
		RecentTransactions: recent,
	}
}

func handleSummary(svc *service.SummaryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := UserIDFromContext(r.Context())
		// An empty month is passed through as-is; SummaryService.Get
		// resolves the default to the real current Karachi month via its
		// injected *time.Location — never server-local time, which
		// time.Now() here would have been.
		month := r.URL.Query().Get("month")
		summary, err := svc.Get(r.Context(), userID, month)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(toSummaryResponse(summary))
	}
}
