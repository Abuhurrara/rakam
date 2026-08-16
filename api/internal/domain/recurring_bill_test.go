package domain

import (
	"testing"
	"time"
)

func TestClampDayOfMonth(t *testing.T) {
	tests := []struct {
		name  string
		day   int
		year  int
		month time.Month
		want  int
	}{
		{"day 31 in leap February clamps to 29", 31, 2024, time.February, 29},
		{"day 31 in non-leap February clamps to 28", 31, 2023, time.February, 28},
		{"day 31 in April clamps to 30", 31, 2025, time.April, 30},
		{"day 31 in December is unchanged", 31, 2025, time.December, 31},
		{"mid-month day never needs clamping", 15, 2025, time.February, 15},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClampDayOfMonth(tt.day, tt.year, tt.month); got != tt.want {
				t.Errorf("ClampDayOfMonth(%d, %d, %s) = %d; want %d", tt.day, tt.year, tt.month, got, tt.want)
			}
		})
	}
}
