# Email Microservice

## Run locally

Copy `.env.example` to an untracked `.env` and replace the placeholder database, JWT, and SMTP values. Never commit `.env`. For example:

```powershell
Copy-Item .env.example .env
go mod download
go run .
```

HTTP defaults to `3000`; gRPC defaults to `50051`. Interactive API docs are at `http://localhost:3000/docs` and the served OpenAPI contract is `http://localhost:3000/openapi.yaml` (Swagger UI assets load from its CDN).

All REST endpoints except `/health` and `/api/v1/auth/*` require `Authorization: Bearer <token>`. Obtain a token from `/api/v1/auth/login`.

## Required configuration

`DB_HOST`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `JWT_SECRET`, `SMTP_HOST`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM`, `SMTP_CREDENTIALS_KEY`, and `INTERNAL_GRPC_TOKEN` are required; missing values stop startup. Generate `SMTP_CREDENTIALS_KEY` with a cryptographically secure random source (for example, `openssl rand -base64 32`); it must be base64 encoding of exactly 32 bytes and encrypts sender SMTP passwords in PostgreSQL with AES-256-GCM. `INTERNAL_GRPC_TOKEN` must be at least 32 characters; Assignment API gRPC clients must send it as `x-internal-token` metadata. Keep both secrets in a secret manager in production and back up the encryption key securely: losing or rotating it without re-encrypting sender credentials makes stored credentials unreadable. Startup encrypts legacy plaintext sender passwords in place.

`DB_PORT` defaults to `5432`; `DB_SSLMODE` defaults to `require`. `SMTP_PORT` defaults to `587`. `HTTP_PORT` and `GRPC_PORT` default to `3000` and `50051`. Both servers start together, and the process stops if either server fails.

For Docker Compose, configure `.env` from the template and run `docker compose up --build`. Compose starts PostgreSQL before the service, maps HTTP and gRPC ports, and uses a persistent PostgreSQL volume. The container image excludes `.env`; Compose passes configuration at runtime.

`POST /api/v1/emails/send` persists one queue record per recipient and returns `202 Accepted` with their `message_ids`; it never waits for SMTP. The worker polls every `EMAIL_WORKER_INTERVAL` (default `2s`) in batches of `EMAIL_WORKER_BATCH_SIZE` (default `10`). Failed attempts use exponential backoff and stop after `EMAIL_MAX_RETRIES` total attempts (default `3`). A worker can safely reclaim a job left in `processing` for five minutes after an interruption.

REST email sending requires a verified sender identity owned by the authenticated user; it never falls back to the service-wide SMTP identity. Only the authenticated internal gRPC Assignment notification path can queue messages using global `SMTP_*` credentials.

Admins can inspect queue state with `GET /api/v1/emails/history?status=failed&recipient=...`, inspect a message and every SMTP attempt with `GET /api/v1/emails/history/:id`, and place a non-delivered message back in the queue with `POST /api/v1/emails/retry/:id`. Manual retries reset the attempt budget while preserving the prior immutable attempt history. See [database reference](database.md).

## Checks

```powershell
go test ./...
```
