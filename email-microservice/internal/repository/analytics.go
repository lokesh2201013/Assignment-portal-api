package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/lokesh2201013/email-service/internal/domain"
	"github.com/lokesh2201013/email-service/internal/errors/apperror"
)

// PostgresAnalyticsRepository implements AnalyticsRepository using PostgreSQL.
type PostgresAnalyticsRepository struct {
	db *sqlx.DB
}

// NewPostgresAnalyticsRepository creates a new analytics repository.
func NewPostgresAnalyticsRepository(db *sqlx.DB) AnalyticsRepository {
	return &PostgresAnalyticsRepository{db: db}
}

// Create inserts a new analytics record into the database.
func (r *PostgresAnalyticsRepository) Create(ctx context.Context, analytics *domain.Analytics) error {
	const query = `
		INSERT INTO analytics (admin_name, sender_id, total_emails, accumulated_email, delivered, bounced, complaints, rejected, created_at, updated_at)
		VALUES (:admin_name, :sender_id, :total_emails, :accumulated_email, :delivered, :bounced, :complaints, :rejected, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id
	`

	rows, err := r.db.NamedQueryContext(ctx, query, analytics)
	if err != nil {
		return apperror.Database("failed to create analytics record", err)
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&analytics.ID); err != nil {
			return apperror.Database("failed to retrieve created analytics ID", err)
		}
	}
	return nil
}

// GetBySenderID retrieves analytics for a specific sender.
func (r *PostgresAnalyticsRepository) GetBySenderID(ctx context.Context, senderID uint) (*domain.Analytics, error) {
	const query = `
		SELECT id, admin_name, sender_id, total_emails, accumulated_email, delivered, bounced, complaints, rejected, created_at, updated_at
		FROM analytics
		WHERE sender_id = $1
	`

	analytics := &domain.Analytics{}
	if err := r.db.GetContext(ctx, analytics, query, senderID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.NotFound("analytics not found for sender")
		}
		return nil, apperror.Database("failed to get analytics", err)
	}
	return analytics, nil
}

// GetByAdminName retrieves analytics for a specific admin.
func (r *PostgresAnalyticsRepository) GetByAdminName(ctx context.Context, adminName string) ([]domain.Analytics, error) {
	const query = `
		SELECT id, admin_name, sender_id, total_emails, accumulated_email, delivered, bounced, complaints, rejected, created_at, updated_at
		FROM analytics
		WHERE admin_name = $1
		ORDER BY created_at DESC
	`

	var analyticsRecords []domain.Analytics
	if err := r.db.SelectContext(ctx, &analyticsRecords, query, adminName); err != nil {
		return nil, apperror.Database("failed to get analytics by admin", err)
	}
	return analyticsRecords, nil
}

// Update updates an analytics record by ID.
func (r *PostgresAnalyticsRepository) Update(ctx context.Context, analytics *domain.Analytics) error {
	const query = `
		UPDATE analytics
		SET admin_name = :admin_name, sender_id = :sender_id, total_emails = :total_emails, 
		    accumulated_email = :accumulated_email, delivered = :delivered, bounced = :bounced, 
		    complaints = :complaints, rejected = :rejected, updated_at = CURRENT_TIMESTAMP
		WHERE id = :id
	`

	result, err := r.db.NamedExecContext(ctx, query, analytics)
	if err != nil {
		return apperror.Database("failed to update analytics", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return apperror.Database("failed to get affected rows", err)
	}
	if rows == 0 {
		return apperror.NotFound("analytics record not found")
	}
	return nil
}

// UpdateBySenderID updates an analytics record by sender ID.
func (r *PostgresAnalyticsRepository) UpdateBySenderID(ctx context.Context, senderID uint, analytics *domain.Analytics) error {
	const query = `
		UPDATE analytics
		SET total_emails = :total_emails, accumulated_email = :accumulated_email, 
		    delivered = :delivered, bounced = :bounced, complaints = :complaints, 
		    rejected = :rejected, updated_at = CURRENT_TIMESTAMP
		WHERE sender_id = :sender_id
	`

	// Bind sender_id to the analytics struct for the named query
	params := map[string]interface{}{
		"sender_id":         senderID,
		"total_emails":      analytics.TotalEmails,
		"accumulated_email": analytics.AccumulatedEmail,
		"delivered":         analytics.Delivered,
		"bounced":           analytics.Bounced,
		"complaints":        analytics.Complaints,
		"rejected":          analytics.Rejected,
	}

	result, err := r.db.NamedExecContext(ctx, query, params)
	if err != nil {
		return apperror.Database("failed to update analytics by sender", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return apperror.Database("failed to get affected rows", err)
	}
	if rows == 0 {
		return apperror.NotFound("analytics record not found for sender")
	}
	return nil
}

// ListByAdminName retrieves all analytics for a specific admin.
func (r *PostgresAnalyticsRepository) ListByAdminName(ctx context.Context, adminName string) ([]domain.Analytics, error) {
	const query = `
		SELECT id, admin_name, sender_id, total_emails, accumulated_email, delivered, bounced, complaints, rejected, created_at, updated_at
		FROM analytics
		WHERE admin_name = $1
		ORDER BY updated_at DESC
	`

	var analyticsRecords []domain.Analytics
	if err := r.db.SelectContext(ctx, &analyticsRecords, query, adminName); err != nil {
		return nil, apperror.Database("failed to list analytics", err)
	}
	return analyticsRecords, nil
}
