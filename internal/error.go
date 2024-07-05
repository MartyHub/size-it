package internal

import (
	"errors"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized")
	ErrNotFound     = errors.New("not found")
)
