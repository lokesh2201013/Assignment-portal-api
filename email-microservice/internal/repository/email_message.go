package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lokesh2201013/email-service/internal/domain"
	"github.com/lokesh2201013/email-service/internal/dto"
	"github.com/lokesh2201013/email-service/internal/errors/apperror"
)

// PostgresEmailMessageRepository implements EmailMessageRepository using PostgreSQL.
type PostgresEmailMessageRepository struct {
	db *sqlx.DB
}

// NewPostgresEmailMessageRepository creates a new email message repository.
func NewPostgresEmailMessageRepository(db *sqlx.DB) EmailMessageRepository {
	return &PostgresEmailMessageRepository{db: db}
}

// Enqueue inserts new email messages into the queue.
func (r *PostgresEmailMessageRepository) Enqueue(ctx context.Context, messages []*domain.EmailMessage) error {
	if len(messages) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return apperror.Database("failed to begin transaction", err)
	}
	defer tx.Rollback()

	const query = `
		INSERT INTO email_messages (
			id, sender_email, recipient, subject, body, format, status, use_global_smtp,
			attempts, max_attempts, last_error, next_retry_at, created_at, updated_at
		) VALUES (
			:id, :sender_email, :recipient, :subject, :body, :format, :status, :use_global_smtp,
			:attempts, :max_attempts, :last_error, :next_retry_at, :created_at, :updated_at
		)
	`

	for _, msg := range messages {
		if _, err := tx.NamedExecContext(ctx, query, msg); err != nil {
			return apperror.Database("failed to enqueue email message", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return apperror.Database("failed to commit email messages", err)
	}
	return nil
}

// GetPending claims up to limit pending emails and transitions them to 'processing' atomically.
func (r *PostgresEmailMessageRepository) GetPending(ctx context.Context, limit int) ([]*domain.EmailMessage, error) {
	if limit <= 0 {
		limit = 10
	}

	// A process can terminate after claiming a job and before persisting its
	// result. Reclaiming old processing jobs makes the database queue durable
	// across worker restarts without relying on in-memory state.
	const query = `
		UPDATE email_messages
		SET status = 'processing', updated_at = CURRENT_TIMESTAMP
		WHERE id IN (
			SELECT id
			FROM email_messages
			WHERE (status = 'queued' AND next_retry_at <= CURRENT_TIMESTAMP)
			   OR (status = 'processing' AND updated_at <= CURRENT_TIMESTAMP - INTERVAL '5 minutes')
			ORDER BY created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, sender_email, recipient, subject, body, format, status, use_global_smtp,
		          attempts, max_attempts, last_error, next_retry_at, created_at, updated_at,
		          delivered_at, failed_at
	`

	var messages []*domain.EmailMessage
	if err := r.db.SelectContext(ctx, &messages, query, limit); err != nil {
		return nil, apperror.Database("failed to fetch pending emails", err)
	}

	if messages == nil {
		messages = []*domain.EmailMessage{}
	}
	return messages, nil
}

// MarkProcessing marks a specific message as processing.
func (r *PostgresEmailMessageRepository) MarkProcessing(ctx context.Context, id string) error {
	const query = `UPDATE email_messages SET status = 'processing', updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return apperror.Database("failed to mark email as processing", err)
	}
	return nil
}

// MarkDelivered marks an email as successfully delivered.
func (r *PostgresEmailMessageRepository) MarkDelivered(ctx context.Context, id string) error {
	const query = `
		UPDATE email_messages
		SET status = 'delivered',
		    attempts = attempts + 1,
		    delivered_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP,
		    last_error = ''
		WHERE id = $1
	`
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return apperror.Database("failed to begin delivery transaction", err)
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, query, id)
	if err != nil {
		return apperror.Database("failed to mark email as delivered", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return apperror.NotFound("email message not found")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO email_delivery_events (message_id, attempt, status) SELECT id, attempts, 'delivered' FROM email_messages WHERE id = $1`, id); err != nil {
		return apperror.Database("failed to record delivery event", err)
	}
	if err := tx.Commit(); err != nil {
		return apperror.Database("failed to commit delivery transaction", err)
	}
	return nil
}

