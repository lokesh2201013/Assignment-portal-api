// Package apperror defines application-level errors with proper context and HTTP mappings.
package apperror

import (
	"errors"
	"fmt"
)

// ErrorType represents the category of an error.
type ErrorType string

const (
	ValidationError    ErrorType = "validation"
	NotFoundError      ErrorType = "not_found"
	ConflictError      ErrorType = "conflict"
	UnauthorizedError  ErrorType = "unauthorized"
	ForbiddenError     ErrorType = "forbidden"
	InternalError      ErrorType = "internal"
	ExternalServiceErr ErrorType = "external_service"
	DatabaseError      ErrorType = "database"
)

// AppError is the domain error type with context and HTTP status.
type AppError struct {
	Type       ErrorType
	Message    string
	StatusCode int
	Err        error // Original error, useful for logging
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (underlying: %v)", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the wrapped error for use with errors.Is/As.
func (e *AppError) Unwrap() error {
	return e.Err
}

// New creates a new AppError with the given type, message, and HTTP status.
func New(errType ErrorType, message string, statusCode int) *AppError {
	return &AppError{
		Type:       errType,
		Message:    message,
		StatusCode: statusCode,
		Err:        nil,
	}
}

// Wrap wraps an existing error with domain context.
func Wrap(errType ErrorType, message string, statusCode int, err error) *AppError {
	return &AppError{
		Type:       errType,
		Message:    message,
		StatusCode: statusCode,
		Err:        err,
	}
}

// Validation returns a validation error with 400 status.
func Validation(message string) *AppError {
	return New(ValidationError, message, 400)
}

// NotFound returns a not-found error with 404 status.
func NotFound(message string) *AppError {
	return New(NotFoundError, message, 404)
}

// Conflict returns a conflict error with 409 status.
func Conflict(message string) *AppError {
	return New(ConflictError, message, 409)
}

// Unauthorized returns an unauthorized error with 401 status.
func Unauthorized(message string) *AppError {
	return New(UnauthorizedError, message, 401)
}

// Forbidden returns a forbidden error with 403 status.
func Forbidden(message string) *AppError {
	return New(ForbiddenError, message, 403)
}

// Database returns a database error with 500 status.
func Database(message string, err error) *AppError {
	return Wrap(DatabaseError, message, 500, err)
}

// Internal returns an internal error with 500 status (safe message, logs original).
func Internal(message string, err error) *AppError {
	return Wrap(InternalError, message, 500, err)
}

// ExternalService returns an external service error with 502 status.
func ExternalService(message string, err error) *AppError {
	return Wrap(ExternalServiceErr, message, 502, err)
}

// IsType checks if an error is of a specific type.
func IsType(err error, errType ErrorType) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type == errType
	}
	return false
}

// StatusCode returns the HTTP status code for an error.
// Returns 500 if error is not an AppError.
func StatusCode(err error) int {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.StatusCode
	}
	return 500
}

// Message returns a safe message for the client.
// Returns the AppError message, or a generic message for non-AppErrors.
func Message(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	return "An unexpected error occurred"
}
