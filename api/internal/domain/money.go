package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Money is an amount in paisa (1/100 of a rupee). Never a float.
type Money int64

var moneyPattern = regexp.MustCompile(`^\d+(\.\d{1,2})?$`)

// ParseMoney is the one place a decimal rupee string (as typed by a user,
// e.g. "12852" or "12852.50") becomes paisa. It never does arithmetic on
// a formatted string — commas and currency symbols are rejected, not
// stripped.
func ParseMoney(s string) (Money, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("%w: amount is required", ErrInvalidAmount)
	}
	if strings.HasPrefix(s, "-") {
		return 0, fmt.Errorf("%w: amount must be positive", ErrInvalidAmount)
	}
	if !moneyPattern.MatchString(s) {
		return 0, fmt.Errorf("%w: amount is not a valid number", ErrInvalidAmount)
	}

	whole, frac, _ := strings.Cut(s, ".")
	for len(frac) < 2 {
		frac += "0"
	}

	wholeVal, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: amount is too large", ErrInvalidAmount)
	}
	fracVal, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: amount is not a valid number", ErrInvalidAmount)
	}

	const maxWhole = (1<<63 - 1 - 99) / 100
	if wholeVal > maxWhole {
		return 0, fmt.Errorf("%w: amount is too large", ErrInvalidAmount)
	}

	paisa := wholeVal*100 + fracVal
	if paisa == 0 {
		return 0, fmt.Errorf("%w: amount must be greater than zero", ErrInvalidAmount)
	}
	return Money(paisa), nil
}

// Rupees is for display only. Never do arithmetic on its result.
func (m Money) Rupees() float64 {
	return float64(m) / 100
}

// String renders m rounded to the nearest rupee with comma thousands
// grouping, e.g. "Rs 12,852".
func (m Money) String() string {
	rupees := (int64(m) + 50) / 100
	if rupees < 0 {
		rupees = -rupees
	}
	digits := strconv.FormatInt(rupees, 10)

	var grouped strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(d)
	}

	sign := ""
	if m < 0 {
		sign = "-"
	}
	return "Rs " + sign + grouped.String()
}
