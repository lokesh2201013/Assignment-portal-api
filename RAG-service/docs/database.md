# RAG Metadata and Vector Stores

PostgreSQL tables and indexes are initialized by `database/postgres.go` during service startup. This is idempotent bootstrap SQL, not a versioned migration framework. Qdrant is a separate vector store and is not covered by PostgreSQL backups alone.

## PostgreSQL tables

| Table | Purpose | Relationships / indexes |
|---|---|---|
| `rag_tenants` | Tenant registry | `tenant_id` primary key; a `default` tenant is seeded |
| `rag_collections` | Collection metadata | UUID primary key; tenant FK with cascade; unique `(tenant_id, name)`; vector backend and dimension |
| `rag_documents` | Uploaded document metadata | UUID primary key; tenant FK; collection name, file size, chunk count, status, timestamp |
| `rag_chunks` | Text chunks and vector-point mapping | UUID primary key; document and tenant FKs with cascade; text, chunk index, Qdrant `point_id`, JSONB metadata |

Indexes cover tenant collections, tenant/collection document listing, tenant/document chunk lookup, and Qdrant point IDs. Deleting a metadata document cascades to its relational chunks; vector points are stored outside PostgreSQL and must also be removed by the service to avoid orphaned vectors.

## Qdrant and Ollama

Qdrant stores vectors and payloads, including tenant/document identifiers used for filtering and citations. Collection dimensions must match the configured embedding model. PostgreSQL stores metadata and chunk text; treat both stores as a coordinated backup/restore unit. Ollama is an external model runtime and is not a database.

## Connection and privacy

Set `DATABASE_DSN` or `DATABASE_URL`. The default connection is local development only. Tenant IDs are security boundaries: all custom SQL and Qdrant filters must preserve tenant scoping. Chunk text and vector payloads can contain source-document data; apply access controls and retention policies to both stores.
