package domain

import "time"

type RecurringBill struct {
	ID                 string
	UserID             string
	Name               string
	AmountPaisa        Money
	CategoryID         *string
	DayOfMonth         int
	IsActive           bool
	LastGeneratedMonth *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ClampDayOfMonth returns day if year/month has that many days, otherwise
// the last day of that month — a day-31 bill posts on Feb 28 (or 29 in a
// leap year) instead of being skipped. Pure calendar math: only the day
// count of a month matters here, never an instant, so no timezone is
// involved.
func ClampDayOfMonth(day, year int, month time.Month) int {
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > lastDay {
		return lastDay
	}
	return day
}