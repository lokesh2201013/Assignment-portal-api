package dto

import (
	"strings"
	"time"

	"github.com/atgsgrouptest/genet-microservice/RAG-service/apperrors"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/models"
)

// QueryRequest represents the HTTP body or query parameter for semantic search and Q&A.
type QueryRequest struct {
	Query          string `json:"query"`
	TenantID       string `json:"tenant_id,omitempty"`
	CollectionName string `json:"collection_name,omitempty"`
}

// Validate checks that the query is not empty and within reasonable bounds.
func (r *QueryRequest) Validate() error {
	r.Query = strings.TrimSpace(r.Query)
	if r.Query == "" {
		return apperrors.New(apperrors.ErrValidation, "Query cannot be empty")
	}
	if len(r.Query) > 4000 {
		return apperrors.New(apperrors.ErrValidation, "Query exceeds maximum length of 4000 characters")
	}

	r.TenantID = strings.TrimSpace(r.TenantID)
	r.CollectionName = strings.TrimSpace(r.CollectionName)
	return nil
}

// QueryResponse contains the LLM's context-augmented response and tenant context.
type QueryResponse struct {
	Answer         string `json:"answer"`
	TenantID       string `json:"tenant_id,omitempty"`
	CollectionName string `json:"collection_name,omitempty"`
}

// UploadResponse indicates the result of uploading and embedding documents.
type UploadResponse struct {
	Message        string   `json:"message"`
	TenantID       string   `json:"tenant_id,omitempty"`
	CollectionName string   `json:"collection_name,omitempty"`
	FilesProcessed int      `json:"files_processed,omitempty"`
	ChunksStored   int      `json:"chunks_stored,omitempty"`
	DocumentIDs    []string `json:"document_ids,omitempty"`
}

// DocumentListResponse returns a list of stored documents for a tenant.
type DocumentListResponse struct {
	TenantID       string               `json:"tenant_id"`
	CollectionName string               `json:"collection_name"`
	Total          int                  `json:"total"`
	Documents      []models.RAGDocument `json:"documents"`
}

// ErrorResponse represents a standardized API error response.
type ErrorResponse struct {
	Error string `json:"error"`
}

// HealthResponse represents service health and readiness status.
type HealthResponse struct {
	Status    string    `json:"status"`
	Database  string    `json:"database,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
