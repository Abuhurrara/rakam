package domain

import "time"

type Budget struct {
	ID         string
	UserID     string
	CategoryID string
	Month      time.Time
	LimitPaisa Money
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// BudgetWithSpent is one row of the Budget screen: a category, its budget
// for the month if one has been set (nil otherwise), and the amount spent
// against it — computed on read from transactions, never stored, so there
// is nothing to invalidate when a transaction changes.
type BudgetWithSpent struct {
	Category   Category
	Budget     *Budget
	SpentPaisa Money
}