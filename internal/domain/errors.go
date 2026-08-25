package domain

import "fmt"

type ErrorCode string

const (
	CodeValidation ErrorCode = "VALIDATION_ERROR"
	CodeConflict   ErrorCode = "VERSION_CONFLICT"
	CodeState      ErrorCode = "INVALID_STATE"
	CodeNotFound   ErrorCode = "NOT_FOUND"
	CodeForbidden  ErrorCode = "FORBIDDEN"
	CodeImmutable  ErrorCode = "IMMUTABLE"
	CodeDuplicate  ErrorCode = "DUPLICATE"
	CodeIntegrity  ErrorCode = "INTEGRITY_FAILURE"
)

type BusinessError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func (e *BusinessError) Error() string { return e.Message }

func NewError(code ErrorCode, format string, args ...any) error {
	return &BusinessError{Code: code, Message: fmt.Sprintf(format, args...)}
}

func ErrorCodeOf(err error) ErrorCode {
	if business, ok := err.(*BusinessError); ok {
		return business.Code
	}
	return "INTERNAL_ERROR"
}
