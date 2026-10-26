# Email Service Database

On startup, `internal/storage/postgres.go` connects to the configured database and creates tables/indexes if missing. `init.sql` intentionally contains no DDL: the PostgreSQL image creates `POSTGRES_DB`, then the service initializes its tables. Do not add a duplicate `CREATE DATABASE` statement to the container init directory.

## Tables

| Table | Purpose | Key columns |
|---|---|---|
| `users` | Email-service accounts | serial ID, unique username, password hash, created/updated/deleted timestamps |
| `senders` | Per-admin SMTP sender credentials and verification state | admin name, unique email, SMTP host/port/user/encrypted password, verified flag |
| `templates` | Reusable email content | unique name, subject, body, `text`/`html` format |
| `analytics` | Aggregate delivery counts | admin name, sender foreign key, total/delivered/bounced/complaint/rejected counts |
| `email_messages` | Durable per-recipient queue and message state | UUID, sender/recipient, content snapshot, status, internal-global-SMTP marker, attempt budget, next retry, latest error, delivery/failure timestamps |
| `email_delivery_events` | Immutable delivery-attempt history | message foreign key, attempt number, outcome (`delivered`, `retry_scheduled`, or `failed`), SMTP error, timestamp |

`analytics.sender_id` references `senders(id)` with `ON DELETE CASCADE`. `email_delivery_events.message_id` references `email_messages(id)` with `ON DELETE CASCADE`. Queue indexes support claiming due work, recipient search, chronological history, and message event lookup.

## Data handling

Sender SMTP passwords are encrypted with AES-256-GCM before storage. `SMTP_CREDENTIALS_KEY` is a required base64-encoded 32-byte key; protect it with a secret manager and backups. Startup encrypts existing plaintext sender passwords and verifies previously encrypted values against the configured key. If the key is lost or changed without re-encrypting the database, credentials cannot be decrypted. `analytics` is aggregate data; use `email_messages` and `email_delivery_events` to troubleshoot an individual notification and its SMTP failures.

## Connection

The service requires `DB_HOST`, `DB_USER`, `DB_PASSWORD`, and `DB_NAME`; set `DB_PORT` and `DB_SSLMODE` to match the environment. Avoid `latest` database image tags in production and use persistent volumes plus tested backups.
