package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/lokesh2201013/email-service/internal/domain"
	"github.com/lokesh2201013/email-service/internal/errors/apperror"
)

// PostgresUserRepository implements UserRepository using PostgreSQL.
type PostgresUserRepository struct {
	db *sqlx.DB
}

// NewPostgresUserRepository creates a new user repository.
func NewPostgresUserRepository(db *sqlx.DB) UserRepository {
	return &PostgresUserRepository{db: db}
}

// Create inserts a new user into the database.
func (r *PostgresUserRepository) Create(ctx context.Context, user *domain.User) error {
	const query = `
		INSERT INTO users (username, password, created_at, updated_at)
		VALUES (:username, :password, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id
	`

	rows, err := r.db.NamedQueryContext(ctx, query, user)
	if err != nil {
		if isUniqueConstraintViolation(err) {
			return apperror.Conflict("username already exists")
		}
		return apperror.Database("failed to create user", err)
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&user.ID); err != nil {
			return apperror.Database("failed to retrieve created user ID", err)
		}
	}
	return nil
}

// GetByUsername retrieves a user by username.
func (r *PostgresUserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	const query = `
		SELECT id, username, password, created_at, updated_at, deleted_at
		FROM users
		WHERE username = $1 AND deleted_at IS NULL
	`

	user := &domain.User{}
	if err := r.db.GetContext(ctx, user, query, username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.NotFound("user not found")
		}
		return nil, apperror.Database("failed to get user", err)
	}
	return user, nil
}

// Update updates an existing user.
func (r *PostgresUserRepository) Update(ctx context.Context, user *domain.User) error {
	const query = `
		UPDATE users
		SET password = :password, updated_at = CURRENT_TIMESTAMP
		WHERE id = :id
	`

	result, err := r.db.NamedExecContext(ctx, query, user)
	if err != nil {
		return apperror.Database("failed to update user", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return apperror.Database("failed to get affected rows", err)
	}
	if rows == 0 {
		return apperror.NotFound("user not found")
	}
	return nil
}

// Delete performs a soft delete on a user.
func (r *PostgresUserRepository) Delete(ctx context.Context, userID uint) error {
	const query = `
		UPDATE users
		SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return apperror.Database("failed to delete user", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return apperror.Database("failed to get affected rows", err)
	}
	if rows == 0 {
		return apperror.NotFound("user not found")
	}
	return nil
}

func isUniqueConstraintViolation(err error) bool {
	return err != nil && errors.Is(err, sql.ErrNoRows) == false
}
