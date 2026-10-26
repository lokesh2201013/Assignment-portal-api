# Assignment API database notes

`database.ConnectDB` loads `.env` when present and opens PostgreSQL using
`DB_HOST`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, and `DB_PORT`. TLS is currently
hard-coded as `sslmode=disable`. PostgreSQL connection and the GORM
`AutoMigrate` calls for `users`, `assignments`, `submit_assignments`, and
`videos` are performed during startup.

The assignment and tenant repositories use `sqlx` against that same connection
pool. Their SQL expects additional tables and columns (including tenant
membership/invitation, course, rubric, revision, and grading data). Those
tables are **not** created by the current `AutoMigrate` call, and no versioned
migration is checked in here. Apply a compatible schema before using those
service-backed endpoints; otherwise their requests will fail at the database.

`DATABASE_DSN`, migration commands, GCS upload storage, audit logging, and
database retention automation are not implemented by this module.
