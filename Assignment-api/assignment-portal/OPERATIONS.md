# Operations

## Tenant onboarding

- `POST /tenants` creates a pending tenant and sends a 48-hour, single-use owner invitation to `owner_email`.
- `POST /tenant-invitations/accept` consumes `{ "token", "name", "password", "branch", "semester" }`. Owner invitations activate a pending tenant. Student invitations require branch and semester.
- `POST /signup` is limited to the legacy `default` tenant. It cannot create membership in an arbitrary tenant.
- A verified tenant admin can call `POST /tenant/invitations` with `{ "email", "role" }` to invite users/admins. Revoked or pending memberships are rejected by protected-route auth checks.
- `POST /admin/invitations/bulk` accepts up to 200 invitations with optional role, branch, semester, and course assignments.
- `POST /admin/roster/import` accepts a multipart CSV file (columns: `email` required; `role`, `name`, `branch`, `semester`, `course_id` optional) for bulk student onboarding.
- `POST /admin/courses` and `GET /admin/courses` manage courses/classes within the tenant. Assignments can be scoped to a course ID to automatically target enrolled students.
- `GET /admin/users` lists tenant members with optional `status` filtering (`pending`, `verified`, `revoked`) and pagination.
- `POST /admin/users/:user_id/status` manages member activation (`verified` or `revoked`). Self-deactivation and deactivating the last verified administrator are prohibited.
- Set `PUBLIC_APP_URL` to the portal origin used to construct invitation URLs. The email service must be configured and reachable.

## Assessment and learning workflow

- Assignments support assessment configuration: `max_score` (default 100), `late_policy` (`allow`, `reject`, `penalty`), `late_penalty_percent` (0-100%), `allow_resubmission` (boolean), `max_attempts` (1-20), `reminder_hours` (1-720, default 48h), and `rubrics` (array of criteria with individual `max_points`).
- `POST /admin/submissions/grade` records submission review with status (`accepted` or `rejected`), numerical `score` or itemized `rubric_scores`, and written `feedback`. If `late_policy` is `penalty`, late submissions automatically apply the penalty deduction upon grading.
- Resubmissions archive previous attempts into `submission_revisions`. Students review revision history via `GET /user/submissions/:submission_id/revisions`.

## Learner dashboard

`GET /user/dashboard` provides a single consolidated view for students with:
- `summary`: total counts for upcoming, overdue, pending review, and graded items.
- `upcoming`: unsubmitted assignments due in the future.
- `overdue`: unsubmitted assignments past their due date.
- `pending_review`: submitted work awaiting instructor grading.
- `graded`: reviewed assignments with scores, maximum points, written instructor feedback, and status.
- `reminders`: approaching assignments due within the assignment's configured `reminder_hours` window.

## Upload storage

Production startup uses GCS and fails closed if ClamAV is unavailable for an upload. Configure `GCS_BUCKET_NAME`, ADC for the service identity, `GCS_ACCESS_ID`, `GCS_PRIVATE_KEY`, `CLAMAV_ADDRESS`, `UPLOAD_MAX_BYTES`, and `UPLOAD_RETENTION_DAYS`. The default upload limit is 10 MiB. Submission and image uploads accept only the extension/MIME combinations implemented in `services/file_service.go`. Downloads are issued through `GET /user/submissions/:submission_id/download?file=submission|image` and expire after five minutes.

Object metadata records scan status and retention days, but GCS does not expire objects from custom metadata alone. Configure a bucket lifecycle rule to delete `tenant/` objects after the required retention period; do not use a locked retention policy unless its compliance period is intentional, because locked policies cannot be shortened.

## Analytics reports

`POST /admin/aid/getData` accepts one approved identifier or alias rather than generated SQL:

- `assignments_by_branch`
- `submission_status`
- `overdue_assignments`

Each report is a fixed parameterized query scoped to the authenticated tenant, executes in a read-only transaction, has a five-second statement timeout, and returns at most 1,000 rows.

## Pagination and observability

`GET /admin/getassignments`, `GET /admin/submissions`, and `GET /user/assignments` accept `page` (default 1) and `limit` (default 25, maximum 100), returning `{ "data": [...], "page": { "page", "limit", "total", "total_pages" } }`.

`GET /metrics` exposes Prometheus text-format request counters. Requests emit OpenTelemetry spans through the configured OTLP HTTP exporter and write audit records to PostgreSQL `audit_logs`. Set `JAEGER_ENDPOINT` or `OTEL_EXPORTER_OTLP_ENDPOINT` to the collector base URL; the service posts to `/v1/traces`. The Assignment API instruments its RAG and email gRPC clients, and those services instrument their gRPC servers. Monitor audit-log retention/partitioning requirements. See [API docs](docs/README.md) and [database docs](docs/database.md).
