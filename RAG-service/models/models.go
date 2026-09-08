package models

import "time"

// DocumentChunk represents a chunk of text extracted from a document.
type DocumentChunk struct {
	Index     int
	Content   string
	Embedding []float64
	Metadata  map[string]interface{}
}

// VectorPoint represents a point to be persisted in a vector database.
type VectorPoint struct {
	ID       string                 `json:"id"`
	Vector   []float64              `json:"vector"`
	Payload  map[string]interface{} `json:"payload"`
	TenantID string                 `json:"tenant_id,omitempty"`
}

// SearchResult represents a semantic search match returned from the vector store.
type SearchResult struct {
	ID       string
	Content  string
	Score    float64
	Payload  map[string]interface{}
	TenantID string
}

// RAGTenant represents a tenant owning collections and documents.
type RAGTenant struct {
	TenantID  string    `db:"tenant_id" json:"tenant_id"`
	Name      string    `db:"name" json:"name"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// RAGCollection represents an isolated collection or knowledge base for a tenant.
type RAGCollection struct {
	CollectionID string    `db:"collection_id" json:"collection_id"`
	TenantID     string    `db:"tenant_id" json:"tenant_id"`
	Name         string    `db:"name" json:"name"`
	VectorDB     string    `db:"vector_db" json:"vector_db"`
	Dimension    int       `db:"dimension" json:"dimension"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

// RAGDocument represents an ingested document stored in PostgreSQL.
type RAGDocument struct {
	DocumentID     string    `db:"document_id" json:"document_id"`
	TenantID       string    `db:"tenant_id" json:"tenant_id"`
	CollectionName string    `db:"collection_name" json:"collection_name"`
	Filename       string    `db:"filename" json:"filename"`
	FileSize       int64     `db:"file_size" json:"file_size"`
	ChunkCount     int       `db:"chunk_count" json:"chunk_count"`
	Status         string    `db:"status" json:"status"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
}

// RAGChunk represents a persistent document chunk mapped to a vector point.
type RAGChunk struct {
	ChunkID    string    `db:"chunk_id" json:"chunk_id"`
	DocumentID string    `db:"document_id" json:"document_id"`
	TenantID   string    `db:"tenant_id" json:"tenant_id"`
	ChunkIndex int       `db:"chunk_index" json:"chunk_index"`
	Content    string    `db:"content" json:"content"`
	PointID    string    `db:"point_id" json:"point_id"`
	Metadata   string    `db:"metadata" json:"metadata,omitempty"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}
