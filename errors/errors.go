package errors

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrAuth         = errors.New("unauthorized")
	ErrAccessDenied = errors.New("access denied")
	ErrConflict     = errors.New("conflict")
)
