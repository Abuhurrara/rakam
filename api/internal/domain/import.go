package domain

import "time"

type ImportDevice struct {
	ID, UserID, Name string
	CreatedAt        time.Time
	LastSeenAt       *time.Time
}

type ImportSuggestion struct {
	ID, UserID, Source, SourceID, Description, Status string
	Kind                                              Kind
	AmountPaisa                                       Money
	OccurredAt, CreatedAt, UpdatedAt                  time.Time
	TransactionID                                     *string
}
