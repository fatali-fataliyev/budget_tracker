package customErrors

import (
	"fmt"
)

const (
	ErrNotFound     = "NOT FOUND"
	ErrInvalidInput = "INVALID INPUT"
	ErrAuth         = "UNAUTHORIZED"
	ErrAccessDenied = "ACCESS DENIED"
	ErrConflict     = "CONFLICT"
	ErrInternal     = "INTERNAL"
)

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e ErrorResponse) Error() string {
	return fmt.Sprintf("code: %s, message: %s", e.Code, e.Message)
}
