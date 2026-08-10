package domain

import "time"

type DebtDirection string

const (
	DirectionIOwe    DebtDirection = "i_owe"
	DirectionTheyOwe DebtDirection = "they_owe"
)

type DebtEntry struct {
	ID          string
	UserID      string
	PersonID    string
	Direction   DebtDirection
	AmountPaisa Money
	Description string
	IncurredAt  time.Time
	SettledAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PersonBalance is a person plus their net balance: the sum of unsettled
// they_owe entries minus the sum of unsettled i_owe entries. Positive means
// they owe me.
type PersonBalance struct {
	Person       Person
	BalancePaisa Money
}

// KindForDirection maps a debt direction to the transaction kind settling it
// produces: paying off money I owed is an expense, being paid back money
// owed to me is income. Shared by DebtService and postgres.DebtRepo so the
// atomic settle path can build a transaction row without round-tripping
// through the service for the mapping.
func KindForDirection(d DebtDirection) Kind {
	if d == DirectionIOwe {
		return KindExpense
	}
	return KindIncome
}
