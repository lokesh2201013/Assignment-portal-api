package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/smtp"
	"testing"
	"time"

	"github.com/lokesh2201013/email-service/internal/config"
	"github.com/lokesh2201013/email-service/internal/domain"
	"github.com/lokesh2201013/email-service/internal/dto"
	"github.com/lokesh2201013/email-service/internal/errors/apperror"
)

type securitySenderRepo struct {
	sender *domain.Sender
	err    error
	calls  int
}

func (r *securitySenderRepo) Create(context.Context, *domain.Sender) error { return nil }
func (r *securitySenderRepo) GetByEmail(context.Context, string) (*domain.Sender, error) {
	r.calls++
	return r.sender, r.err
}
func (r *securitySenderRepo) GetByID(context.Context, uint) (*domain.Sender, error) {
	return nil, errors.New("not implemented")
}
func (r *securitySenderRepo) ListByAdminAndVerified(context.Context, string, bool) ([]domain.Sender, error) {
	return nil, nil
}
func (r *securitySenderRepo) Update(context.Context, *domain.Sender) error { return nil }
func (r *securitySenderRepo) Delete(context.Context, string) error         { return nil }
func (r *securitySenderRepo) Exists(context.Context, string) (bool, error) { return false, nil }

type securityEmailRepo struct {
	enqueued  []*domain.EmailMessage
	delivered string
	failed    string
}

func (r *securityEmailRepo) Enqueue(_ context.Context, messages []*domain.EmailMessage) error {
	r.enqueued = append(r.enqueued, messages...)
	return nil
}
func (r *securityEmailRepo) GetPending(context.Context, int) ([]*domain.EmailMessage, error) {
	return nil, nil
}
func (r *securityEmailRepo) MarkProcessing(context.Context, string) error { return nil }
func (r *securityEmailRepo) MarkDelivered(_ context.Context, id string) error {
	r.delivered = id
	return nil
}
func (r *securityEmailRepo) MarkFailed(_ context.Context, id, _ string, _ *time.Time) error {
	r.failed = id
	return nil
}
func (r *securityEmailRepo) GetByID(context.Context, string) (*domain.EmailMessage, error) {
	return nil, errors.New("not implemented")
}
func (r *securityEmailRepo) ListEvents(context.Context, string) ([]domain.EmailDeliveryEvent, error) {
	return nil, nil
}
func (r *securityEmailRepo) ListHistory(context.Context, dto.EmailHistoryFilter) ([]*domain.EmailMessage, int, error) {
	return nil, 0, nil
}
func (r *securityEmailRepo) RetryMessage(context.Context, string) error { return nil }

type recordingSMTPSender struct {
	addr string
	from string
	to   []string
}

func (s *recordingSMTPSender) SendMail(addr string, _ smtp.Auth, from string, to []string, _ []byte) error {
	s.addr, s.from, s.to = addr, from, to
	return nil
}

func securityEmailService(sender *securitySenderRepo, messages *securityEmailRepo) *EmailService {
	cfg := &config.Config{
		SMTPFrom: "service@example.com", SMTPHost: "smtp.global.test", SMTPPort: 587,
		SMTPUsername: "global-user", SMTPPassword: "global-password",
	}
	return NewEmailService(sender, nil, messages, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func validRequest(from string) *dto.SendEmailRequest {
	return &dto.SendEmailRequest{
		From: from, To: []string{"recipient@example.net"}, Subject: "assignment",
		Body: "notice", Format: "text",
	}
}

func TestSendEmailForUserRequiresOwnedExplicitSender(t *testing.T) {
	messages := &securityEmailRepo{}
	senderRepo := &securitySenderRepo{
		sender: &domain.Sender{Email: "owner@example.com", AdminName: "alice", Verified: true},
	}
	service := securityEmailService(senderRepo, messages)

	if _, err := service.SendEmailForUser(context.Background(), "bob", validRequest("owner@example.com")); !apperror.IsType(err, apperror.ForbiddenError) {
		t.Fatalf("foreign sender error = %v, want forbidden", err)
	}
	if len(messages.enqueued) != 0 {
		t.Fatal("message was enqueued for a sender owned by another user")
	}

	if _, err := service.SendEmailForUser(context.Background(), "alice", validRequest("owner@example.com")); err != nil {
		t.Fatalf("owned sender was rejected: %v", err)
	}
	if len(messages.enqueued) != 1 || messages.enqueued[0].UseGlobalSMTP {
		t.Fatalf("owned sender queue entry = %#v, want a non-global message", messages.enqueued)
	}
}

func TestSendEmailForUserCannotFallbackToGlobalSMTP(t *testing.T) {
	messages := &securityEmailRepo{}
	senderRepo := &securitySenderRepo{err: apperror.NotFound("sender not found")}
	service := securityEmailService(senderRepo, messages)

	if _, err := service.SendEmailForUser(context.Background(), "new-user", validRequest("service@example.com")); err == nil {
		t.Fatal("REST send unexpectedly fell back to global SMTP credentials")
	}
	if len(messages.enqueued) != 0 {
		t.Fatal("global SMTP message was enqueued by a registered user")
	}
}

func TestInternalNotificationUsesDedicatedGlobalSMTPQueuePath(t *testing.T) {
	messages := &securityEmailRepo{}
	senderRepo := &securitySenderRepo{err: apperror.NotFound("sender not found")}
	service := securityEmailService(senderRepo, messages)

	req := validRequest("attacker@example.com")
	if _, err := service.SendInternalNotification(context.Background(), req); err != nil {
		t.Fatalf("internal notification was rejected: %v", err)
	}
	if len(messages.enqueued) != 1 {
		t.Fatalf("queued messages = %d, want 1", len(messages.enqueued))
	}
	if !messages.enqueued[0].UseGlobalSMTP || messages.enqueued[0].SenderEmail != "service@example.com" {
		t.Fatalf("internal queue entry = %#v, want configured global sender", messages.enqueued[0])
	}

	smtpSender := &recordingSMTPSender{}
	service.SetSMTPSender(smtpSender)
	service.processSingleMessage(context.Background(), messages.enqueued[0])
	if smtpSender.addr != "smtp.global.test:587" || smtpSender.from != "service@example.com" {
		t.Fatalf("SMTP send used addr=%q from=%q, want configured internal identity", smtpSender.addr, smtpSender.from)
	}
	if messages.failed != "" || messages.delivered != messages.enqueued[0].ID {
		t.Fatalf("delivery status: failed=%q delivered=%q", messages.failed, messages.delivered)
	}
	if senderRepo.calls != 0 {
		t.Fatal("global internal delivery unexpectedly queried user sender records")
	}
}
