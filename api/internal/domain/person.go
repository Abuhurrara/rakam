package domain

import "time"

type Person struct {
	ID        string
	UserID    string
	Name      string
	Phone     *string
	Notes     *string
	CreatedAt time.Time
	UpdatedAt time.Time
}
