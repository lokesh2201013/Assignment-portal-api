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

## Local setup

```sql
CREATE DATABASE videos;
```

Connect to that database and apply `schema.sql`. Production deployments should manage schema changes through a migration tool, keep backups, and add tenant ownership before serving data to multiple tenants.
