package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/atgsgrouptest/genet-microservice/RAG-service/apperrors"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/models"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/repositories"
	"github.com/google/uuid"
)

// RAGService defines the business logic contract for multi-tenant document embedding, storage, and retrieval.
type RAGService interface {
	ProcessDocument(ctx context.Context, tenantID, collectionName, filename string, content []byte) (*models.RAGDocument, error)
	ProcessQuery(ctx context.Context, tenantID, collectionName, query string) (string, error)
	ProcessQueryWithSources(ctx context.Context, tenantID, collectionName, query string) (string, []models.SourceReference, error)
	ListDocuments(ctx context.Context, tenantID, collectionName string) ([]models.RAGDocument, error)
	ListCollections(ctx context.Context, tenantID string) ([]models.RAGCollection, error)
	DeleteDocument(ctx context.Context, tenantID, collectionName, documentID string) error
	UpdateDocumentStatus(ctx context.Context, tenantID, documentID, status string) error
}

type ragServiceImpl struct {
	vectorRepo        repositories.VectorRepository
	metaRepo          repositories.MetadataRepository
	aiClient          AIClient
	chunker           *DocumentChunker
	defaultCollection string
	defaultTenantID   string
	topK              int
	logger            *slog.Logger
}

// NewRAGService creates a new instance of RAGService with multi-tenancy and metadata persistence support.
func NewRAGService(
	vectorRepo repositories.VectorRepository,
	metaRepo repositories.MetadataRepository,
	aiClient AIClient,
	chunker *DocumentChunker,
	defaultCollection string,
	defaultTenantID string,
	topK int,
	logger *slog.Logger,
) RAGService {
	if topK <= 0 {
		topK = 5
	}
	if defaultTenantID == "" {
		defaultTenantID = "default"
	}
	if defaultCollection == "" {
		defaultCollection = "rag-corpus"
	}

	return &ragServiceImpl{
		vectorRepo:        vectorRepo,
		metaRepo:          metaRepo,
		aiClient:          aiClient,
		chunker:           chunker,
		defaultCollection: defaultCollection,
		defaultTenantID:   defaultTenantID,
		topK:              topK,
		logger:            logger,
	}
}

// ProcessDocument splits a document, persists metadata and chunks in PostgreSQL, and upserts vectors into Qdrant.
func (s *ragServiceImpl) ProcessDocument(ctx context.Context, tenantID, collectionName, filename string, content []byte) (*models.RAGDocument, error) {
	if len(content) == 0 {
		return nil, apperrors.New(apperrors.ErrValidation, "Document content cannot be empty")
	}

	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = s.defaultTenantID
	}

	collectionName = strings.TrimSpace(collectionName)
	if collectionName == "" {
		collectionName = s.defaultCollection
	}

	chunks := s.chunker.Split(string(content))
	if len(chunks) == 0 {
		return nil, apperrors.New(apperrors.ErrValidation, "Document produced no text chunks after parsing")
	}

	docID := uuid.New().String()
	doc := &models.RAGDocument{
		DocumentID:     docID,
		TenantID:       tenantID,
		CollectionName: collectionName,
		Filename:       filename,
		FileSize:       int64(len(content)),
		ChunkCount:     len(chunks),
		Status:         "completed",
		CreatedAt:      time.Now().UTC(),
	}

	var vectorPoints []models.VectorPoint
	var ragChunks []models.RAGChunk

	for i, chunkText := range chunks {
		vec, err := s.aiClient.GetEmbedding(ctx, chunkText)
		if err != nil {
			s.logger.Warn("Failed to embed chunk",
				slog.String("tenant_id", tenantID),
				slog.String("filename", filename),
				slog.Int("chunk_index", i),
				slog.Any("error", err),
			)
			continue
		}

		pointID := uuid.New().String()
		chunkID := uuid.New().String()

		vectorPoints = append(vectorPoints, models.VectorPoint{
			ID:       pointID,
			Vector:   vec,
			TenantID: tenantID,
			Payload: map[string]interface{}{
				"text":        chunkText,
				"filename":    filename,
				"chunk_index": i,
				"tenant_id":   tenantID,
				"document_id": docID,
			},
		})

		ragChunks = append(ragChunks, models.RAGChunk{
			ChunkID:    chunkID,
			DocumentID: docID,
			TenantID:   tenantID,
			ChunkIndex: i,
			Content:    chunkText,
			PointID:    pointID,
			Metadata:   fmt.Sprintf(`{"filename":"%s","chunk_index":%d}`, filename, i),
			CreatedAt:  time.Now().UTC(),
		})
	}

	if len(vectorPoints) == 0 {
		return nil, apperrors.New(apperrors.ErrExternal, "Failed to embed any chunks from document")
	}

	// 1. Persist relational vectorstore metadata in PostgreSQL if available
	if s.metaRepo != nil {
		if err := s.metaRepo.SaveDocumentWithChunks(ctx, doc, ragChunks); err != nil {
			s.logger.Error("Failed to persist document metadata to PostgreSQL",
				slog.String("tenant_id", tenantID),
				slog.String("document_id", docID),
				slog.Any("error", err),
			)
			return nil, fmt.Errorf("failed to save document metadata in database: %w", err)
		}
	}

	// 2. Persist vector embeddings in Qdrant with tenant payload isolation
	if err := s.vectorRepo.UpsertPoints(ctx, collectionName, vectorPoints); err != nil {
		s.logger.Error("Failed to upsert points into vector repository",
			slog.String("tenant_id", tenantID),
			slog.String("collection", collectionName),
			slog.Any("error", err),
		)
		return nil, fmt.Errorf("failed to persist points to vector repository: %w", err)
	}

	s.logger.Info("Successfully processed and stored document and vector chunks",
		slog.String("tenant_id", tenantID),
		slog.String("collection", collectionName),
		slog.String("filename", filename),
		slog.String("document_id", docID),
		slog.Int("chunks_stored", len(vectorPoints)),
	)

	return doc, nil
}

