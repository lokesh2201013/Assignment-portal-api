package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"

	"github.com/lokesh2201013/email-service/internal/config"
	"github.com/lokesh2201013/email-service/internal/domain"
	"github.com/lokesh2201013/email-service/internal/dto"
	"github.com/lokesh2201013/email-service/internal/errors/apperror"
	"github.com/lokesh2201013/email-service/internal/repository"
)

// EmailService handles email sending and tracking.
type EmailService struct {
	senderRepo    repository.SenderRepository
	analyticsRepo repository.AnalyticsRepository
	cfg           *config.Config
	logger        *slog.Logger
}

// NewEmailService creates a new email service.
func NewEmailService(
	senderRepo repository.SenderRepository,
	analyticsRepo repository.AnalyticsRepository,
	cfg *config.Config,
	logger *slog.Logger,
) *EmailService {
	return &EmailService{
		senderRepo:    senderRepo,
		analyticsRepo: analyticsRepo,
		cfg:           cfg,
		logger:        logger,
	}
}

// SendEmail sends emails through a verified sender identity and tracks analytics.
func (s *EmailService) SendEmail(ctx context.Context, req *dto.SendEmailRequest) (*dto.SendEmailResponse, error) {
	// Validate input
	if err := s.validateSendRequest(req); err != nil {
		return nil, err
	}

	sender, err := s.resolveSender(ctx, req.From)
	if err != nil {
		return nil, err
	}

	// Build email message
	message := s.buildMessage(sender, req)

	// Send emails
	sentCount, failed := s.sendToRecipients(sender, req.To, message)

	// Update analytics when the sender is a persisted identity
	if sender.ID != 0 {
		if err := s.updateAnalytics(ctx, sender.ID, len(req.To), sentCount); err != nil {
			s.logger.Error("failed to update analytics", "error", err)
			// Don't fail the request if analytics update fails
		}
	}

	s.logger.Info("emails sent", "sender", sender.Email, "sent", sentCount, "failed", failed)

	return &dto.SendEmailResponse{
		Message: fmt.Sprintf("emails sent successfully to %d recipients", sentCount),
		Count:   sentCount,
	}, nil
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

	// Calculate rates
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
		// Get sender email for display
		sender, err := s.senderRepo.GetByID(ctx, records[i].SenderID)
		if err != nil {
			s.logger.Warn("failed to get sender for analytics", "error", err)
			continue
		}
		result = append(result, *s.analyticsToDTO(&records[i], sender.Email))
	}

	return &dto.AdminAnalyticsResponse{Senders: result}, nil
}

// sendToRecipients attempts to send an email to multiple recipients.
// Returns (sentCount, failedCount).
func (s *EmailService) sendToRecipients(sender *domain.Sender, recipients []string, message []byte) (int, int) {
	addr := fmt.Sprintf("%s:%d", sender.SMTPHost, sender.SMTPPort)
	auth := smtp.PlainAuth("", sender.Username, sender.Password, sender.SMTPHost)

	sentCount := 0
	for _, to := range recipients {
		if err := smtp.SendMail(addr, auth, sender.Email, []string{to}, message); err != nil {
			s.logger.Warn("failed to send email", "to", to, "error", err)
			continue
		}
		sentCount++
	}

	return sentCount, len(recipients) - sentCount
}

// buildMessage constructs an RFC 822 compliant email message.
func (s *EmailService) buildMessage(sender *domain.Sender, req *dto.SendEmailRequest) []byte {
	var message strings.Builder

	// Headers
	message.WriteString(fmt.Sprintf("From: %s\r\n", sender.Email))
	message.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(req.To, ",")))
	message.WriteString(fmt.Sprintf("Subject: %s\r\n", req.Subject))

	// Content type based on format
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
			// Create new analytics record if it doesn't exist
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

	// Update existing record
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

	sender, err := s.senderRepo.GetByEmail(ctx, from)
	if err == nil {
		if !sender.Verified {
			return nil, apperror.Validation("sender email is not verified")
		}
		return sender, nil
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
