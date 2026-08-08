package domain

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrInvalidCategory    = errors.New("invalid category")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrInvalidAmount      = errors.New("invalid amount")
	ErrInvalidTransaction = errors.New("invalid transaction")
)