// ProcessQuery performs semantic search with tenant isolation and asks the LLM for an answer.
func (s *ragServiceImpl) ProcessQuery(ctx context.Context, tenantID, collectionName, query string) (string, error) {
	answer, _, err := s.ProcessQueryWithSources(ctx, tenantID, collectionName, query)
	return answer, err
}

func (s *ragServiceImpl) ProcessQueryWithSources(ctx context.Context, tenantID, collectionName, query string) (string, []models.SourceReference, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", nil, apperrors.New(apperrors.ErrValidation, "Query cannot be empty")
	}

	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = s.defaultTenantID
	}

	collectionName = strings.TrimSpace(collectionName)
	if collectionName == "" {
		collectionName = s.defaultCollection
	}

	queryVector, err := s.aiClient.GetEmbedding(ctx, query)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	searchResults, err := s.vectorRepo.SearchPoints(ctx, collectionName, queryVector, s.topK, tenantID)
	if err != nil {
		return "", nil, fmt.Errorf("vector search failed: %w", err)
	}

	var contextBuilder strings.Builder
	var sources []models.SourceReference
	for _, result := range searchResults {
		if result.Content == "" {
			continue
		}
		contextBuilder.WriteString(result.Content)
		contextBuilder.WriteString("\n")

		filename := ""
		if payloadName, ok := result.Payload["filename"].(string); ok {
			filename = payloadName
		}
		documentID := ""
		if payloadDocID, ok := result.Payload["document_id"].(string); ok {
			documentID = payloadDocID
		}
		chunkIndex := 0
		if payloadChunk, ok := result.Payload["chunk_index"].(float64); ok {
			chunkIndex = int(payloadChunk)
		} else if payloadChunk, ok := result.Payload["chunk_index"].(int); ok {
			chunkIndex = payloadChunk
		}

		sources = append(sources, models.SourceReference{
			DocumentID: documentID,
			Filename:   filename,
			ChunkIndex: chunkIndex,
			Score:      result.Score,
		})
	}
	if len(searchResults) == 0 {
		queryTerms := strings.FieldsFunc(query, func(r rune) bool { return strings.ContainsRune(" ,;.!?\n\t", r) })
		for _, term := range queryTerms {
			if term != "" {
				contextBuilder.WriteString(term)
				contextBuilder.WriteString(" ")
			}
		}
	}
	contextText := contextBuilder.String()
	answer, err := s.aiClient.GenerateAnswer(ctx, contextText, query)
	if err != nil {
		return "", nil, fmt.Errorf("LLM answer generation failed: %w", err)
	}

	return answer, sources, nil
}

// ListDocuments returns all documents stored for a tenant in PostgreSQL.
func (s *ragServiceImpl) ListDocuments(ctx context.Context, tenantID, collectionName string) ([]models.RAGDocument, error) {
	if s.metaRepo == nil {
		return []models.RAGDocument{}, nil
	}

	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = s.defaultTenantID
	}

	collectionName = strings.TrimSpace(collectionName)
	return s.metaRepo.GetDocuments(ctx, tenantID, collectionName)
}

func (s *ragServiceImpl) ListCollections(ctx context.Context, tenantID string) ([]models.RAGCollection, error) {
	if s.metaRepo == nil {
		return []models.RAGCollection{}, nil
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = s.defaultTenantID
	}
	return s.metaRepo.ListCollections(ctx, tenantID)
}

func (s *ragServiceImpl) DeleteDocument(ctx context.Context, tenantID, collectionName, documentID string) error {
	if s.metaRepo == nil {
		return apperrors.New(apperrors.ErrDatabase, "Metadata repository is not configured")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = s.defaultTenantID
	}
	return s.metaRepo.DeleteDocument(ctx, tenantID, collectionName, documentID)
}

func (s *ragServiceImpl) UpdateDocumentStatus(ctx context.Context, tenantID, documentID, status string) error {
	if s.metaRepo == nil {
		return apperrors.New(apperrors.ErrDatabase, "Metadata repository is not configured")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = s.defaultTenantID
	}
	status = strings.TrimSpace(status)
	if status == "" {
		return apperrors.New(apperrors.ErrValidation, "Status cannot be empty")
	}
	return s.metaRepo.UpdateDocumentStatus(ctx, tenantID, documentID, status)
}
