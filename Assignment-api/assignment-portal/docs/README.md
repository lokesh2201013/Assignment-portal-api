# Assignment API

## Run locally

From this directory, set the required database, JWT, object-storage and malware-scanner settings, then run:

```powershell
go mod download
go run .
```

The HTTP server defaults to port `8080`. Open Swagger UI at `http://localhost:8080/docs`; the exact OpenAPI YAML is at `http://localhost:8080/openapi.yaml`. Swagger UI assets load from the Swagger UI CDN.

PostgreSQL tables are created/altered during startup. See [database reference](database.md). The service also calls the Email gRPC service at `EMAIL_SERVICE_ADDR` and RAG at `RAG_SERVICE_ADDR`.

## Configuration

`DATABASE_DSN` is preferred. Otherwise the service assembles a DSN from `DB_HOST`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, and `DB_PORT`; `DB_SSLMODE` defaults to `disable`.

Important settings: `PORT` (8080), `JWT_SECRET` (replace the development fallback), `JWT_ISSUER`, `JWT_TTL_HOURS`, `ALLOWED_ORIGINS`, `EMAIL_SERVICE_ADDR` (localhost:50051), `RAG_SERVICE_ADDR` (localhost:50052), `PUBLIC_APP_URL`, `GCS_BUCKET_NAME`, `GCS_ACCESS_ID`, `GCS_PRIVATE_KEY`, `CLAMAV_ADDRESS`, `UPLOAD_MAX_BYTES` (10 MiB), `UPLOAD_RETENTION_DAYS` (365), `ELASTIC_ADDRESSES`, and `RABBITMQ_URL`.

Tracing exports OTLP over HTTP. Set `JAEGER_ENDPOINT` or `OTEL_EXPORTER_OTLP_ENDPOINT` to the collector base URL (default `http://localhost:4318`). The exporter posts to `/v1/traces`. Request metrics are available at `/metrics`.

## Authentication and tenant scope

`POST /signup` is the legacy default-tenant registration path. Create a tenant with `POST /tenants`, accept its owner invite at `POST /tenant-invitations/accept`, and issue member invites with authenticated `POST /tenant/invitations`. Protected operations derive the tenant from the verified JWT; do not trust a client-provided tenant header as an authorization boundary.

## Checks

```powershell
go test ./...
```
