package domain

import (
	"errors"
	"testing"
)

func TestParseMoney(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Money
		wantErr bool
	}{
		{name: "whole number", input: "12852", want: 1285200},
		{name: "two decimals", input: "12852.50", want: 1285250},
		{name: "one decimal", input: "12852.5", want: 1285250},
		{name: "zero decimals explicit", input: "1.00", want: 100},
		{name: "three decimals rejected", input: "12852.505", wantErr: true},
		{name: "negative rejected", input: "-100", wantErr: true},
		{name: "zero rejected", input: "0", wantErr: true},
		{name: "zero with decimals rejected", input: "0.00", wantErr: true},
		{name: "empty rejected", input: "", wantErr: true},
		{name: "whitespace only rejected", input: "   ", wantErr: true},
		{name: "comma-formatted rejected", input: "12,852", wantErr: true},
		{name: "currency symbol rejected", input: "Rs 100", wantErr: true},
		{name: "not a number rejected", input: "abc", wantErr: true},
		{name: "leading plus rejected", input: "+100", wantErr: true},
		{name: "trailing dot rejected", input: "100.", wantErr: true},
		{name: "whitespace padded accepted", input: "  100.50  ", want: 10050},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMoney(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseMoney(%q) = %d, nil; want error", tt.input, got)
				}
				if !errors.Is(err, ErrInvalidAmount) {
					t.Fatalf("ParseMoney(%q) error = %v; want wrapping ErrInvalidAmount", tt.input, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseMoney(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ParseMoney(%q) = %d; want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestMoney_String(t *testing.T) {
	tests := []struct {
		name string
		m    Money
		want string
	}{
		{name: "small amount", m: 1285250, want: "Rs 12,853"},
		{name: "rounds down", m: 1285249, want: "Rs 12,852"},
		{name: "under a thousand", m: 50000, want: "Rs 500"},
		{name: "millions", m: 123456789, want: "Rs 1,234,568"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.String(); got != tt.want {
				t.Fatalf("Money(%d).String() = %q; want %q", tt.m, got, tt.want)
			}
		})
	}
}

func TestMoney_Rupees(t *testing.T) {
	if got := Money(1285250).Rupees(); got != 12852.5 {
		t.Fatalf("Rupees() = %v; want 12852.5", got)
	}
}