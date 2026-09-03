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
		`CREATE TABLE IF NOT EXISTS users (user_id UUID PRIMARY KEY, name TEXT NOT NULL, email TEXT NOT NULL UNIQUE, password TEXT NOT NULL, role TEXT NOT NULL DEFAULT 'user', branch TEXT, semester INTEGER)`,
		`CREATE TABLE IF NOT EXISTS assignments (assignment_id UUID PRIMARY KEY, email TEXT NOT NULL, admin_id UUID NOT NULL, task TEXT NOT NULL, created_at TEXT, updated_at TEXT, due_date TEXT, branch TEXT NOT NULL, semester INTEGER NOT NULL, subject_code TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS submit_assignments (submission_id UUID PRIMARY KEY, assignment_id UUID NOT NULL, user_id UUID NOT NULL, status TEXT NOT NULL DEFAULT 'pending', file TEXT, image TEXT, comments TEXT, late_submission BOOLEAN, created_at TEXT)`,
		`CREATE TABLE IF NOT EXISTS videos (id UUID PRIMARY KEY, url TEXT, title TEXT NOT NULL, tags JSONB, status TEXT, description TEXT)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}
