package service

import (
	"context"
	"log/slog"
	"net/smtp"

	"github.com/lokesh2201013/email-service/internal/domain"
	"github.com/lokesh2201013/email-service/internal/dto"
	"github.com/lokesh2201013/email-service/internal/errors/apperror"
	"github.com/lokesh2201013/email-service/internal/repository"
)

// SenderService handles email sender identity operations.
type SenderService struct {
	senderRepo repository.SenderRepository
	logger     *slog.Logger
}

// NewSenderService creates a new sender service.
func NewSenderService(senderRepo repository.SenderRepository, logger *slog.Logger) *SenderService {
	return &SenderService{
		senderRepo: senderRepo,
		logger:     logger,
	}
}

// VerifyIdentity verifies an email sender identity via SMTP connection test.
func (s *SenderService) VerifyIdentity(ctx context.Context, req *dto.VerifyEmailRequest, adminName string) (*dto.SenderResponse, error) {
	// Validate input
	if err := s.validateVerifyRequest(req); err != nil {
		return nil, err
	}

	// Check if sender already exists
	exists, err := s.senderRepo.Exists(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperror.Conflict("email already registered as sender")
	}

	// Test SMTP connection
	if err := s.testSMTPConnection(req); err != nil {
		s.logger.Warn("SMTP verification failed", "email", req.Email, "error", err.Error())
		return nil, apperror.ExternalService("failed to verify SMTP credentials", err)
	}

	// Create sender
	sender := &domain.Sender{
		AdminName: adminName,
		Email:     req.Email,
		SMTPHost:  req.SMTPHost,
		SMTPPort:  req.SMTPPort,
		Username:  req.Username,
		Password:  req.AppPassword, // Store encrypted in production
		Verified:  true,
	}

	if err := s.senderRepo.Create(ctx, sender); err != nil {
		return nil, err
	}

	s.logger.Info("sender verified successfully", "email", req.Email, "admin", adminName)

	return s.senderToDTO(sender), nil
}

// ListVerifiedSenders returns all verified senders for an admin.
func (s *SenderService) ListVerifiedSenders(ctx context.Context, adminName string) ([]dto.SenderResponse, error) {
	senders, err := s.senderRepo.ListByAdminAndVerified(ctx, adminName, true)
	if err != nil {
		return nil, err
	}

	result := make([]dto.SenderResponse, 0, len(senders))
	for i := range senders {
		result = append(result, *s.senderToDTO(&senders[i]))
	}

	return result, nil
}

// GetSender retrieves a sender by email.
func (s *SenderService) GetSender(ctx context.Context, email string) (*domain.Sender, error) {
	return s.senderRepo.GetByEmail(ctx, email)
}

// GetSenderByID retrieves a sender by ID.
func (s *SenderService) GetSenderByID(ctx context.Context, senderID uint) (*domain.Sender, error) {
	return s.senderRepo.GetByID(ctx, senderID)
}

// DeleteSender removes a sender identity.
func (s *SenderService) DeleteSender(ctx context.Context, email string, adminName string) error {
	// Get sender to verify ownership
	sender, err := s.senderRepo.GetByEmail(ctx, email)
	if err != nil {
		return err
	}

	// Verify admin owns this sender
	if sender.AdminName != adminName {
		return apperror.Forbidden("cannot delete sender owned by another admin")
	}

	if err := s.senderRepo.Delete(ctx, email); err != nil {
		return err
	}

	s.logger.Info("sender deleted", "email", email, "admin", adminName)
	return nil
}

// testSMTPConnection verifies SMTP credentials by attempting a connection.
func (s *SenderService) testSMTPConnection(req *dto.VerifyEmailRequest) error {
	// Create auth
	auth := smtp.PlainAuth("", req.Username, req.AppPassword, req.SMTPHost)

	// Try to connect and send test message
	addr := req.SMTPHost + ":" + string(rune(req.SMTPPort))
	conn, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Start TLS if available
	if err := conn.StartTLS(nil); err != nil {
		s.logger.Warn("TLS not available for SMTP", "host", req.SMTPHost)
		// Continue without TLS, some servers don't support it
	}

	// Authenticate
	if err := conn.Auth(auth); err != nil {
		return err
	}

	// Close connection gracefully
	conn.Quit()
	return nil
}

// senderToDTO converts a domain sender to response DTO (no password exposure).
func (s *SenderService) senderToDTO(sender *domain.Sender) *dto.SenderResponse {
	return &dto.SenderResponse{
		ID:        sender.ID,
		Email:     sender.Email,
		AdminName: sender.AdminName,
		SMTPHost:  sender.SMTPHost,
		SMTPPort:  sender.SMTPPort,
		Verified:  sender.Verified,
		CreatedAt: sender.CreatedAt,
		UpdatedAt: sender.UpdatedAt,
	}
}

// validateVerifyRequest validates sender verification input.
func (s *SenderService) validateVerifyRequest(req *dto.VerifyEmailRequest) error {
	if len(req.AdminName) == 0 {
		return apperror.Validation("admin_name is required")
	}
	if len(req.Email) == 0 {
		return apperror.Validation("email is required")
	}
	if len(req.SMTPHost) == 0 {
		return apperror.Validation("smtp_host is required")
	}
	if req.SMTPPort <= 0 || req.SMTPPort > 65535 {
		return apperror.Validation("smtp_port must be between 1 and 65535")
	}
	if len(req.Username) == 0 {
		return apperror.Validation("username is required")
	}
	if len(req.AppPassword) == 0 {
		return apperror.Validation("password is required")
	}
	return nil
}
