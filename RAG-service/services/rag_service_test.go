package services

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/atgsgrouptest/genet-microservice/RAG-service/apperrors"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockVectorRepository mocks repositories.VectorRepository
type MockVectorRepository struct {
	mock.Mock
}

func (m *MockVectorRepository) UpsertPoints(ctx context.Context, collection string, points []models.VectorPoint) error {
	args := m.Called(ctx, collection, points)
	return args.Error(0)
}

func (m *MockVectorRepository) SearchPoints(ctx context.Context, collection string, vector []float64, limit int, tenantID string) ([]models.SearchResult, error) {
	args := m.Called(ctx, collection, vector, limit, tenantID)
	return args.Get(0).([]models.SearchResult), args.Error(1)
}

func (m *MockVectorRepository) Health(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockMetadataRepository mocks repositories.MetadataRepository
type MockMetadataRepository struct {
	mock.Mock
}

func (m *MockMetadataRepository) EnsureTenant(ctx context.Context, tenantID, name string) error {
	args := m.Called(ctx, tenantID, name)
	return args.Error(0)
}

func (m *MockMetadataRepository) EnsureCollection(ctx context.Context, tenantID, collectionName string, dimension int) (*models.RAGCollection, error) {
	args := m.Called(ctx, tenantID, collectionName, dimension)
	return args.Get(0).(*models.RAGCollection), args.Error(1)
}

func (m *MockMetadataRepository) SaveDocumentWithChunks(ctx context.Context, doc *models.RAGDocument, chunks []models.RAGChunk) error {
	args := m.Called(ctx, doc, chunks)
	return args.Error(0)
}

func (m *MockMetadataRepository) GetDocuments(ctx context.Context, tenantID, collectionName string) ([]models.RAGDocument, error) {
	args := m.Called(ctx, tenantID, collectionName)
	return args.Get(0).([]models.RAGDocument), args.Error(1)
}

func (m *MockMetadataRepository) ListCollections(ctx context.Context, tenantID string) ([]models.RAGCollection, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]models.RAGCollection), args.Error(1)
}

func (m *MockMetadataRepository) DeleteDocument(ctx context.Context, tenantID, collectionName, documentID string) error {
	args := m.Called(ctx, tenantID, collectionName, documentID)
	return args.Error(0)
}

func (m *MockMetadataRepository) UpdateDocumentStatus(ctx context.Context, tenantID, documentID, status string) error {
	args := m.Called(ctx, tenantID, documentID, status)
	return args.Error(0)
}

func (m *MockMetadataRepository) GetChunksByDocument(ctx context.Context, tenantID, docID string) ([]models.RAGChunk, error) {
	args := m.Called(ctx, tenantID, docID)
	return args.Get(0).([]models.RAGChunk), args.Error(1)
}

func (m *MockMetadataRepository) GetChunksByPointIDs(ctx context.Context, tenantID string, pointIDs []string) ([]models.RAGChunk, error) {
	args := m.Called(ctx, tenantID, pointIDs)
	return args.Get(0).([]models.RAGChunk), args.Error(1)
}

func (m *MockMetadataRepository) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockAIClient mocks AIClient
type MockAIClient struct {
	mock.Mock
}