// MarkFailed records a failure. If nextRetryAt is provided, the message is requeued.
func (r *PostgresEmailMessageRepository) MarkFailed(ctx context.Context, id string, errMsg string, nextRetryAt *time.Time) error {
	var query string
	var args []interface{}

	if nextRetryAt != nil {
		query = `
			UPDATE email_messages
			SET status = 'queued',
			    attempts = attempts + 1,
			    last_error = $2,
			    next_retry_at = $3,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = $1
		`
		args = []interface{}{id, errMsg, *nextRetryAt}
	} else {
		query = `
			UPDATE email_messages
			SET status = 'failed',
			    attempts = attempts + 1,
			    last_error = $2,
			    failed_at = CURRENT_TIMESTAMP,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = $1
		`
		args = []interface{}{id, errMsg}
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return apperror.Database("failed to begin failure transaction", err)
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return apperror.Database("failed to update failed email state", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return apperror.NotFound("email message not found")
	}
	status := "retry_scheduled"
	if nextRetryAt == nil {
		status = "failed"
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO email_delivery_events (message_id, attempt, status, error) SELECT id, attempts, $2, $3 FROM email_messages WHERE id = $1`, id, status, errMsg); err != nil {
		return apperror.Database("failed to record failure event", err)
	}
	if err := tx.Commit(); err != nil {
		return apperror.Database("failed to commit failure transaction", err)
	}
	return nil
}

// GetByID retrieves a single email message by ID.
func (r *PostgresEmailMessageRepository) GetByID(ctx context.Context, id string) (*domain.EmailMessage, error) {
	const query = `
		SELECT id, sender_email, recipient, subject, body, format, status, use_global_smtp,
		       attempts, max_attempts, last_error, next_retry_at, created_at, updated_at,
		       delivered_at, failed_at
		FROM email_messages
		WHERE id = $1
	`
	msg := &domain.EmailMessage{}
	if err := r.db.GetContext(ctx, msg, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.NotFound("email message not found")
		}
		return nil, apperror.Database("failed to get email message", err)
	}
	return msg, nil
}

// ListEvents returns the immutable attempt trail for one message.
func (r *PostgresEmailMessageRepository) ListEvents(ctx context.Context, messageID string) ([]domain.EmailDeliveryEvent, error) {
	const query = `SELECT id, message_id, attempt, status, error, occurred_at FROM email_delivery_events WHERE message_id = $1 ORDER BY attempt ASC, id ASC`
	var events []domain.EmailDeliveryEvent
	if err := r.db.SelectContext(ctx, &events, query, messageID); err != nil {
		return nil, apperror.Database("failed to list delivery events", err)
	}
	if events == nil {
		events = []domain.EmailDeliveryEvent{}
	}
	return events, nil
}

// ListHistory retrieves paginated delivery history with optional filtering by status and recipient.
func (r *PostgresEmailMessageRepository) ListHistory(ctx context.Context, filter dto.EmailHistoryFilter) ([]*domain.EmailMessage, int, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	if strings.TrimSpace(filter.Status) != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, strings.TrimSpace(filter.Status))
		argIdx++
	}
	if strings.TrimSpace(filter.Recipient) != "" {
		conditions = append(conditions, fmt.Sprintf("recipient ILIKE $%d", argIdx))
		args = append(args, "%"+strings.TrimSpace(filter.Recipient)+"%")
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM email_messages %s", whereClause)
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, apperror.Database("failed to count email messages", err)
	}

	offset := (page - 1) * limit
	query := fmt.Sprintf(`
		SELECT id, sender_email, recipient, subject, body, format, status, use_global_smtp,
		       attempts, max_attempts, last_error, next_retry_at, created_at, updated_at,
		       delivered_at, failed_at
		FROM email_messages
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	queryArgs := append(args, limit, offset)
	var messages []*domain.EmailMessage
	if err := r.db.SelectContext(ctx, &messages, query, queryArgs...); err != nil {
		return nil, 0, apperror.Database("failed to list email messages", err)
	}

	if messages == nil {
		messages = []*domain.EmailMessage{}
	}
	return messages, total, nil
}

// RetryMessage resets a message to queued state for immediate retry.
func (r *PostgresEmailMessageRepository) RetryMessage(ctx context.Context, id string) error {
	const query = `
		UPDATE email_messages
		SET status = 'queued',
			attempts = 0,
			last_error = '',
			failed_at = NULL,
		    next_retry_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return apperror.Database("failed to retry email message", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return apperror.NotFound("email message not found")
	}
	return nil
}
