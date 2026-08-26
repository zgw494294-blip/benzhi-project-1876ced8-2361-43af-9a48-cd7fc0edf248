package domain

import "fmt"

type ErrorCode string

const (
	ErrValidation ErrorCode = "VALIDATION_ERROR"
	ErrConflict   ErrorCode = "VERSION_CONFLICT"
	ErrState      ErrorCode = "INVALID_STATE"
	ErrNotFound   ErrorCode = "NOT_FOUND"
	ErrForbidden  ErrorCode = "FORBIDDEN"
)

type DomainError struct {
	Code  ErrorCode
	Field string
	Msg   string
}

func (e *DomainError) Error() string {
	if e.Field == "" {
		return e.Msg
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Msg)
}
func NewError(code ErrorCode, field, msg string) error {
	return &DomainError{Code: code, Field: field, Msg: msg}
}
