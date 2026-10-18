package database

import (
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/lokesh2201013/config"
)

var DB *sqlx.DB

func ConnectDB(cfg config.Config, logger *slog.Logger) (*sqlx.DB, error) {
	db, err := sqlx.Open("pgx", cfg.DatabaseDSN)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	if err := createTables(db); err != nil {
		db.Close()
		return nil, err
	}

	DB = db
	logger.Info("connected to PostgreSQL")
	return db, nil
}

func createTables(db *sqlx.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS tenants (tenant_id TEXT PRIMARY KEY, name TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE IF NOT EXISTS users (user_id UUID PRIMARY KEY, tenant_id TEXT NOT NULL DEFAULT 'default', name TEXT NOT NULL, email TEXT NOT NULL, password TEXT NOT NULL, role TEXT NOT NULL DEFAULT 'user', branch TEXT, semester INTEGER)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default'`,
		`ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key`,
		`CREATE UNIQUE INDEX IF NOT EXISTS users_tenant_email_key ON users (tenant_id, email)`,
		`CREATE TABLE IF NOT EXISTS tenant_memberships (tenant_id TEXT NOT NULL REFERENCES tenants(tenant_id), user_id UUID NOT NULL REFERENCES users(user_id), role TEXT NOT NULL CHECK (role IN ('admin', 'user')), status TEXT NOT NULL CHECK (status IN ('pending', 'verified', 'revoked')), created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), PRIMARY KEY (tenant_id, user_id))`,
		`INSERT INTO tenants (tenant_id, name) VALUES ('default', 'Default') ON CONFLICT (tenant_id) DO NOTHING`,
		`INSERT INTO tenants (tenant_id, name) SELECT DISTINCT tenant_id, tenant_id FROM users ON CONFLICT (tenant_id) DO NOTHING`,
		`INSERT INTO tenant_memberships (tenant_id, user_id, role, status) SELECT tenant_id, user_id, role, 'verified' FROM users ON CONFLICT (tenant_id, user_id) DO NOTHING`,
		`CREATE TABLE IF NOT EXISTS tenant_invitations (invitation_id UUID PRIMARY KEY, tenant_id TEXT NOT NULL REFERENCES tenants(tenant_id), email TEXT NOT NULL, role TEXT NOT NULL CHECK (role IN ('admin', 'user')), token_hash TEXT NOT NULL UNIQUE, invited_by UUID REFERENCES users(user_id), expires_at TIMESTAMPTZ NOT NULL, accepted_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE INDEX IF NOT EXISTS tenant_invitations_email_idx ON tenant_invitations (tenant_id, email, expires_at)`,
		`CREATE TABLE IF NOT EXISTS assignments (assignment_id UUID PRIMARY KEY, tenant_id TEXT NOT NULL DEFAULT 'default', email TEXT NOT NULL, admin_id UUID NOT NULL, task TEXT NOT NULL, created_at TEXT, updated_at TEXT, due_date TEXT, branch TEXT NOT NULL, semester INTEGER NOT NULL, subject_code TEXT NOT NULL)`,
		`ALTER TABLE assignments ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default'`,
		`CREATE INDEX IF NOT EXISTS assignments_tenant_filters_idx ON assignments (tenant_id, branch, semester, subject_code)`,
		`CREATE TABLE IF NOT EXISTS submit_assignments (submission_id UUID PRIMARY KEY, tenant_id TEXT NOT NULL DEFAULT 'default', assignment_id UUID NOT NULL, user_id UUID NOT NULL, status TEXT NOT NULL DEFAULT 'pending', file TEXT, image TEXT, comments TEXT, late_submission BOOLEAN, created_at TEXT)`,
		`ALTER TABLE submit_assignments ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default'`,
		`CREATE INDEX IF NOT EXISTS submissions_tenant_assignment_status_idx ON submit_assignments (tenant_id, assignment_id, status)`,
		`CREATE TABLE IF NOT EXISTS videos (id UUID PRIMARY KEY, tenant_id TEXT NOT NULL DEFAULT 'default', url TEXT, title TEXT NOT NULL, tags JSONB, status TEXT, description TEXT)`,
		`ALTER TABLE videos ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default'`,
		`CREATE INDEX IF NOT EXISTS videos_tenant_idx ON videos (tenant_id)`,
		`CREATE TABLE IF NOT EXISTS audit_logs (audit_id BIGSERIAL PRIMARY KEY, tenant_id TEXT NOT NULL, actor_id UUID, action TEXT NOT NULL, resource TEXT NOT NULL, request_id TEXT NOT NULL, status_code INTEGER NOT NULL, occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE INDEX IF NOT EXISTS audit_logs_tenant_time_idx ON audit_logs (tenant_id, occurred_at DESC)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}
