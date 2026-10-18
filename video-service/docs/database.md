# Video Service Database

The Video service connects to PostgreSQL using a threaded connection pool. It does not create or migrate tables on startup. Create the database and apply [`schema.sql`](../schema.sql) before starting the service.

## `videos` table

| Column | Type | Notes |
|---|---|---|
| `id` | `BIGSERIAL` | Primary key; current playback API accepts an integer ID |
| `url` | `TEXT` | Original or source URL; defaults to an empty string |
| `title` | `TEXT` | Required display title |
| `description` | `TEXT` | Description; defaults to an empty string |
| `status` | `TEXT` | Processing state; defaults to `queued` |
| `created_at` | `TIMESTAMPTZ` | Creation time |
| `updated_at` | `TIMESTAMPTZ` | Last metadata update |

The repository currently reads by `id` and updates only `status`. The separate Assignment API has its own tenant-scoped `videos` table using UUID IDs; these schemas are not interchangeable. This service's table has no tenant column today.

## `chat_messages` table

| Column | Type | Notes |
|---|---|---|
| `id` | `BIGSERIAL` | Primary key and history cursor |
| `user_id` | `TEXT` | Chat connection's user identifier; system messages use `system` |
| `username` | `TEXT` | Display name or system-message sender |
| `message` | `TEXT` | Required message, limited to 500 characters |
| `message_type` | `TEXT` | `chat_message` or `system_message` |
| `created_at` | `TIMESTAMPTZ` | Database-generated message time |

Chat messages are written to PostgreSQL before being sent to connected clients. `GET /api/messages` returns saved messages in ascending order and supports `limit` (1–100) and `before_id` pagination. Connected-user presence remains process-local; message history survives service restarts.

## Local setup

```sql
CREATE DATABASE videos;
```

Connect to that database and apply `schema.sql` to create both the video and chat tables. Production deployments should manage schema changes through a migration tool, keep backups, and add tenant ownership before serving data to multiple tenants.
