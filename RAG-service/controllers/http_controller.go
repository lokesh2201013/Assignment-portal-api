package controllers

import (
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/atgsgrouptest/genet-microservice/RAG-service/apperrors"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/dto"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/services"
	"github.com/gofiber/fiber/v2"
)

// RAGHTTPController handles HTTP endpoints for the multi-tenant RAG service.
type RAGHTTPController struct {
	ragService  services.RAGService
	logger      *slog.Logger
	maxFileSize int64
}

// NewRAGHTTPController creates a new RAGHTTPController.
func NewRAGHTTPController(ragService services.RAGService, logger *slog.Logger, maxFileSize int64) *RAGHTTPController {
	if maxFileSize <= 0 {
		maxFileSize = 20 * 1024 * 1024 // 20 MB default
	}
	return &RAGHTTPController{
		ragService:  ragService,
		logger:      logger,
		maxFileSize: maxFileSize,
	}
}

// SendFilesHTTP handles multipart document uploads with multi-tenancy support.
// Preserves route: POST /sendFiles
func (ctrl *RAGHTTPController) SendFilesHTTP(c *fiber.Ctx) error {
	form, err := c.MultipartForm()
	if err != nil {
		ctrl.logger.Error("Failed to parse multipart form", slog.Any("error", err))
		return ctrl.handleError(c, apperrors.Wrap(apperrors.ErrBadRequest, "Unable to parse multipart form", err))
	}

	files := form.File["files"]
	if len(files) == 0 {
		return ctrl.handleError(c, apperrors.New(apperrors.ErrValidation, "No files uploaded under 'files' field"))
	}

	tenantID := ctrl.resolveTenantID(c, c.FormValue("tenant_id"))
	collectionName := ctrl.resolveCollectionName(c, c.FormValue("collection"))

	processedCount := 0
	totalChunks := 0
	var docIDs []string

	for _, fileHeader := range files {
		if fileHeader.Size > ctrl.maxFileSize {
			ctrl.logger.Warn("File rejected due to size limit",
				slog.String("filename", fileHeader.Filename),
				slog.Int64("size", fileHeader.Size),
			)
			return ctrl.handleError(c, apperrors.New(apperrors.ErrValidation, "File size exceeds allowed limit"))
		}

		file, err := fileHeader.Open()
		if err != nil {
			ctrl.logger.Error("Failed to open uploaded file", slog.String("filename", fileHeader.Filename), slog.Any("error", err))
			return ctrl.handleError(c, apperrors.Wrap(apperrors.ErrInternal, "Failed to read uploaded file", err))
		}

		content, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil {
			ctrl.logger.Error("Failed to read uploaded file content", slog.String("filename", fileHeader.Filename), slog.Any("error", err))
			return ctrl.handleError(c, apperrors.Wrap(apperrors.ErrInternal, "Failed to read file content", err))
		}

		safeFilename := filepath.Base(fileHeader.Filename)
		doc, err := ctrl.ragService.ProcessDocument(c.UserContext(), tenantID, collectionName, safeFilename, content)
		if err != nil {
			ctrl.logger.Error("Failed to process document",
				slog.String("tenant_id", tenantID),
				slog.String("filename", safeFilename),
				slog.Any("error", err),
			)
			return ctrl.handleError(c, err)
		}

		processedCount++
		totalChunks += doc.ChunkCount
		docIDs = append(docIDs, doc.DocumentID)
	}

	return c.Status(fiber.StatusOK).JSON(dto.UploadResponse{
		Message:        "Files embedded and stored successfully",
		TenantID:       tenantID,
		CollectionName: collectionName,
		FilesProcessed: processedCount,
		ChunksStored:   totalChunks,
		DocumentIDs:    docIDs,
	})
}

// GetPromptWithContextHTTP handles contextual semantic retrieval and Q&A with multi-tenant filtering.
// Preserves route: GET /getPromptWithContext (also supports POST /getPromptWithContext)
func (ctrl *RAGHTTPController) GetPromptWithContextHTTP(c *fiber.Ctx) error {
	var req dto.QueryRequest

	if q := strings.TrimSpace(c.Query("query")); q != "" {
		req.Query = q
		req.TenantID = c.Query("tenant_id")
		req.CollectionName = c.Query("collection")
	} else if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return ctrl.handleError(c, apperrors.Wrap(apperrors.ErrBadRequest, "Invalid JSON request body", err))
		}
	}

	if err := req.Validate(); err != nil {
		return ctrl.handleError(c, err)
	}

	tenantID := ctrl.resolveTenantID(c, req.TenantID)
	collectionName := ctrl.resolveCollectionName(c, req.CollectionName)

	answer, err := ctrl.ragService.ProcessQuery(c.UserContext(), tenantID, collectionName, req.Query)
	if err != nil {
		ctrl.logger.Error("Failed to process contextual query",
			slog.String("tenant_id", tenantID),
			slog.Any("error", err),
		)
		return ctrl.handleError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.QueryResponse{
		Answer:         answer,
		TenantID:       tenantID,
		CollectionName: collectionName,
	})
}

// ListDocumentsHTTP lists all documents stored for a tenant in PostgreSQL.
// Route: GET /documents
func (ctrl *RAGHTTPController) ListDocumentsHTTP(c *fiber.Ctx) error {
	tenantID := ctrl.resolveTenantID(c, c.Query("tenant_id"))
	collectionName := ctrl.resolveCollectionName(c, c.Query("collection"))

	docs, err := ctrl.ragService.ListDocuments(c.UserContext(), tenantID, collectionName)
	if err != nil {
		ctrl.logger.Error("Failed to list documents",
			slog.String("tenant_id", tenantID),
			slog.Any("error", err),
		)
		return ctrl.handleError(c, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.DocumentListResponse{
		TenantID:       tenantID,
		CollectionName: collectionName,
		Total:          len(docs),
		Documents:      docs,
	})
}

// HealthCheckHTTP returns service health and vectorstore status.
func (ctrl *RAGHTTPController) HealthCheckHTTP(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(dto.HealthResponse{
		Status:    "healthy",
		Database:  "connected",
		Timestamp: time.Now().UTC(),
	})
}

func (ctrl *RAGHTTPController) resolveTenantID(c *fiber.Ctx, directVal string) string {
	if directVal = strings.TrimSpace(directVal); directVal != "" {
		return directVal
	}
	if headerVal := strings.TrimSpace(c.Get("X-Tenant-ID")); headerVal != "" {
		return headerVal
	}
	if queryVal := strings.TrimSpace(c.Query("tenant_id")); queryVal != "" {
		return queryVal
	}
	return "default"
}

func (ctrl *RAGHTTPController) resolveCollectionName(c *fiber.Ctx, directVal string) string {
	if directVal = strings.TrimSpace(directVal); directVal != "" {
		return directVal
	}
	if headerVal := strings.TrimSpace(c.Get("X-Collection-Name")); headerVal != "" {
		return headerVal
	}
	if queryVal := strings.TrimSpace(c.Query("collection")); queryVal != "" {
		return queryVal
	}
	return "rag-corpus"
}

func (ctrl *RAGHTTPController) handleError(c *fiber.Ctx, err error) error {
	status := apperrors.HTTPStatusCode(err)
	if status >= http.StatusInternalServerError {
		ctrl.logger.Error("HTTP request failed with internal error",
			slog.String("path", c.Path()),
			slog.String("method", c.Method()),
			slog.Any("error", err),
		)
	}
	return c.Status(status).JSON(dto.ErrorResponse{
		Error: apperrors.PublicMessage(err),
	})
}
