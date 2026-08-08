package domain

import "time"

type Kind string

const (
	KindExpense Kind = "expense"
	KindIncome  Kind = "income"
)

type Category struct {
	ID         string
	UserID     string
	Name       string
	Kind       Kind
	Icon       string
	Color      string
	SortOrder  int
	IsArchived bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
