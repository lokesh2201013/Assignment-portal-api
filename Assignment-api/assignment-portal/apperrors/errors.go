package apperrors

import (
	"errors"
	"fmt"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrValidation   = errors.New("validation")
	ErrNotFound     = errors.New("not_found")
	ErrConflict     = errors.New("conflict")
	ErrDatabase     = errors.New("database")
	ErrExternal     = errors.New("external")
	ErrUnexpected   = errors.New("unexpected")
)

type Error struct {
	code error
	msg  string
	err  error
}

func New(code error, msg string) error {
	return &Error{code: code, msg: msg}
}

func Wrap(code error, msg string, err error) error {
	if err == nil {
		return New(code, msg)
	}
	return &Error{code: code, msg: msg, err: err}
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.err != nil {
		return fmt.Sprintf("%s: %v", e.msg, e.err)
	}
	return e.msg
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *Error) Is(target error) bool {
	if target == nil {
		return false
	}
	if e == nil {
		return false
	}
	return e.code == target || target == e.code
}

func (e *Error) Code() error {
	if e == nil {
		return nil
	}
	return e.code
}

func PublicMessage(err error) string {
	if err == nil {
		return ""
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Error()
	}
	return "An unexpected error occurred"
}
