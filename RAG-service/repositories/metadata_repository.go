package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/atgsgrouptest/genet-microservice/RAG-service/apperrors"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// MetadataRepository defines the persistence contract for multi-tenant metadata in PostgreSQL.
type MetadataRepository interface {
	EnsureTenant(ctx context.Context, tenantID, name string) error
	EnsureCollection(ctx context.Context, tenantID, collectionName string, dimension int) (*models.RAGCollection, error)
	SaveDocumentWithChunks(ctx context.Context, doc *models.RAGDocument, chunks []models.RAGChunk) error
	GetDocuments(ctx context.Context, tenantID, collectionName string) ([]models.RAGDocument, error)
	GetChunksByDocument(ctx context.Context, tenantID, docID string) ([]models.RAGChunk, error)
	GetChunksByPointIDs(ctx context.Context, tenantID string, pointIDs []string) ([]models.RAGChunk, error)
	Ping(ctx context.Context) error
}

// PostgresMetadataRepository implements MetadataRepository using sqlx and PostgreSQL.
type PostgresMetadataRepository struct {
	db     *sqlx.DB
	logger *slog.Logger
}

// NewPostgresMetadataRepository creates a new PostgresMetadataRepository.
func NewPostgresMetadataRepository(db *sqlx.DB, logger *slog.Logger) *PostgresMetadataRepository {
	return &PostgresMetadataRepository{
		db:     db,
		logger: logger,
	}
}

// EnsureTenant verifies or creates a tenant record in rag_tenants.
func (r *PostgresMetadataRepository) EnsureTenant(ctx context.Context, tenantID, name string) error {
	if tenantID == "" {
		tenantID = "default"
	}
	if name == "" {
		name = tenantID
	}

	query := `
		INSERT INTO rag_tenants (tenant_id, name, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (tenant_id) DO NOTHING;
	`
	_, err := r.db.ExecContext(ctx, query, tenantID, name)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Failed to ensure tenant", err)
	}
	return nil
}

// EnsureCollection verifies or creates a collection for a given tenant.
func (r *PostgresMetadataRepository) EnsureCollection(ctx context.Context, tenantID, collectionName string, dimension int) (*models.RAGCollection, error) {
	if tenantID == "" {
		tenantID = "default"
	}
	if collectionName == "" {
		collectionName = "rag-corpus"
	}
	if dimension <= 0 {
		dimension = 768
	}

	if err := r.EnsureTenant(ctx, tenantID, tenantID); err != nil {
		return nil, err
	}

	collectionID := uuid.New().String()
	query := `
		INSERT INTO rag_collections (collection_id, tenant_id, name, vector_db, dimension, created_at)
		VALUES ($1, $2, $3, 'qdrant', $4, NOW())
		ON CONFLICT (tenant_id, name) DO UPDATE SET name = EXCLUDED.name
		RETURNING collection_id, tenant_id, name, vector_db, dimension, created_at;
	`

	var coll models.RAGCollection
	err := r.db.QueryRowxContext(ctx, query, collectionID, tenantID, collectionName, dimension).StructScan(&coll)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Failed to ensure collection in PostgreSQL", err)
	}

	return &coll, nil
}

