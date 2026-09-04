// Package handler contains gRPC service implementations.
package handler

import (
	"context"
	"log/slog"

	"github.com/lokesh2201013/email-service/internal/dto"
	"github.com/lokesh2201013/email-service/internal/service"
	pb "github.com/lokesh2201013/email-service/proto"
)

// EmailServiceServer implements the gRPC Email Service.
type EmailServiceServer struct {
	pb.UnimplementedEmailServiceServer
	emailService *service.EmailService
	smtpFrom     string
	logger       *slog.Logger
}

// NewEmailServiceServer creates a new email service server.
func NewEmailServiceServer(emailService *service.EmailService, smtpFrom string, logger *slog.Logger) pb.EmailServiceServer {
	return &EmailServiceServer{
		emailService: emailService,
		smtpFrom:     smtpFrom,
		logger:       logger,
	}
}

// SendAssignmentNotification sends an assignment notification via gRPC.
func (s *EmailServiceServer) SendAssignmentNotification(ctx context.Context, req *pb.AssignmentEmailRequest) (*pb.EmailResponse, error) {
	dtoReq := &dto.SendEmailRequest{
		From:    s.smtpFrom,
		To:      req.GetRecipients(),
		Subject: req.GetSubject(),
		Body:    req.GetBody(),
		Format:  "text",
	}

	resp, err := s.emailService.SendEmail(ctx, dtoReq)
	if err != nil {
		s.logger.Error("SendAssignmentNotification error", "error", err)
		return nil, err
	}

	return &pb.EmailResponse{
		Success: true,
		Message: resp.Message,
	}, nil
}

// GetMetrics retrieves email metrics via gRPC.
// func (s *EmailServiceServer) GetMetrics(ctx context.Context, req *pb.GetMetricsRequest) (*pb.MetricsResponse, error) {
// 	// Call service
// 	resp, err := s.emailService.GetAnalytics(ctx, req.SenderEmail)
// 	if err != nil {
// 		s.logger.Error("GetMetrics error", "error", err)
// 		return nil, err
// 	}

// 	// Convert DTO response to gRPC
// 	return &pb.MetricsResponse{
// 		SenderEmail:   resp.SenderEmail,
// 		TotalEmails:   int32(resp.TotalEmails),
// 		Delivered:     int32(resp.Delivered),
// 		Bounced:       int32(resp.Bounced),
// 		Complaints:    int32(resp.Complaints),
// 		Rejected:      int32(resp.Rejected),
// 		DeliveryRate:  resp.DeliveryRate,
// 		BounceRate:    resp.BounceRate,
// 		ComplaintRate: resp.ComplaintRate,
// 		RejectRate:    resp.RejectRate,
// 	}, nil
// }
