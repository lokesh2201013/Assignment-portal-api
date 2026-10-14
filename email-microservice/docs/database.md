# Email Service Database

On startup, `internal/storage/postgres.go` connects to the configured database and creates tables/indexes if missing. `init.sql` intentionally contains no DDL: the PostgreSQL image creates `POSTGRES_DB`, then the service initializes its tables. Do not add a duplicate `CREATE DATABASE` statement to the container init directory.

## Tables

| Table | Purpose | Key columns |
|---|---|---|
| `users` | Email-service accounts | serial ID, unique username, password hash, created/updated/deleted timestamps |
| `senders` | Per-admin SMTP sender credentials and verification state | admin name, unique email, SMTP host/port/user/password, verified flag |
| `templates` | Reusable email content | unique name, subject, body, `text`/`html` format |
| `analytics` | Aggregate delivery counts | admin name, sender foreign key, total/delivered/bounced/complaint/rejected counts |

`analytics.sender_id` references `senders(id)` with `ON DELETE CASCADE`. Indexes cover sender email/admin, analytics sender/admin, and usernames.

## Data handling

SMTP sender passwords are credentials and must be encrypted or protected with a managed secret store in production. The current schema stores the supplied value directly; restrict database access and plan a migration to encryption at rest before production. `analytics` is aggregate data; the service does not currently persist a per-message history table or delivery events from an external provider.

## Connection

The service requires `DB_HOST`, `DB_USER`, `DB_PASSWORD`, and `DB_NAME`; set `DB_PORT` and `DB_SSLMODE` to match the environment. Avoid `latest` database image tags in production and use persistent volumes plus tested backups.