// SaveDocumentWithChunks persists document metadata and all corresponding chunks in an atomic transaction.
func (r *PostgresMetadataRepository) SaveDocumentWithChunks(ctx context.Context, doc *models.RAGDocument, chunks []models.RAGChunk) error {
	if doc == nil {
		return apperrors.New(apperrors.ErrValidation, "Document cannot be nil")
	}
	if doc.DocumentID == "" {
		doc.DocumentID = uuid.New().String()
	}
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = time.Now().UTC()
	}
	if doc.TenantID == "" {
		doc.TenantID = "default"
	}

	if err := r.EnsureTenant(ctx, doc.TenantID, doc.TenantID); err != nil {
		return err
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Failed to begin transaction", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	docQuery := `
		INSERT INTO rag_documents (document_id, tenant_id, collection_name, filename, file_size, chunk_count, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8);
	`
	_, err = tx.ExecContext(ctx, docQuery,
		doc.DocumentID,
		doc.TenantID,
		doc.CollectionName,
		doc.Filename,
		doc.FileSize,
		len(chunks),
		doc.Status,
		doc.CreatedAt,
	)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Failed to insert document metadata", err)
	}

	chunkQuery := `
		INSERT INTO rag_chunks (chunk_id, document_id, tenant_id, chunk_index, content, point_id, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8);
	`
	for _, chunk := range chunks {
		chunkID := chunk.ChunkID
		if chunkID == "" {
			chunkID = uuid.New().String()
		}
		createdAt := chunk.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}

		metadataJSON := chunk.Metadata
		if metadataJSON == "" {
			metadataJSON = "{}"
		}

		_, err = tx.ExecContext(ctx, chunkQuery,
			chunkID,
			doc.DocumentID,
			doc.TenantID,
			chunk.ChunkIndex,
			chunk.Content,
			chunk.PointID,
			metadataJSON,
			createdAt,
		)
		if err != nil {
			return apperrors.Wrap(apperrors.ErrDatabase, fmt.Sprintf("Failed to insert chunk index %d", chunk.ChunkIndex), err)
		}
	}

	if err := tx.Commit(); err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Failed to commit document and chunks transaction", err)
	}

	r.logger.Debug("Persisted document and vectorstore chunks to PostgreSQL",
		slog.String("tenant_id", doc.TenantID),
		slog.String("document_id", doc.DocumentID),
		slog.Int("chunk_count", len(chunks)),
	)
	return nil
}

// GetDocuments retrieves all documents belonging to a tenant and collection.
func (r *PostgresMetadataRepository) GetDocuments(ctx context.Context, tenantID, collectionName string) ([]models.RAGDocument, error) {
	if tenantID == "" {
		tenantID = "default"
	}

	var query string
	var args []interface{}

	if collectionName != "" {
		query = `
			SELECT document_id, tenant_id, collection_name, filename, file_size, chunk_count, status, created_at
			FROM rag_documents
			WHERE tenant_id = $1 AND collection_name = $2
			ORDER BY created_at DESC;
		`
		args = []interface{}{tenantID, collectionName}
	} else {
		query = `
			SELECT document_id, tenant_id, collection_name, filename, file_size, chunk_count, status, created_at
			FROM rag_documents
			WHERE tenant_id = $1
			ORDER BY created_at DESC;
		`
		args = []interface{}{tenantID}
	}

	var docs []models.RAGDocument
	err := r.db.SelectContext(ctx, &docs, query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []models.RAGDocument{}, nil
		}
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Failed to fetch documents", err)
	}
	return docs, nil
}

// GetChunksByDocument retrieves all chunks stored for a document.
func (r *PostgresMetadataRepository) GetChunksByDocument(ctx context.Context, tenantID, docID string) ([]models.RAGChunk, error) {
	query := `
		SELECT chunk_id, document_id, tenant_id, chunk_index, content, point_id, COALESCE(metadata::text, '{}') as metadata, created_at
		FROM rag_chunks
		WHERE tenant_id = $1 AND document_id = $2
		ORDER BY chunk_index ASC;
	`
	var chunks []models.RAGChunk
	err := r.db.SelectContext(ctx, &chunks, query, tenantID, docID)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Failed to fetch chunks", err)
	}
	return chunks, nil
}

// GetChunksByPointIDs fetches chunks matching vector point IDs for a tenant.
func (r *PostgresMetadataRepository) GetChunksByPointIDs(ctx context.Context, tenantID string, pointIDs []string) ([]models.RAGChunk, error) {
	if len(pointIDs) == 0 {
		return []models.RAGChunk{}, nil
	}

	query, args, err := sqlx.In(`
		SELECT chunk_id, document_id, tenant_id, chunk_index, content, point_id, COALESCE(metadata::text, '{}') as metadata, created_at
		FROM rag_chunks
		WHERE tenant_id = ? AND point_id IN (?);
	`, tenantID, pointIDs)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternal, "Failed to build IN query", err)
	}

	query = r.db.Rebind(query)
	var chunks []models.RAGChunk
	err = r.db.SelectContext(ctx, &chunks, query, args...)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Failed to fetch chunks by point IDs", err)
	}
	return chunks, nil
}

// Ping checks PostgreSQL connectivity.
func (r *PostgresMetadataRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}
