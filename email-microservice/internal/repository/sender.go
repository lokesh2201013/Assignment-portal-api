package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/lokesh2201013/email-service/internal/credentials"
	"github.com/lokesh2201013/email-service/internal/domain"
	"github.com/lokesh2201013/email-service/internal/errors/apperror"
)

// PostgresSenderRepository implements SenderRepository using PostgreSQL.
type PostgresSenderRepository struct {
	db  *sqlx.DB
	key []byte
}

// NewPostgresSenderRepository creates a new sender repository.
func NewPostgresSenderRepository(db *sqlx.DB, key []byte) *PostgresSenderRepository {
	return &PostgresSenderRepository{db: db, key: key}
}

// MigrateCredentials encrypts existing plaintext SMTP app passwords and
// verifies that existing ciphertext can be opened with the configured key.
func (r *PostgresSenderRepository) MigrateCredentials(ctx context.Context) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return apperror.Database("failed to begin SMTP credential migration", err)
	}
	defer tx.Rollback()

	type storedCredential struct {
		ID       uint   `db:"id"`
		Password string `db:"password"`
	}
	var stored []storedCredential
	if err := tx.SelectContext(ctx, &stored, `SELECT id, password FROM senders FOR UPDATE`); err != nil {
		return apperror.Database("failed to read SMTP credentials for migration", err)
	}

	for _, item := range stored {
		if credentials.IsEncrypted(item.Password) {
			if _, err := credentials.Decrypt(r.key, item.Password); err != nil {
				return apperror.Database("SMTP credential encryption key is invalid or stored credential is corrupt", err)
			}
			continue
		}
		encrypted, err := credentials.Encrypt(r.key, item.Password)
		if err != nil {
			return apperror.Internal("failed to encrypt stored SMTP credential", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE senders SET password = $1 WHERE id = $2`, encrypted, item.ID); err != nil {
			return apperror.Database("failed to encrypt stored SMTP credential", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return apperror.Database("failed to commit SMTP credential migration", err)
	}
	return nil
}

// Create inserts a new sender into the database.
func (r *PostgresSenderRepository) Create(ctx context.Context, sender *domain.Sender) error {
	const query = `
		INSERT INTO senders (admin_name, email, smtp_host, smtp_port, username, password, verified, created_at, updated_at)
		VALUES (:admin_name, :email, :smtp_host, :smtp_port, :username, :password, :verified, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id
	`

	storedSender := *sender
	encryptedPassword, err := credentials.Encrypt(r.key, sender.Password)
	if err != nil {
		return apperror.Internal("failed to encrypt SMTP credential", err)
	}
	storedSender.Password = encryptedPassword
	rows, err := r.db.NamedQueryContext(ctx, query, &storedSender)
	if err != nil {
		if isUniqueConstraintViolation(err) {
			return apperror.Conflict("email already exists")
		}
		return apperror.Database("failed to create sender", err)
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&sender.ID); err != nil {
			return apperror.Database("failed to retrieve created sender ID", err)
		}
	}
	return nil
}

// GetByEmail retrieves a sender by email address.
func (r *PostgresSenderRepository) GetByEmail(ctx context.Context, email string) (*domain.Sender, error) {
	const query = `
		SELECT id, admin_name, email, smtp_host, smtp_port, username, password, verified, created_at, updated_at
		FROM senders
		WHERE email = $1
	`

	sender := &domain.Sender{}
	if err := r.db.GetContext(ctx, sender, query, email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.NotFound("sender not found")
		}
		return nil, apperror.Database("failed to get sender", err)
	}
	if err := r.decryptPassword(sender); err != nil {
		return nil, apperror.Database("failed to decrypt SMTP credential", err)
	}
	return sender, nil
}

// GetByID retrieves a sender by ID.
func (r *PostgresSenderRepository) GetByID(ctx context.Context, senderID uint) (*domain.Sender, error) {
	const query = `
		SELECT id, admin_name, email, smtp_host, smtp_port, username, password, verified, created_at, updated_at
		FROM senders
		WHERE id = $1
	`

	sender := &domain.Sender{}
	if err := r.db.GetContext(ctx, sender, query, senderID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.NotFound("sender not found")
		}
		return nil, apperror.Database("failed to get sender", err)
	}
	if err := r.decryptPassword(sender); err != nil {
		return nil, apperror.Database("failed to decrypt SMTP credential", err)
	}
	return sender, nil
}

// ListByAdminAndVerified retrieves all senders for an admin with a specific verification status.
func (r *PostgresSenderRepository) ListByAdminAndVerified(ctx context.Context, adminName string, verified bool) ([]domain.Sender, error) {
	const query = `
		SELECT id, admin_name, email, smtp_host, smtp_port, username, password, verified, created_at, updated_at
		FROM senders
		WHERE admin_name = $1 AND verified = $2
		ORDER BY created_at DESC
	`

	var senders []domain.Sender
	if err := r.db.SelectContext(ctx, &senders, query, adminName, verified); err != nil {
		return nil, apperror.Database("failed to list senders", err)
	}
	for i := range senders {
		if err := r.decryptPassword(&senders[i]); err != nil {
			return nil, apperror.Database("failed to decrypt SMTP credential", err)
		}
	}
	return senders, nil
}

// Update updates an existing sender.
func (r *PostgresSenderRepository) Update(ctx context.Context, sender *domain.Sender) error {
	const query = `
		UPDATE senders
		SET admin_name = :admin_name, email = :email, smtp_host = :smtp_host, 
		    smtp_port = :smtp_port, username = :username, password = :password, 
		    verified = :verified, updated_at = CURRENT_TIMESTAMP
		WHERE id = :id
	`

	storedSender := *sender
	encryptedPassword, err := credentials.Encrypt(r.key, sender.Password)
	if err != nil {
		return apperror.Internal("failed to encrypt SMTP credential", err)
	}
	storedSender.Password = encryptedPassword
	result, err := r.db.NamedExecContext(ctx, query, &storedSender)
	if err != nil {
		return apperror.Database("failed to update sender", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return apperror.Database("failed to get affected rows", err)
	}
	if rows == 0 {
		return apperror.NotFound("sender not found")
	}
	return nil
}

// Delete removes a sender by email.
func (r *PostgresSenderRepository) Delete(ctx context.Context, email string) error {
	// First delete associated analytics
	const deleteAnalyticsQuery = `
		DELETE FROM analytics
		WHERE sender_id = (SELECT id FROM senders WHERE email = $1)
	`
	if _, err := r.db.ExecContext(ctx, deleteAnalyticsQuery, email); err != nil {
		return apperror.Database("failed to delete associated analytics", err)
	}

	// Then delete sender
	const deleteSenderQuery = `
		DELETE FROM senders
		WHERE email = $1
	`
	result, err := r.db.ExecContext(ctx, deleteSenderQuery, email)
	if err != nil {
		return apperror.Database("failed to delete sender", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return apperror.Database("failed to get affected rows", err)
	}
	if rows == 0 {
		return apperror.NotFound("sender not found")
	}
	return nil
}

// Exists checks if a sender exists by email.
func (r *PostgresSenderRepository) Exists(ctx context.Context, email string) (bool, error) {
	const query = `
		SELECT EXISTS(SELECT 1 FROM senders WHERE email = $1)
	`

	var exists bool
	if err := r.db.GetContext(ctx, &exists, query, email); err != nil {
		return false, apperror.Database("failed to check sender existence", err)
	}
	return exists, nil
}

func (r *PostgresSenderRepository) decryptPassword(sender *domain.Sender) error {
	password, err := credentials.Decrypt(r.key, sender.Password)
	if err != nil {
		return err
	}
	sender.Password = password
	return nil
}
