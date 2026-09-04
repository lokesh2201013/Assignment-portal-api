// Package storage provides database connection and schema management.
package storage

import (
	"context"
	"fmt"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// DB is the global database connection pool.
var DB *sqlx.DB

// Connect establishes a connection to PostgreSQL and runs migrations.
func Connect(ctx context.Context, dsn string, logger *slog.Logger) (*sqlx.DB, error) {
	db, err := sqlx.ConnectContext(ctx, "pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Run migrations
	if err := createTables(ctx, db, logger); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	logger.Info("database connected successfully")
	DB = db
	return db, nil
}

// Close closes the database connection.
func Close(db *sqlx.DB) error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// createTables creates all necessary tables if they don't exist.
func createTables(ctx context.Context, db *sqlx.DB, logger *slog.Logger) error {
	statements := []string{
		createUsersTable,
		createSendersTable,
		createTemplatesTable,
		createAnalyticsTable,
		createIndexes,
	}

	for _, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			logger.Error("failed to create table", "error", err, "statement", stmt)
			return err
		}
	}

	logger.Info("database schema initialized")
	return nil
}

const (
	createUsersTable = `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(50) NOT NULL UNIQUE,
			password VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP
		);
	`

	createSendersTable = `
		CREATE TABLE IF NOT EXISTS senders (
			id SERIAL PRIMARY KEY,
			admin_name VARCHAR(50) NOT NULL,
			email VARCHAR(255) NOT NULL UNIQUE,
			smtp_host VARCHAR(255) NOT NULL,
			smtp_port INTEGER NOT NULL,
			username VARCHAR(255) NOT NULL,
			password VARCHAR(255) NOT NULL,
			verified BOOLEAN DEFAULT false,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`

	createTemplatesTable = `
		CREATE TABLE IF NOT EXISTS templates (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL UNIQUE,
			subject VARCHAR(255) NOT NULL,
			body TEXT NOT NULL,
			format VARCHAR(10) DEFAULT 'text',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`

	createAnalyticsTable = `
		CREATE TABLE IF NOT EXISTS analytics (
			id SERIAL PRIMARY KEY,
			admin_name VARCHAR(50) NOT NULL,
			sender_id INTEGER NOT NULL REFERENCES senders(id) ON DELETE CASCADE,
			total_emails INTEGER DEFAULT 0,
			accumulated_email INTEGER DEFAULT 0,
			delivered INTEGER DEFAULT 0,
			bounced INTEGER DEFAULT 0,
			complaints INTEGER DEFAULT 0,
			rejected INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`

	createIndexes = `
		CREATE INDEX IF NOT EXISTS idx_senders_email ON senders(email);
		CREATE INDEX IF NOT EXISTS idx_senders_admin_name ON senders(admin_name);
		CREATE INDEX IF NOT EXISTS idx_analytics_sender_id ON analytics(sender_id);
		CREATE INDEX IF NOT EXISTS idx_analytics_admin_name ON analytics(admin_name);
		CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	`
)
