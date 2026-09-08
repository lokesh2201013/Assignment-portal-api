package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atgsgrouptest/genet-microservice/RAG-service/apperrors"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/dto"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/models"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRAGService struct {
	mock.Mock
}

func (m *MockRAGService) ProcessDocument(ctx context.Context, tenantID, collectionName, filename string, content []byte) (*models.RAGDocument, error) {
	args := m.Called(ctx, tenantID, collectionName, filename, content)
	if doc := args.Get(0); doc != nil {
		return doc.(*models.RAGDocument), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRAGService) ProcessQuery(ctx context.Context, tenantID, collectionName, query string) (string, error) {
	args := m.Called(ctx, tenantID, collectionName, query)
	return args.String(0), args.Error(1)
}

func (m *MockRAGService) ListDocuments(ctx context.Context, tenantID, collectionName string) ([]models.RAGDocument, error) {
	args := m.Called(ctx, tenantID, collectionName)
	return args.Get(0).([]models.RAGDocument), args.Error(1)
}

func setupTestFiberApp(ctrl *RAGHTTPController) *fiber.App {
	app := fiber.New()
	app.Post("/sendFiles", ctrl.SendFilesHTTP)
	app.Get("/getPromptWithContext", ctrl.GetPromptWithContextHTTP)
	app.Post("/getPromptWithContext", ctrl.GetPromptWithContextHTTP)
	app.Get("/documents", ctrl.ListDocumentsHTTP)
	app.Get("/healthz", ctrl.HealthCheckHTTP)
	return app
}

func TestRAGHTTPController_HealthCheck(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockSvc := new(MockRAGService)
	ctrl := NewRAGHTTPController(mockSvc, logger, 1024*1024)
	app := setupTestFiberApp(ctrl)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var health dto.HealthResponse
	err = json.NewDecoder(resp.Body).Decode(&health)
	assert.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
}

func TestRAGHTTPController_GetPromptWithContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("successful query via query param with tenant header", func(t *testing.T) {
		mockSvc := new(MockRAGService)
		ctrl := NewRAGHTTPController(mockSvc, logger, 1024*1024)
		app := setupTestFiberApp(ctrl)

		mockSvc.On("ProcessQuery", mock.Anything, "tenant_abc", "rag-corpus", "hello world").Return("AI generated answer", nil)

		req := httptest.NewRequest(http.MethodGet, "/getPromptWithContext?query=hello%20world", nil)
		req.Header.Set("X-Tenant-ID", "tenant_abc")
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res dto.QueryResponse
		err = json.NewDecoder(resp.Body).Decode(&res)
		assert.NoError(t, err)
		assert.Equal(t, "AI generated answer", res.Answer)
		assert.Equal(t, "tenant_abc", res.TenantID)
		mockSvc.AssertExpectations(t)
	})

	t.Run("empty query returns 422 validation error", func(t *testing.T) {
		mockSvc := new(MockRAGService)
		ctrl := NewRAGHTTPController(mockSvc, logger, 1024*1024)
		app := setupTestFiberApp(ctrl)

		req := httptest.NewRequest(http.MethodGet, "/getPromptWithContext?query=", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

		var errRes dto.ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&errRes)
		assert.NoError(t, err)
		assert.Equal(t, "Query cannot be empty", errRes.Error)
	})

	t.Run("successful query via JSON body", func(t *testing.T) {
		mockSvc := new(MockRAGService)
		ctrl := NewRAGHTTPController(mockSvc, logger, 1024*1024)
		app := setupTestFiberApp(ctrl)

		mockSvc.On("ProcessQuery", mock.Anything, "default", "rag-corpus", "explain kubernetes").Return("Kubernetes is an orchestrator.", nil)

		bodyBytes, _ := json.Marshal(dto.QueryRequest{Query: "explain kubernetes"})
		req := httptest.NewRequest(http.MethodPost, "/getPromptWithContext", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res dto.QueryResponse
		err = json.NewDecoder(resp.Body).Decode(&res)
		assert.NoError(t, err)
		assert.Equal(t, "Kubernetes is an orchestrator.", res.Answer)
	})
}

func TestRAGHTTPController_SendFiles(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("missing files field returns error", func(t *testing.T) {
		mockSvc := new(MockRAGService)
		ctrl := NewRAGHTTPController(mockSvc, logger, 1024*1024)
		app := setupTestFiberApp(ctrl)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/sendFiles", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	})

	t.Run("successful file upload with tenant context", func(t *testing.T) {
		mockSvc := new(MockRAGService)
		ctrl := NewRAGHTTPController(mockSvc, logger, 1024*1024)
		app := setupTestFiberApp(ctrl)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("files", "guide.txt")
		assert.NoError(t, err)
		_, _ = part.Write([]byte("Important content for RAG"))
		_ = writer.WriteField("tenant_id", "tenant_xyz")
		_ = writer.Close()

		expectedDoc := &models.RAGDocument{
			DocumentID: "doc-123",
			ChunkCount: 2,
		}
		mockSvc.On("ProcessDocument", mock.Anything, "tenant_xyz", "rag-corpus", "guide.txt", []byte("Important content for RAG")).Return(expectedDoc, nil)

		req := httptest.NewRequest(http.MethodPost, "/sendFiles", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var res dto.UploadResponse
		err = json.NewDecoder(resp.Body).Decode(&res)
		assert.NoError(t, err)
		assert.Equal(t, "Files embedded and stored successfully", res.Message)
		assert.Equal(t, "tenant_xyz", res.TenantID)
		assert.Equal(t, 1, res.FilesProcessed)
		assert.Equal(t, 2, res.ChunksStored)
		assert.Contains(t, res.DocumentIDs, "doc-123")
		mockSvc.AssertExpectations(t)
	})
}

func TestRAGHTTPController_ListDocuments(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockSvc := new(MockRAGService)
	ctrl := NewRAGHTTPController(mockSvc, logger, 1024*1024)
	app := setupTestFiberApp(ctrl)

	mockDocs := []models.RAGDocument{
		{DocumentID: "doc-1", Filename: "lec1.txt", ChunkCount: 3},
	}
	mockSvc.On("ListDocuments", mock.Anything, "tenant_1", "rag-corpus").Return(mockDocs, nil)

	req := httptest.NewRequest(http.MethodGet, "/documents?tenant_id=tenant_1", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var res dto.DocumentListResponse
	err = json.NewDecoder(resp.Body).Decode(&res)
	assert.NoError(t, err)
	assert.Equal(t, 1, res.Total)
	assert.Equal(t, "tenant_1", res.TenantID)
	assert.Equal(t, "lec1.txt", res.Documents[0].Filename)
}
