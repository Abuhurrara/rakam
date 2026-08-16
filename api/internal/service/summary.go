package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
	"github.com/Abuhurrara/rakam/api/internal/port"
)

// UpcomingBill pairs a bill with the next Karachi calendar date it's due —
// this month's clamped day_of_month, or next month's if it's already been
// generated for the current month.
type UpcomingBill struct {
	Bill  domain.RecurringBill
	DueAt time.Time
}

type Summary struct {
	Month              time.Time
	IncomePaisa        domain.Money
	ExpensePaisa       domain.Money
	NetPaisa           domain.Money
	DaysRemaining      int
	BudgetLimitPaisa   domain.Money
	BudgetSpentPaisa   domain.Money
	OwedToMePaisa      domain.Money
	IOwePaisa          domain.Money
	NetOwedPaisa       domain.Money
	UpcomingBills      []UpcomingBill
	RecentTransactions []domain.Transaction
}

const recentTransactionLimit = 5
const upcomingBillWindow = 7 * 24 * time.Hour

type SummaryService struct {
	txRepo    port.TransactionRepo
	budgetSvc *BudgetService
	billSvc   *RecurringBillService
	personSvc *PersonService
	loc       *time.Location
}

func NewSummaryService(txRepo port.TransactionRepo, budgetSvc *BudgetService, billSvc *RecurringBillService, personSvc *PersonService, loc *time.Location) *SummaryService {
	return &SummaryService{txRepo: txRepo, budgetSvc: budgetSvc, billSvc: billSvc, personSvc: personSvc, loc: loc}
}

// Get is the one unusual GET in this API: it has a write side effect,
// generating any recurring-bill transactions that have become due, before
// assembling the read. That's safe to call on every request — GenerateDue's
// claim is a database-level compare-and-swap per bill, so a second call in
// the same month creates nothing (see RecurringBillRepo.GenerateDue).
//
// GenerateDue always runs for the real current Karachi month, regardless of
// the month being viewed via monthStr — a GET for a past month must not
// imply generating that past month's bills; only "now" ever triggers
// generation.
func (s *SummaryService) Get(ctx context.Context, userID, monthStr string) (Summary, error) {
	if _, err := s.billSvc.GenerateDue(ctx, userID); err != nil {
		return Summary{}, err
	}

	if monthStr == "" {
		now := time.Now().In(s.loc)
		monthStr = fmt.Sprintf("%04d-%02d", now.Year(), now.Month())
	}
	first, next, err := karachiMonthRange(monthStr, s.loc)
	if err != nil {
		return Summary{}, err
	}

	income, expense, err := s.txRepo.SumByKind(ctx, userID, first, next)
	if err != nil {
		return Summary{}, fmt.Errorf("summing month transactions: %w", err)
	}

	budgets, err := s.budgetSvc.ListForMonth(ctx, userID, monthStr)
	if err != nil {
		return Summary{}, fmt.Errorf("listing budgets: %w", err)
	}
	var budgetLimit, budgetSpent domain.Money
	for _, b := range budgets {
		if b.Budget != nil {
			budgetLimit += b.Budget.LimitPaisa
		}
		budgetSpent += b.SpentPaisa
	}

	balances, err := s.personSvc.List(ctx, userID)
	if err != nil {
		return Summary{}, fmt.Errorf("listing people: %w", err)
	}
	var owedToMe, iOwe domain.Money
	for _, pb := range balances {
		if pb.BalancePaisa > 0 {
			owedToMe += pb.BalancePaisa
		} else {
			iOwe += -pb.BalancePaisa
		}
	}

	recent, _, err := s.txRepo.List(ctx, userID, port.TransactionFilter{Limit: recentTransactionLimit})
	if err != nil {
		return Summary{}, fmt.Errorf("listing recent transactions: %w", err)
	}

	bills, err := s.billSvc.List(ctx, userID)
	if err != nil {
		return Summary{}, fmt.Errorf("listing recurring bills: %w", err)
	}
	now := time.Now().In(s.loc)
	var upcoming []UpcomingBill
	for _, b := range bills {
		if !b.IsActive {
			continue
		}
		due := nextDueDate(b, now)
		if !due.After(now.Add(upcomingBillWindow)) {
			upcoming = append(upcoming, UpcomingBill{Bill: b, DueAt: due})
		}
	}

	daysRemaining := 0
	if !now.Before(first) && now.Before(next) {
		daysRemaining = int(next.Sub(now).Hours() / 24)
	}

	return Summary{
		Month:              first,
		IncomePaisa:        income,
		ExpensePaisa:       expense,
		NetPaisa:           income - expense,
		DaysRemaining:      daysRemaining,
		BudgetLimitPaisa:   budgetLimit,
		BudgetSpentPaisa:   budgetSpent,
		OwedToMePaisa:      owedToMe,
		IOwePaisa:          iOwe,
		NetOwedPaisa:       owedToMe - iOwe,
		UpcomingBills:      upcoming,
		RecentTransactions: recent,
	}, nil
}

// nextDueDate is this month's clamped day_of_month, or next month's if the
// bill has already generated for the current month (LastGeneratedMonth not
// before it) — so a bill GenerateDue just posted this request doesn't show
// as "due" again until its next real occurrence.
func nextDueDate(b domain.RecurringBill, now time.Time) time.Time {
	loc := now.Location()
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	if b.LastGeneratedMonth != nil && !b.LastGeneratedMonth.Before(month) {
		month = month.AddDate(0, 1, 0)
	}
	day := domain.ClampDayOfMonth(b.DayOfMonth, month.Year(), month.Month())
	return time.Date(month.Year(), month.Month(), day, 0, 0, 0, 0, loc)
}
