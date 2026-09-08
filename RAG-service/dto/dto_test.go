package dto

import (
	"strings"
	"testing"

	"github.com/atgsgrouptest/genet-microservice/RAG-service/apperrors"
	"github.com/stretchr/testify/assert"
)

func TestQueryRequest_Validate(t *testing.T) {
	t.Run("empty query returns validation error", func(t *testing.T) {
		req := QueryRequest{Query: ""}
		err := req.Validate()
		assert.Error(t, err)
		assert.ErrorIs(t, err, apperrors.ErrValidation)
		assert.Equal(t, "Query cannot be empty", apperrors.PublicMessage(err))
	})

	t.Run("whitespace query returns validation error", func(t *testing.T) {
		req := QueryRequest{Query: "   \t\n  "}
		err := req.Validate()
		assert.Error(t, err)
		assert.ErrorIs(t, err, apperrors.ErrValidation)
	})

	t.Run("excessively long query returns validation error", func(t *testing.T) {
		req := QueryRequest{Query: strings.Repeat("a", 4001)}
		err := req.Validate()
		assert.Error(t, err)
		assert.ErrorIs(t, err, apperrors.ErrValidation)
		assert.Contains(t, apperrors.PublicMessage(err), "exceeds maximum length")
	})

	t.Run("valid query with tenant_id trims correctly", func(t *testing.T) {
		req := QueryRequest{
			Query:          "  What is quantum computing?  ",
			TenantID:       "  tenant_123  ",
			CollectionName: "  physics  ",
		}
		err := req.Validate()
		assert.NoError(t, err)
		assert.Equal(t, "What is quantum computing?", req.Query)
		assert.Equal(t, "tenant_123", req.TenantID)
		assert.Equal(t, "physics", req.CollectionName)
	})
}
