package apperrors

import (
	"errors"
	"fmt"
)

var (
	ErrValidation   = errors.New("validation error")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrDatabase     = errors.New("database error")
	ErrExternal     = errors.New("external service error")
	ErrUnexpected   = errors.New("unexpected error")
)

type Error struct {
	Kind    error
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

func (e *Error) Is(target error) bool {
	return errors.Is(e.Kind, target)
}

func New(kind error, message string) error {
	return &Error{Kind: kind, Message: message}
}

func Wrap(kind error, message string, err error) error {
	return &Error{Kind: kind, Message: message, Err: err}
}

func PublicMessage(err error) string {
	var appErr *Error
	if errors.As(err, &appErr) && appErr.Message != "" {
		return appErr.Message
	}
	return "Internal server error"
}
