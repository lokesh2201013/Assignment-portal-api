# Assignment API operations

## Local setup

Run commands from this module directory (`Assignment-api/assignment-portal`):

```powershell
go test ./...
go run .
```

PostgreSQL is required. Set `DB_HOST`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, and
`DB_PORT` in the process environment or a local `.env` file. `ELASTICSEARCH_URL`
selects the Elasticsearch HTTP(S) endpoint; it defaults to
`http://localhost:9200`. Startup fails if PostgreSQL cannot initialize or if
Elasticsearch cannot be reached within five seconds. The HTTP API listens on
port `8080`.

Example PowerShell setup (replace the local values; do not commit credentials):

```powershell
$env:DB_HOST = "localhost"
$env:DB_PORT = "5432"
$env:DB_USER = "postgres"
$env:DB_PASSWORD = "<local-password>"
$env:DB_NAME = "assignment_portal"
$env:ELASTICSEARCH_URL = "http://localhost:9200"
$env:INTERNAL_GRPC_TOKEN = "<shared-internal-token>"
go run .
```

The `INTERNAL_GRPC_TOKEN` value must match the email service's shared token.
The Assignment API sends it as gRPC metadata `x-internal-token`; notification
requests fail closed when it is unset. See `.env.example` for placeholder
configuration only; do not use its placeholder values as credentials.

The assignment/tenant SQL repositories use tables beyond those created by the
current GORM `AutoMigrate` call. Provision their required schema before using
the tenant onboarding, assignment service, grading, dashboard, or revision
routes. This repository currently does not ship a versioned SQL migration for
that schema. The older signup, login, upload, and video handlers use the GORM
models and are not a substitute for those SQL tables.

The notifier connects to the email gRPC service at `localhost:50051` and sends
the `INTERNAL_GRPC_TOKEN` as `x-internal-token`; document
assistant/file upload handlers connect to the RAG service at `localhost:50052`.
Those addresses are currently fixed in the handlers. The legacy video handler
also expects RabbitMQ at `localhost:5672`; its current failure helper terminates
the API process if the broker is unavailable. These auxiliary services are not
needed to start the API.

## Implemented HTTP workflows

- `POST /signup` remains limited to the `default` tenant. `POST /login` issues
  a token for the matched user's tenant; when the same email exists in multiple
  tenants, provide `tenant_id` to select the account. `POST /tenants` and
  `POST /tenant-invitations/accept` implement tenant onboarding.
- Authenticated tenant handlers implement invitations, bulk roster import,
  courses, and member status. Admin assignment handlers implement paginated
  assignment/submission lists, assignment creation, grading, and student
  dashboard/revision reads. The exact paths and methods are in
  [the OpenAPI document](docs/openapi.yaml).
- `POST /user/upload` stores multipart files in local `uploads` directories
  and applies the assignment's due-date/resubmission rules. It does not use
  GCS, ClamAV, or signed downloads.
- The legacy assistant/media endpoints are only partially operational and are
  not described as supported workflows in the OpenAPI document. In particular,
  the analytics handler asks a local Ollama instance to generate SQL and runs
  it directly; do not expose that endpoint to untrusted users or production
  traffic.

## Configuration limits

- `PUBLIC_APP_URL` supplies the base URL for tenant invitation links. The
  notifier service must be reachable for invitation/assignment emails.
- Elasticsearch can be configured with `ELASTICSEARCH_URL`; only one HTTP(S)
  endpoint is currently supported.
- Database TLS is currently hard-coded to `sslmode=disable`; use only for
  local development or change the database configuration before deployment.
- Do not commit `.env` files or credentials.

See [database notes](docs/database.md) for the current database initialization
and schema limitations.
