package apperror

import (
	"errors"
	"fmt"
)

// Canonical error codes shared across domain, application, and HTTP transport layers.
const (
	CodeValidationFailed   = "validation_failed"
	CodeUnauthenticated    = "unauthenticated"
	CodeForbidden          = "forbidden"
	CodeNotFound           = "not_found"
	CodeMethodNotAllowed   = "method_not_allowed"
	CodeConflict           = "conflict"
	CodeVersionConflict    = "version_conflict"
	CodePayloadTooLarge    = "payload_too_large"
	CodeRateLimited        = "rate_limited"
	CodeServiceUnavailable = "service_unavailable"
	CodeInternalError      = "internal_error"
)

// FieldError describes a validation failure on a specific request field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error represents a domain or application error with a stable machine-readable code.
// It has zero HTTP or database dependencies so domain packages can safely use it.
type Error struct {
	Code    string
	Message string
	Fields  []FieldError
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// WithCause attaches an underlying cause error (kept server-side, never serialized to HTTP clients).
func (e *Error) WithCause(err error) *Error {
	cp := *e
	cp.Cause = err
	return &cp
}

// As extracts an *Error from an error chain.
func As(err error) (*Error, bool) {
	var target *Error
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}

func Validation(message string, fields ...FieldError) *Error {
	if message == "" {
		message = "request validation failed"
	}
	return &Error{Code: CodeValidationFailed, Message: message, Fields: fields}
}

func Unauthenticated(message string) *Error {
	if message == "" {
		message = "authentication required"
	}
	return &Error{Code: CodeUnauthenticated, Message: message}
}

func Forbidden(message string) *Error {
	if message == "" {
		message = "permission denied"
	}
	return &Error{Code: CodeForbidden, Message: message}
}

func NotFound(message string) *Error {
	if message == "" {
		message = "resource not found"
	}
	return &Error{Code: CodeNotFound, Message: message}
}

func MethodNotAllowed(message string) *Error {
	if message == "" {
		message = "method not allowed"
	}
	return &Error{Code: CodeMethodNotAllowed, Message: message}
}

func Conflict(message string) *Error {
	if message == "" {
		message = "resource conflict"
	}
	return &Error{Code: CodeConflict, Message: message}
}

func VersionConflict(message string) *Error {
	if message == "" {
		message = "resource version conflict"
	}
	return &Error{Code: CodeVersionConflict, Message: message}
}

func PayloadTooLarge(message string) *Error {
	if message == "" {
		message = "request body exceeds maximum allowed size"
	}
	return &Error{Code: CodePayloadTooLarge, Message: message}
}

func RateLimited(message string) *Error {
	if message == "" {
		message = "too many requests"
	}
	return &Error{Code: CodeRateLimited, Message: message}
}

func ServiceUnavailable(message string) *Error {
	if message == "" {
		message = "service temporarily unavailable"
	}
	return &Error{Code: CodeServiceUnavailable, Message: message}
}

func Internal(cause error) *Error {
	return &Error{
		Code:    CodeInternalError,
		Message: "internal server error",
		Cause:   cause,
	}
}
