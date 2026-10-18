package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lokesh2201013/email-service/internal/config"
	"github.com/lokesh2201013/email-service/internal/domain"
	"github.com/lokesh2201013/email-service/internal/dto"
	"github.com/lokesh2201013/email-service/internal/errors/apperror"
	"github.com/lokesh2201013/email-service/internal/repository"
)

// SMTPSender abstracts email transmission for testability.
type SMTPSender interface {
	SendMail(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

type defaultSMTPSender struct{}

func (s *defaultSMTPSender) SendMail(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
	return smtp.SendMail(addr, a, from, to, msg)
}

// EmailService handles email sending, queueing, retries, and delivery tracking.
type EmailService struct {
	senderRepo       repository.SenderRepository
	analyticsRepo    repository.AnalyticsRepository
	emailMessageRepo repository.EmailMessageRepository
	smtpSender       SMTPSender
	cfg              *config.Config
	logger           *slog.Logger
	queueNotify      chan struct{}
}

// NewEmailService creates a new email service.
func NewEmailService(
	senderRepo repository.SenderRepository,
	analyticsRepo repository.AnalyticsRepository,
	emailMessageRepo repository.EmailMessageRepository,
	cfg *config.Config,
	logger *slog.Logger,
) *EmailService {
	return &EmailService{
		senderRepo:       senderRepo,
		analyticsRepo:    analyticsRepo,
		emailMessageRepo: emailMessageRepo,
		smtpSender:       &defaultSMTPSender{},
		cfg:              cfg,
		logger:           logger,
		queueNotify:      make(chan struct{}, 1),
	}
}

// SetSMTPSender allows injecting a custom or mock SMTP sender for testing.
func (s *EmailService) SetSMTPSender(sender SMTPSender) {
	s.smtpSender = sender
}

// SendEmail enqueues emails into the durable queue and triggers queue dispatch.
func (s *EmailService) SendEmail(ctx context.Context, req *dto.SendEmailRequest) (*dto.SendEmailResponse, error) {
	// Validate input
	if err := s.validateSendRequest(req); err != nil {
		return nil, err
	}

	sender, err := s.resolveSender(ctx, req.From)
	if err != nil {
		return nil, err
	}

	maxRetries := 3
	if s.cfg != nil && s.cfg.EmailMaxRetries > 0 {
		maxRetries = s.cfg.EmailMaxRetries
	}

	now := time.Now()
	messages := make([]*domain.EmailMessage, 0, len(req.To))
	messageIDs := make([]string, 0, len(req.To))

	for _, to := range req.To {
		id := uuid.New().String()
		messageIDs = append(messageIDs, id)
		messages = append(messages, &domain.EmailMessage{
			ID:          id,
			SenderEmail: sender.Email,
			Recipient:   to,
			Subject:     req.Subject,
			Body:        req.Body,
			Format:      req.Format,
			Status:      domain.EmailStatusQueued,
			Attempts:    0,
			MaxAttempts: maxRetries,
			NextRetryAt: now,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	if s.emailMessageRepo == nil {
		return nil, apperror.Internal("email queue is unavailable", errors.New("email message repository is nil"))
	}
	if err := s.emailMessageRepo.Enqueue(ctx, messages); err != nil {
		return nil, err
	}
	s.notifyQueue()

	s.logger.Info("emails enqueued", "sender", sender.Email, "recipients", len(req.To))

	return &dto.SendEmailResponse{
		Message:    fmt.Sprintf("emails queued for delivery to %d recipients", len(req.To)),
		Count:      len(req.To),
		MessageIDs: messageIDs,
	}, nil
}

// ProcessQueue processes pending queued messages in batches.
func (s *EmailService) ProcessQueue(ctx context.Context) (int, error) {
	if s.emailMessageRepo == nil {
		return 0, nil
	}

	batchSize := 10
	if s.cfg != nil && s.cfg.EmailWorkerBatchSize > 0 {
		batchSize = s.cfg.EmailWorkerBatchSize
	}

	messages, err := s.emailMessageRepo.GetPending(ctx, batchSize)
	if err != nil {
		return 0, err
	}

	if len(messages) == 0 {
		return 0, nil
	}

	for _, msg := range messages {
		s.processSingleMessage(ctx, msg)
	}

	return len(messages), nil
}

// processSingleMessage attempts to send an individual queued message.
func (s *EmailService) processSingleMessage(ctx context.Context, msg *domain.EmailMessage) {
	sender, err := s.resolveSender(ctx, msg.SenderEmail)
	if err != nil {
		s.handleDeliveryFailure(ctx, msg, fmt.Errorf("sender identity unavailable: %w", err))
		return
	}

	req := &dto.SendEmailRequest{
		From:    sender.Email,
		To:      []string{msg.Recipient},
		Subject: msg.Subject,
		Body:    msg.Body,
		Format:  msg.Format,
	}
	messageBytes := s.buildMessage(sender, req)

	addr := fmt.Sprintf("%s:%d", sender.SMTPHost, sender.SMTPPort)
	auth := smtp.PlainAuth("", sender.Username, sender.Password, sender.SMTPHost)

	if err := s.smtpSender.SendMail(addr, auth, sender.Email, []string{msg.Recipient}, messageBytes); err != nil {
		s.logger.Warn("delivery attempt failed", "message_id", msg.ID, "recipient", msg.Recipient, "error", err)
		s.handleDeliveryFailure(ctx, msg, err)
		return
	}

	if err := s.emailMessageRepo.MarkDelivered(ctx, msg.ID); err != nil {
		s.logger.Error("failed to mark email delivered", "message_id", msg.ID, "error", err)
	}

	if sender.ID != 0 && s.analyticsRepo != nil {
		if err := s.updateAnalytics(ctx, sender.ID, 1, 1); err != nil {
			s.logger.Error("failed to update analytics", "error", err)
		}
	}
	s.logger.Info("email delivered successfully", "message_id", msg.ID, "recipient", msg.Recipient)
}

// handleDeliveryFailure computes backoff and records failure or schedules a retry.
func (s *EmailService) handleDeliveryFailure(ctx context.Context, msg *domain.EmailMessage, deliveryErr error) {
	nextAttempt := msg.Attempts + 1
	errMsg := deliveryErr.Error()

	if nextAttempt < msg.MaxAttempts {
		// Exponential backoff: base 10 seconds * 2^attempts (capped at 2 hours)
		backoff := time.Duration(1<<uint(nextAttempt)) * 10 * time.Second
		if backoff > 2*time.Hour {
			backoff = 2 * time.Hour
		}
		nextRetry := time.Now().Add(backoff)
		if err := s.emailMessageRepo.MarkFailed(ctx, msg.ID, errMsg, &nextRetry); err != nil {
			s.logger.Error("failed to update message retry state", "message_id", msg.ID, "error", err)
		}
		s.logger.Info("email scheduled for retry", "message_id", msg.ID, "attempt", nextAttempt, "next_retry", nextRetry)
	} else {
		if err := s.emailMessageRepo.MarkFailed(ctx, msg.ID, errMsg, nil); err != nil {
			s.logger.Error("failed to mark message failed", "message_id", msg.ID, "error", err)
		}
		s.logger.Error("email delivery permanently failed", "message_id", msg.ID, "recipient", msg.Recipient, "error", errMsg)
	}
}

// StartQueueWorker launches the background worker loop processing queued messages.
func (s *EmailService) StartQueueWorker(ctx context.Context) {
	interval := 2 * time.Second
	if s.cfg != nil && s.cfg.EmailWorkerInterval > 0 {
		interval = s.cfg.EmailWorkerInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.logger.Info("email queue worker started", "interval", interval)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("email queue worker stopping")
			return
		case <-ticker.C:
			if _, err := s.ProcessQueue(ctx); err != nil {
				s.logger.Error("queue processing error", "error", err)
			}
		case <-s.queueNotify:
			if _, err := s.ProcessQueue(ctx); err != nil {
				s.logger.Error("queue processing error", "error", err)
			}
		}
	}
}

func (s *EmailService) notifyQueue() {
	select {
	case s.queueNotify <- struct{}{}:
	default:
	}
}

// GetHistory retrieves paginated delivery history with filtering.
func (s *EmailService) GetHistory(ctx context.Context, filter dto.EmailHistoryFilter) (*dto.EmailHistoryResponse, error) {
	if s.emailMessageRepo == nil {
		return nil, apperror.Internal("email repository unavailable", errors.New("email message repository is nil"))
	}

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

	messages, total, err := s.emailMessageRepo.ListHistory(ctx, filter)
	if err != nil {
		return nil, err
	}

	data := make([]dto.EmailMessageResponse, 0, len(messages))
	for _, m := range messages {
		data = append(data, toEmailMessageResponse(m, false))
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}

	return &dto.EmailHistoryResponse{
		Data: data,
		Page: dto.PageMetadata{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

// GetMessageByID retrieves a specific email message with full body and error history.
func (s *EmailService) GetMessageByID(ctx context.Context, id string) (*dto.EmailMessageResponse, error) {
	if s.emailMessageRepo == nil {
		return nil, apperror.Internal("email repository unavailable", errors.New("email message repository is nil"))
	}
	msg, err := s.emailMessageRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := toEmailMessageResponse(msg, true)
	events, err := s.emailMessageRepo.ListEvents(ctx, id)
	if err != nil {
		return nil, err
	}
	resp.Events = make([]dto.EmailDeliveryEventResponse, 0, len(events))
	for _, event := range events {
		resp.Events = append(resp.Events, dto.EmailDeliveryEventResponse{Attempt: event.Attempt, Status: event.Status, Error: event.Error, OccurredAt: event.OccurredAt})
	}
	return &resp, nil
}

// RetryMessage queues a failed or stuck email message for immediate retry.
func (s *EmailService) RetryMessage(ctx context.Context, id string) (*dto.RetryEmailResponse, error) {
	if s.emailMessageRepo == nil {
		return nil, apperror.Internal("email repository unavailable", errors.New("email message repository is nil"))
	}
	msg, err := s.emailMessageRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if msg.Status == domain.EmailStatusDelivered {
		return nil, apperror.Validation("delivered email cannot be retried")
	}

	if err := s.emailMessageRepo.RetryMessage(ctx, id); err != nil {
		return nil, err
	}
	s.notifyQueue()

	return &dto.RetryEmailResponse{
		Message: "email queued for retry",
		ID:      id,
		Status:  domain.EmailStatusQueued,
	}, nil
}

func toEmailMessageResponse(m *domain.EmailMessage, includeBody bool) dto.EmailMessageResponse {
	resp := dto.EmailMessageResponse{
		ID:          m.ID,
		SenderEmail: m.SenderEmail,
		Recipient:   m.Recipient,
		Subject:     m.Subject,
		Format:      m.Format,
		Status:      m.Status,
		Attempts:    m.Attempts,
		MaxAttempts: m.MaxAttempts,
		LastError:   m.LastError,
		CreatedAt:   m.CreatedAt,
		DeliveredAt: m.DeliveredAt,
		FailedAt:    m.FailedAt,
	}
	if !m.NextRetryAt.IsZero() && m.Status == domain.EmailStatusQueued {
		resp.NextRetryAt = &m.NextRetryAt
	}
	if includeBody {
		resp.Body = m.Body
	}
	return resp
}

// GetAnalytics retrieves analytics for a sender.
func (s *EmailService) GetAnalytics(ctx context.Context, senderEmail string) (*dto.AnalyticsResponse, error) {
	sender, err := s.senderRepo.GetByEmail(ctx, senderEmail)
	if err != nil {
		return nil, err
	}

	analytics, err := s.analyticsRepo.GetBySenderID(ctx, sender.ID)
	if err != nil {
		return nil, err
	}

	analytics.CalculateMetrics()
	return s.analyticsToDTO(analytics, senderEmail), nil
}

// GetAdminAnalytics retrieves analytics for all senders of an admin.
func (s *EmailService) GetAdminAnalytics(ctx context.Context, adminName string) (*dto.AdminAnalyticsResponse, error) {
	records, err := s.analyticsRepo.ListByAdminName(ctx, adminName)
	if err != nil {
		return nil, err
	}

	result := make([]dto.AnalyticsResponse, 0, len(records))
	for i := range records {
		records[i].CalculateMetrics()
		sender, err := s.senderRepo.GetByID(ctx, records[i].SenderID)
		if err != nil {
			s.logger.Warn("failed to get sender for analytics", "error", err)
			continue
		}
		result = append(result, *s.analyticsToDTO(&records[i], sender.Email))
	}

	return &dto.AdminAnalyticsResponse{Senders: result}, nil
}

// buildMessage constructs an RFC 822 compliant email message.
func (s *EmailService) buildMessage(sender *domain.Sender, req *dto.SendEmailRequest) []byte {
	var message strings.Builder

	message.WriteString(fmt.Sprintf("From: %s\r\n", sender.Email))
	message.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(req.To, ",")))
	message.WriteString(fmt.Sprintf("Subject: %s\r\n", req.Subject))

	if req.Format == "html" {
		message.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	} else {
		message.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	}

	message.WriteString("\r\n")
	message.WriteString(req.Body)

	return []byte(message.String())
}

// updateAnalytics increments email counts for a sender.
func (s *EmailService) updateAnalytics(ctx context.Context, senderID uint, totalSent int, successCount int) error {
	analytics, err := s.analyticsRepo.GetBySenderID(ctx, senderID)
	if err != nil {
		if apperror.IsType(err, apperror.NotFoundError) {
			sender, err := s.senderRepo.GetByID(ctx, senderID)
			if err != nil {
				return err
			}

			newAnalytics := &domain.Analytics{
				SenderID:         senderID,
				AdminName:        sender.AdminName,
				TotalEmails:      totalSent,
				AccumulatedEmail: totalSent,
				Delivered:        successCount,
			}
			return s.analyticsRepo.Create(ctx, newAnalytics)
		}
		return err
	}

	analytics.TotalEmails += totalSent
	analytics.AccumulatedEmail += totalSent
	analytics.Delivered += successCount

	return s.analyticsRepo.Update(ctx, analytics)
}

// analyticsToDTO converts domain analytics to response DTO.
func (s *EmailService) analyticsToDTO(analytics *domain.Analytics, senderEmail string) *dto.AnalyticsResponse {
	return &dto.AnalyticsResponse{
		SenderEmail:   senderEmail,
		TotalEmails:   analytics.TotalEmails,
		Delivered:     analytics.Delivered,
		Bounced:       analytics.Bounced,
		Complaints:    analytics.Complaints,
		Rejected:      analytics.Rejected,
		DeliveryRate:  analytics.DeliveryRate,
		BounceRate:    analytics.BounceRate,
		ComplaintRate: analytics.ComplaintRate,
		RejectRate:    analytics.RejectRate,
	}
}

// resolveSender looks up a verified sender identity, or uses SMTP config
// when From is the service default (assignment notifications have no From field).
func (s *EmailService) resolveSender(ctx context.Context, from string) (*domain.Sender, error) {
	if from == "" {
		if s.cfg == nil || s.cfg.SMTPFrom == "" {
			return nil, apperror.Validation("from email is required")
		}
		from = s.cfg.SMTPFrom
	}

	if s.senderRepo != nil {
		sender, err := s.senderRepo.GetByEmail(ctx, from)
		if err == nil {
			if !sender.Verified {
				return nil, apperror.Validation("sender email is not verified")
			}
			return sender, nil
		}
	}

	if s.cfg != nil && from == s.cfg.SMTPFrom {
		return &domain.Sender{
			Email:    s.cfg.SMTPFrom,
			SMTPHost: s.cfg.SMTPHost,
			SMTPPort: s.cfg.SMTPPort,
			Username: s.cfg.SMTPUsername,
			Password: s.cfg.SMTPPassword,
			Verified: true,
		}, nil
	}

	return nil, apperror.Validation("sender email not found or not verified")
}

// validateSendRequest validates send email input.
func (s *EmailService) validateSendRequest(req *dto.SendEmailRequest) error {
	if len(req.From) == 0 && (s.cfg == nil || s.cfg.SMTPFrom == "") {
		return apperror.Validation("from email is required")
	}
	if len(req.To) == 0 {
		return apperror.Validation("at least one recipient is required")
	}
	if len(req.Subject) == 0 {
		return apperror.Validation("subject is required")
	}
	if len(req.Body) == 0 {
		return apperror.Validation("body is required")
	}
	if req.Format == "" {
		req.Format = "text"
	}
	if req.Format != "text" && req.Format != "html" {
		return apperror.Validation("format must be 'text' or 'html'")
	}
	return nil
}
