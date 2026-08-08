package domain

import "errors"

var (
	ErrNotFound        = errors.New("not found")
	ErrInvalidCategory = errors.New("invalid category")
)
