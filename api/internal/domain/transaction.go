package domain

import "time"

type Transaction struct {
	ID              string
	UserID          string
	Kind            Kind
	AmountPaisa     Money
	CategoryID      *string
	Description     *string
	OccurredAt      time.Time
	RecurringBillID *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}