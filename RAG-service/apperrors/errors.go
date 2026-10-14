package apperrors

import (
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrValidation   = errors.New("validation error")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrBadRequest   = errors.New("bad request")
	ErrDatabase     = errors.New("database error")
	ErrExternal     = errors.New("external service error")
	ErrUnexpected   = errors.New("unexpected error")
	ErrInternal     = ErrUnexpected
)

// AppError represents a structured application error with domain classification.
type AppError struct {
	Kind    error
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func (e *AppError) Is(target error) bool {
	return errors.Is(e.Kind, target)
}

// New creates a new AppError with a domain kind and user-facing message.
func New(kind error, message string) error {
	return &AppError{Kind: kind, Message: message}
}

// Wrap creates an AppError wrapping an underlying error.
func Wrap(kind error, message string, err error) error {
	return &AppError{Kind: kind, Message: message, Err: err}
}

// PublicMessage returns a safe, user-facing error message without leaking internal implementation details.
func PublicMessage(err error) string {
	if err == nil {
		return ""
	}
	var appErr *AppError
	if errors.As(err, &appErr) && appErr.Message != "" {
		return appErr.Message
	}
	return "Internal server error"
}

// HTTPStatusCode maps an error to the appropriate HTTP status code.
func HTTPStatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}

	switch {
	case errors.Is(err, ErrValidation):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ErrBadRequest):
		return http.StatusBadRequest
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrConflict):
		return http.StatusConflict
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrExternal):
		return http.StatusBadGateway
	case errors.Is(err, ErrDatabase):
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// ToGRPCStatus maps a domain error to a gRPC status error.
func ToGRPCStatus(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, ErrValidation), errors.Is(err, ErrBadRequest):
		return status.Errorf(codes.InvalidArgument, "%s", PublicMessage(err))
	case errors.Is(err, ErrNotFound):
		return status.Errorf(codes.NotFound, "%s", PublicMessage(err))
	case errors.Is(err, ErrConflict):
		return status.Errorf(codes.AlreadyExists, "%s", PublicMessage(err))
	case errors.Is(err, ErrUnauthorized):
		return status.Errorf(codes.Unauthenticated, "%s", PublicMessage(err))
	case errors.Is(err, ErrForbidden):
		return status.Errorf(codes.PermissionDenied, "%s", PublicMessage(err))
	case errors.Is(err, ErrExternal):
		return status.Errorf(codes.Unavailable, "%s", PublicMessage(err))
	default:
		return status.Errorf(codes.Internal, "%s", PublicMessage(err))
	}
}
