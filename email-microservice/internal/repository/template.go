package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/lokesh2201013/email-service/internal/domain"
	"github.com/lokesh2201013/email-service/internal/errors/apperror"
)

// PostgresTemplateRepository implements TemplateRepository using PostgreSQL.
type PostgresTemplateRepository struct {
	db *sqlx.DB
}

// NewPostgresTemplateRepository creates a new template repository.
func NewPostgresTemplateRepository(db *sqlx.DB) TemplateRepository {
	return &PostgresTemplateRepository{db: db}
}

// Create inserts a new template into the database.
func (r *PostgresTemplateRepository) Create(ctx context.Context, template *domain.Template) error {
	const query = `
		INSERT INTO templates (name, subject, body, format, created_at, updated_at)
		VALUES (:name, :subject, :body, :format, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id
	`

	rows, err := r.db.NamedQueryContext(ctx, query, template)
	if err != nil {
		if isUniqueConstraintViolation(err) {
			return apperror.Conflict("template name already exists")
		}
		return apperror.Database("failed to create template", err)
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&template.ID); err != nil {
			return apperror.Database("failed to retrieve created template ID", err)
		}
	}
	return nil
}

// GetByID retrieves a template by ID.
func (r *PostgresTemplateRepository) GetByID(ctx context.Context, templateID uint) (*domain.Template, error) {
	const query = `
		SELECT id, name, subject, body, format, created_at, updated_at
		FROM templates
		WHERE id = $1
	`

	template := &domain.Template{}
	if err := r.db.GetContext(ctx, template, query, templateID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.NotFound("template not found")
		}
		return nil, apperror.Database("failed to get template", err)
	}
	return template, nil
}

// GetByName retrieves a template by name.
func (r *PostgresTemplateRepository) GetByName(ctx context.Context, name string) (*domain.Template, error) {
	const query = `
		SELECT id, name, subject, body, format, created_at, updated_at
		FROM templates
		WHERE name = $1
	`

	template := &domain.Template{}
	if err := r.db.GetContext(ctx, template, query, name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.NotFound("template not found")
		}
		return nil, apperror.Database("failed to get template", err)
	}
	return template, nil
}

// List retrieves all templates.
func (r *PostgresTemplateRepository) List(ctx context.Context) ([]domain.Template, error) {
	const query = `
		SELECT id, name, subject, body, format, created_at, updated_at
		FROM templates
		ORDER BY created_at DESC
	`

	var templates []domain.Template
	if err := r.db.SelectContext(ctx, &templates, query); err != nil {
		return nil, apperror.Database("failed to list templates", err)
	}
	return templates, nil
}

// Update updates an existing template.
func (r *PostgresTemplateRepository) Update(ctx context.Context, template *domain.Template) error {
	const query = `
		UPDATE templates
		SET name = :name, subject = :subject, body = :body, format = :format, updated_at = CURRENT_TIMESTAMP
		WHERE id = :id
	`

	result, err := r.db.NamedExecContext(ctx, query, template)
	if err != nil {
		return apperror.Database("failed to update template", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return apperror.Database("failed to get affected rows", err)
	}
	if rows == 0 {
		return apperror.NotFound("template not found")
	}
	return nil
}

// Delete removes a template.
func (r *PostgresTemplateRepository) Delete(ctx context.Context, templateID uint) error {
	const query = `
		DELETE FROM templates
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, templateID)
	if err != nil {
		return apperror.Database("failed to delete template", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return apperror.Database("failed to get affected rows", err)
	}
	if rows == 0 {
		return apperror.NotFound("template not found")
	}
	return nil
}
