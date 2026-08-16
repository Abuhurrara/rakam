package domain

import "errors"

var (
	ErrNotFound             = errors.New("not found")
	ErrInvalidCategory      = errors.New("invalid category")
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrInvalidAmount        = errors.New("invalid amount")
	ErrInvalidTransaction   = errors.New("invalid transaction")
	ErrInvalidPerson        = errors.New("invalid person")
	ErrInvalidDebtEntry     = errors.New("invalid debt entry")
	ErrAlreadySettled       = errors.New("debt entry already settled")
	ErrInvalidBudget        = errors.New("invalid budget")
	ErrInvalidRecurringBill = errors.New("invalid recurring bill")
	ErrInvalidMonth         = errors.New("invalid month")
	// ErrPersonHasDebtEntries blocks deleting a person with any debt entry,
	// settled or not — debt_entries.person_id has no ON DELETE cascade, and
	// settled entries must stay viewable (SPEC.md), so a person can only be
	// deleted once they have zero debt history at all.
	ErrPersonHasDebtEntries = errors.New("person has debt entries")
)
