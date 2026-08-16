package service

import (
	"fmt"
	"time"

	"github.com/Abuhurrara/rakam/api/internal/domain"
)

// karachiMonthRange parses a "2006-01" month string and returns the
// half-open [first, next) instant range for that month, anchored to loc.
// Anchoring to loc (rather than the UTC instant time.Parse produces) is
// what makes a purchase at 2am Karachi on the 1st land in this month
// instead of the previous one when compared against a timestamptz column.
func karachiMonthRange(monthStr string, loc *time.Location) (first, next time.Time, err error) {
	parsed, err := time.Parse("2006-01", monthStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: month must be YYYY-MM", domain.ErrInvalidMonth)
	}
	first = time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, loc)
	next = first.AddDate(0, 1, 0)
	return first, next, nil
}
