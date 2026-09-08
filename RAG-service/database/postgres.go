package database

import (
	"fmt"
	"log/slog"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

var DB *sqlx.DB

// ConnectDB establishes a pooled connection to PostgreSQL and auto-migrates the RAG multi-tenancy schema.
func ConnectDB(databaseDSN string, logger *slog.Logger) (*sqlx.DB, error) {
	if databaseDSN == "" {
		return nil, fmt.Errorf("database DSN cannot be empty")
	}

	db, err := sqlx.Open("pgx", databaseDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := createTables(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to run database migrations: %w", err)
	}

	DB = db
	logger.Info("Connected to PostgreSQL and verified RAG vectorstore schema")
	return db, nil
}

func createTables(db *sqlx.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS rag_tenants (
			tenant_id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`INSERT INTO rag_tenants (tenant_id, name)
		 VALUES ('default', 'Default Tenant')
		 ON CONFLICT (tenant_id) DO NOTHING;`,
		`CREATE TABLE IF NOT EXISTS rag_collections (
			collection_id UUID PRIMARY KEY,
			tenant_id TEXT NOT NULL REFERENCES rag_tenants(tenant_id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			vector_db TEXT NOT NULL DEFAULT 'qdrant',
			dimension INTEGER NOT NULL DEFAULT 768,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(tenant_id, name)
		);`,
		`CREATE TABLE IF NOT EXISTS rag_documents (
			document_id UUID PRIMARY KEY,
			tenant_id TEXT NOT NULL REFERENCES rag_tenants(tenant_id) ON DELETE CASCADE,
			collection_name TEXT NOT NULL,
			filename TEXT NOT NULL,
			file_size BIGINT NOT NULL DEFAULT 0,
			chunk_count INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'completed',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE TABLE IF NOT EXISTS rag_chunks (
			chunk_id UUID PRIMARY KEY,
			document_id UUID NOT NULL REFERENCES rag_documents(document_id) ON DELETE CASCADE,
			tenant_id TEXT NOT NULL REFERENCES rag_tenants(tenant_id) ON DELETE CASCADE,
			chunk_index INTEGER NOT NULL,
			content TEXT NOT NULL,
			point_id TEXT NOT NULL,
			metadata JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`CREATE INDEX IF NOT EXISTS idx_rag_collections_tenant ON rag_collections(tenant_id);`,
		`CREATE INDEX IF NOT EXISTS idx_rag_documents_tenant_coll ON rag_documents(tenant_id, collection_name);`,
		`CREATE INDEX IF NOT EXISTS idx_rag_chunks_tenant_doc ON rag_chunks(tenant_id, document_id);`,
		`CREATE INDEX IF NOT EXISTS idx_rag_chunks_point_id ON rag_chunks(point_id);`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("failed executing statement: %s, error: %w", stmt, err)
		}
	}

	return nil
}
