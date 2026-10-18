# Assignment API Database

The service connects to PostgreSQL and runs idempotent `CREATE TABLE IF NOT EXISTS`, `ALTER TABLE`, index, and seed statements from `database/postgres.go` on every startup. This is an application bootstrap migration, not a versioned migration system. Back up the database before deployment and do not remove columns or data without a reviewed migration.

## Tables

| Table | Purpose | Tenant key / important fields |
|---|---|---|
| `tenants` | Tenant registry | `tenant_id` primary key, `name`, `status`, `created_at` |
| `users` | Login identities and profile | UUID `user_id`, `tenant_id`, `name`, `email`, password hash, role, branch, semester; unique `(tenant_id, email)` |
| `tenant_memberships` | Tenant-specific role and activation | `(tenant_id, user_id)` primary key; role `admin`/`user`; status `pending`/`verified`/`revoked` |
| `tenant_invitations` | Single-use invitations | UUID id, tenant, email, role, hashed token, inviter, expiry, accepted timestamp, branch, semester, display_name, course_id |
| `courses` | Course and class registry | UUID `course_id`, `tenant_id`, `code`, `name`, `branch`, `semester`, `created_by`, `created_at`; unique `(tenant_id, code)` |
| `course_enrollments` | Student course registrations | `(tenant_id, course_id, user_id)` primary key |
| `assignments` | Work assigned to student groups | UUID id, tenant, admin, task, due date, branch, semester, subject code, `course_id`, `max_score`, `late_policy`, `late_penalty_percent`, `allow_resubmission`, `max_attempts`, `reminder_hours` |
| `assignment_rubrics` | Grading criteria for assignments | UUID `rubric_id`, `tenant_id`, `assignment_id`, `criterion`, `description`, `max_points`, `sort_order` |
| `submit_assignments` | Student submissions and review status | UUID submission id, tenant, assignment/user IDs, status, file/image references, comments, late flag, `score`, `feedback`, `attempt_number`, `graded_at`, `graded_by` |
| `submission_revisions` | Versioned history of student attempts | UUID `revision_id`, `tenant_id`, `submission_id`, `assignment_id`, `user_id`, `attempt_number`, `status`, file/image refs, comments, feedback, score, late flag, timestamps |
| `rubric_scores` | Itemized scores awarded per rubric | `(tenant_id, submission_id, rubric_id)` primary key, `points` |
| `videos` | Video metadata for the portal | UUID id, tenant, URL, title, JSONB tags, status, description |
| `audit_logs` | Request audit trail | tenant, actor, action, route/resource, request ID, status, occurrence time |

Tenant-aware indexes support assignment filters, submission review lists, video lookups, course lookups, revision lookups, and audit chronology. Legacy timestamp columns in assignment/submission tables are text; do not assume they are PostgreSQL timestamps in custom queries.

## Relationships and operational notes

- `tenant_memberships` references `tenants` and `users`; invitations reference tenants and optionally the inviting user.
- Assignment/submission UUID fields are not declared as foreign keys in the current bootstrap schema. Application services enforce the relevant ownership and tenant checks.
- The `default` tenant is inserted automatically, and legacy users are backfilled into memberships during startup.
- `audit_logs` may contain identifiers and request metadata. Establish retention/partitioning and backups before production use.
- Passwords and invitation tokens must remain hashed at rest. Do not include database credentials or raw invitation tokens in logs or documentation.

## Connection

Use `DATABASE_DSN` (PostgreSQL URL or pgx DSN). Otherwise set `DB_HOST`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_PORT`, and `DB_SSLMODE`. The default SSL mode is `disable`, which is suitable only for local development.
