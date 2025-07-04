package errors

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
	Code       string `json:"code"`
	Message    string `json:"message"`
	IsFeedBack bool   `json:"is_feedback"`
}

func (e ErrorResponse) Error() string {
	return fmt.Sprintf("code: %s, isFeedback: %v, message: %s", e.Code, e.IsFeedBack, e.Message)
}
