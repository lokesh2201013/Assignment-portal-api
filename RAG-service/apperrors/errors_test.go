package apperrors

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAppError_WrappingAndIs(t *testing.T) {
	underlying := errors.New("underlying db error")
	appErr := Wrap(ErrDatabase, "failed to fetch data", underlying)

	assert.True(t, errors.Is(appErr, ErrDatabase))
	assert.False(t, errors.Is(appErr, ErrNotFound))
	assert.Equal(t, underlying, errors.Unwrap(appErr))
	assert.Contains(t, appErr.Error(), "failed to fetch data: underlying db error")
}

func TestPublicMessage(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "app error with message",
			err:      New(ErrValidation, "Filename cannot be empty"),
			expected: "Filename cannot be empty",
		},
		{
			name:     "wrapped standard error",
			err:      fmt.Errorf("sql: connection refused"),
			expected: "Internal server error",
		},
		{
			name:     "wrapped app error with underlying error",
			err:      Wrap(ErrExternal, "Failed to connect to AI service", errors.New("timeout")),
			expected: "Failed to connect to AI service",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, PublicMessage(tc.err))
		})
	}
}

func TestHTTPStatusCode(t *testing.T) {
	assert.Equal(t, http.StatusOK, HTTPStatusCode(nil))
	assert.Equal(t, http.StatusUnprocessableEntity, HTTPStatusCode(New(ErrValidation, "invalid")))
	assert.Equal(t, http.StatusBadRequest, HTTPStatusCode(New(ErrBadRequest, "bad request")))
	assert.Equal(t, http.StatusNotFound, HTTPStatusCode(New(ErrNotFound, "not found")))
	assert.Equal(t, http.StatusConflict, HTTPStatusCode(New(ErrConflict, "already exists")))
	assert.Equal(t, http.StatusUnauthorized, HTTPStatusCode(New(ErrUnauthorized, "unauthorized")))
	assert.Equal(t, http.StatusForbidden, HTTPStatusCode(New(ErrForbidden, "forbidden")))
	assert.Equal(t, http.StatusBadGateway, HTTPStatusCode(New(ErrExternal, "upstream error")))
	assert.Equal(t, http.StatusInternalServerError, HTTPStatusCode(New(ErrDatabase, "query failed")))
	assert.Equal(t, http.StatusInternalServerError, HTTPStatusCode(errors.New("random error")))
}

func TestToGRPCStatus(t *testing.T) {
	assert.Nil(t, ToGRPCStatus(nil))

	errVal := ToGRPCStatus(New(ErrValidation, "invalid field"))
	assert.Equal(t, codes.InvalidArgument, status.Code(errVal))

	errNF := ToGRPCStatus(New(ErrNotFound, "missing"))
	assert.Equal(t, codes.NotFound, status.Code(errNF))

	errExt := ToGRPCStatus(New(ErrExternal, "service down"))
	assert.Equal(t, codes.Unavailable, status.Code(errExt))

	errUnknown := ToGRPCStatus(errors.New("internal bug"))
	assert.Equal(t, codes.Internal, status.Code(errUnknown))
}
