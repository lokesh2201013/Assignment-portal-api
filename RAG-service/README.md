# RAG Service

## Run locally

Start PostgreSQL, Qdrant, and Ollama first. Pull the configured LLM and embedding models, then from this directory:

```powershell
go mod download
go run .
```

HTTP defaults to `8001`; gRPC defaults to `50052`. Swagger UI is at `http://localhost:8001/docs`; OpenAPI YAML is served at `http://localhost:8001/openapi.yaml`. Swagger UI assets load from the Swagger UI CDN. gRPC definitions are in `proto/`.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `APP_PORT` | `8001` | HTTP listener |
| `GRPC_PORT` | `50052` | gRPC listener |
| `DATABASE_DSN` / `DATABASE_URL` | local PostgreSQL URL | Metadata database |
| `QDRANT_URL` | `http://localhost:6333` | Vector database endpoint |
| `QDRANT_COLLECTION` | `rag-corpus` | Default vector collection |
| `OLLAMA_URL` | `http://localhost:11434` | Ollama endpoint |
| `EMBEDDING_MODEL` | `nomic-embed-text` | Embedding model |
| `LLM_MODEL` | `llama3.1:8b` | Answer-generation model |
| `DEFAULT_TENANT_ID` | `default` | Fallback tenant |
| `CHUNK_SIZE` / `CHUNK_OVERLAP` | `250` / `20` | Chunking configuration |
| `TOP_K` | `5` | Vector results used for context |
| `MAX_FILE_SIZE_BYTES` | `20971520` | Upload limit |
| `JAEGER_ENDPOINT` / `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4318` | OTLP HTTP collector base; `/v1/traces` is appended |

## Data stores and checks

PostgreSQL metadata schema is initialized on startup. Qdrant stores vectors and Ollama provides embeddings/answers; see [database and vector-store reference](docs/database.md).

```powershell
go test ./...
```
