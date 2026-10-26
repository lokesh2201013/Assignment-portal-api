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
		createEmailMessagesTable,
		addGlobalSMTPSupport,
		createEmailDeliveryEventsTable,
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

	createEmailMessagesTable = `
		CREATE TABLE IF NOT EXISTS email_messages (
			id VARCHAR(36) PRIMARY KEY,
			sender_email VARCHAR(255) NOT NULL,
			recipient VARCHAR(255) NOT NULL,
			subject VARCHAR(255) NOT NULL,
			body TEXT NOT NULL,
			format VARCHAR(10) DEFAULT 'text',
			status VARCHAR(20) DEFAULT 'queued',
			use_global_smtp BOOLEAN NOT NULL DEFAULT false,
			attempts INTEGER DEFAULT 0,
			max_attempts INTEGER DEFAULT 3,
			last_error TEXT DEFAULT '',
			next_retry_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			delivered_at TIMESTAMP WITH TIME ZONE,
			failed_at TIMESTAMP WITH TIME ZONE
		);
	`

	createEmailDeliveryEventsTable = `
		CREATE TABLE IF NOT EXISTS email_delivery_events (
			id BIGSERIAL PRIMARY KEY,
			message_id VARCHAR(36) NOT NULL REFERENCES email_messages(id) ON DELETE CASCADE,
			attempt INTEGER NOT NULL,
			status VARCHAR(30) NOT NULL,
			error TEXT NOT NULL DEFAULT '',
			occurred_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`

	addGlobalSMTPSupport = `
		ALTER TABLE email_messages
		ADD COLUMN IF NOT EXISTS use_global_smtp BOOLEAN NOT NULL DEFAULT false;
	`

	createIndexes = `
		CREATE INDEX IF NOT EXISTS idx_senders_email ON senders(email);
		CREATE INDEX IF NOT EXISTS idx_senders_admin_name ON senders(admin_name);
		CREATE INDEX IF NOT EXISTS idx_analytics_sender_id ON analytics(sender_id);
		CREATE INDEX IF NOT EXISTS idx_analytics_admin_name ON analytics(admin_name);
		CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
		CREATE INDEX IF NOT EXISTS idx_email_messages_status_retry ON email_messages(status, next_retry_at);
		CREATE INDEX IF NOT EXISTS idx_email_messages_recipient ON email_messages(recipient);
		CREATE INDEX IF NOT EXISTS idx_email_messages_created_at ON email_messages(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_email_delivery_events_message_id ON email_delivery_events(message_id, occurred_at);
	`
)
