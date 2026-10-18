# Email Microservice

## Run locally

Create an untracked `.env` with the required database, JWT, and SMTP credentials, then:

```powershell
go mod download
go run .
```

HTTP defaults to `3000`; gRPC defaults to `50051`. Interactive API docs are at `http://localhost:3000/docs` and the served OpenAPI contract is `http://localhost:3000/openapi.yaml` (Swagger UI assets load from its CDN).

All REST endpoints except `/health` and `/api/v1/auth/*` require `Authorization: Bearer <token>`. Obtain a token from `/api/v1/auth/login`.

## Required configuration

`DB_HOST`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, and `JWT_SECRET` are required. `DB_PORT` defaults to `5432`; `DB_SSLMODE` defaults to `require`. SMTP requires `SMTP_HOST`, `SMTP_USERNAME`, `SMTP_PASSWORD`, and `SMTP_FROM`; `SMTP_PORT` defaults to `587`. `HTTP_PORT` and `GRPC_PORT` default to `3000` and `50051`. `JAEGER_ENDPOINT` or `OTEL_EXPORTER_OTLP_ENDPOINT` selects the OTLP HTTP collector (default `http://localhost:4318`, with `/v1/traces` appended).

The current send path delivers synchronously; the API response reports the send attempt, not a queued job. See [database reference](database.md).

## Checks

```powershell
go test ./...
```