func (m *MockAIClient) GetEmbedding(ctx context.Context, text string) ([]float64, error) {
	args := m.Called(ctx, text)
	if res := args.Get(0); res != nil {
		return res.([]float64), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAIClient) GenerateAnswer(ctx context.Context, contextText, question string) (string, error) {
	args := m.Called(ctx, contextText, question)
	return args.String(0), args.Error(1)
}

func TestRAGService_ProcessDocument(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	chunker := NewDocumentChunker(50, 10)

	t.Run("empty content returns validation error", func(t *testing.T) {
		repo := new(MockVectorRepository)
		meta := new(MockMetadataRepository)
		ai := new(MockAIClient)
		svc := NewRAGService(repo, meta, ai, chunker, "test-coll", "default", 5, logger)

		doc, err := svc.ProcessDocument(context.Background(), "default", "test-coll", "test.txt", []byte{})
		assert.Nil(t, doc)
		assert.True(t, errors.Is(err, apperrors.ErrValidation))
	})

	t.Run("successful document chunking, postgres persistence, and vector upsert", func(t *testing.T) {
		repo := new(MockVectorRepository)
		meta := new(MockMetadataRepository)
		ai := new(MockAIClient)
		svc := NewRAGService(repo, meta, ai, chunker, "test-coll", "tenant_1", 5, logger)

		content := []byte("This is a test document that should be processed.")
		mockEmbedding := []float64{0.1, 0.2, 0.3}

		ai.On("GetEmbedding", mock.Anything, mock.AnythingOfType("string")).Return(mockEmbedding, nil)
		meta.On("SaveDocumentWithChunks", mock.Anything, mock.MatchedBy(func(doc *models.RAGDocument) bool {
			return doc.TenantID == "tenant_1" && doc.Filename == "sample.txt"
		}), mock.MatchedBy(func(chunks []models.RAGChunk) bool {
			return len(chunks) > 0 && chunks[0].TenantID == "tenant_1"
		})).Return(nil)

		repo.On("UpsertPoints", mock.Anything, "test-coll", mock.MatchedBy(func(points []models.VectorPoint) bool {
			return len(points) > 0 && points[0].TenantID == "tenant_1"
		})).Return(nil)

		doc, err := svc.ProcessDocument(context.Background(), "tenant_1", "test-coll", "sample.txt", content)
		assert.NoError(t, err)
		assert.NotNil(t, doc)
		assert.Equal(t, "tenant_1", doc.TenantID)
		assert.Equal(t, "sample.txt", doc.Filename)

		ai.AssertExpectations(t)
		meta.AssertExpectations(t)
		repo.AssertExpectations(t)
	})
}

func TestRAGService_ProcessQuery(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	chunker := NewDocumentChunker(50, 10)

	t.Run("empty query returns validation error", func(t *testing.T) {
		repo := new(MockVectorRepository)
		meta := new(MockMetadataRepository)
		ai := new(MockAIClient)
		svc := NewRAGService(repo, meta, ai, chunker, "test-coll", "default", 5, logger)

		ans, err := svc.ProcessQuery(context.Background(), "default", "test-coll", "   ")
		assert.Empty(t, ans)
		assert.True(t, errors.Is(err, apperrors.ErrValidation))
	})

	t.Run("successful query search with tenant isolation and LLM completion", func(t *testing.T) {
		repo := new(MockVectorRepository)
		meta := new(MockMetadataRepository)
		ai := new(MockAIClient)
		svc := NewRAGService(repo, meta, ai, chunker, "test-coll", "tenant_xyz", 5, logger)

		query := "What is RAG?"
		mockEmbedding := []float64{0.5, 0.6}
		mockResults := []models.SearchResult{
			{Content: "RAG stands for Retrieval-Augmented Generation.", Score: 0.95, TenantID: "tenant_xyz", Payload: map[string]interface{}{"filename": "rag-guide.txt", "document_id": "doc-1", "chunk_index": 1}},
		}

		ai.On("GetEmbedding", mock.Anything, query).Return(mockEmbedding, nil)
		repo.On("SearchPoints", mock.Anything, "test-coll", mockEmbedding, 5, "tenant_xyz").Return(mockResults, nil)
		ai.On("GenerateAnswer", mock.Anything, "RAG stands for Retrieval-Augmented Generation.\n", query).Return("It combines retrieval with generative AI.", nil)

		answer, err := svc.ProcessQuery(context.Background(), "tenant_xyz", "test-coll", query)
		assert.NoError(t, err)
		assert.Equal(t, "It combines retrieval with generative AI.", answer)

		ai.AssertExpectations(t)
		repo.AssertExpectations(t)
	})

	t.Run("query returns citation metadata for each source", func(t *testing.T) {
		repo := new(MockVectorRepository)
		meta := new(MockMetadataRepository)
		ai := new(MockAIClient)
		svc := NewRAGService(repo, meta, ai, chunker, "test-coll", "tenant_xyz", 5, logger)

		query := "What is RAG?"
		mockEmbedding := []float64{0.5, 0.6}
		mockResults := []models.SearchResult{
			{Content: "RAG stands for Retrieval-Augmented Generation.", Score: 0.97, TenantID: "tenant_xyz", Payload: map[string]interface{}{"filename": "rag-guide.txt", "document_id": "doc-1", "chunk_index": 2}},
		}

		ai.On("GetEmbedding", mock.Anything, query).Return(mockEmbedding, nil)
		repo.On("SearchPoints", mock.Anything, "test-coll", mockEmbedding, 5, "tenant_xyz").Return(mockResults, nil)
		ai.On("GenerateAnswer", mock.Anything, "RAG stands for Retrieval-Augmented Generation.\n", query).Return("It combines retrieval with generative AI.", nil)

		answer, sources, err := svc.ProcessQueryWithSources(context.Background(), "tenant_xyz", "test-coll", query)
		assert.NoError(t, err)
		assert.Equal(t, "It combines retrieval with generative AI.", answer)
		assert.Len(t, sources, 1)
		assert.Equal(t, "rag-guide.txt", sources[0].Filename)
		assert.Equal(t, "doc-1", sources[0].DocumentID)
		assert.Equal(t, 2, sources[0].ChunkIndex)
	})
}
