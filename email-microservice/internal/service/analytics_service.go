package service

import (
	"context"
	"log/slog"

	"github.com/lokesh2201013/email-service/internal/domain"
	"github.com/lokesh2201013/email-service/internal/errors/apperror"
	"github.com/lokesh2201013/email-service/internal/repository"
)

// AnalyticsService handles analytics queries and calculations.
type AnalyticsService struct {
	analyticsRepo repository.AnalyticsRepository
	senderRepo    repository.SenderRepository
	logger        *slog.Logger
}

// NewAnalyticsService creates a new analytics service.
func NewAnalyticsService(
	analyticsRepo repository.AnalyticsRepository,
	senderRepo repository.SenderRepository,
	logger *slog.Logger,
) *AnalyticsService {
	return &AnalyticsService{
		analyticsRepo: analyticsRepo,
		senderRepo:    senderRepo,
		logger:        logger,
	}
}

// GetSenderAnalytics retrieves detailed analytics for a specific sender.
func (s *AnalyticsService) GetSenderAnalytics(ctx context.Context, senderID uint) (*domain.Analytics, error) {
	analytics, err := s.analyticsRepo.GetBySenderID(ctx, senderID)
	if err != nil {
		return nil, err
	}

	analytics.CalculateMetrics()
	return analytics, nil
}

// GetAdminAnalytics retrieves aggregated analytics for all senders of an admin.
func (s *AnalyticsService) GetAdminAnalytics(ctx context.Context, adminName string) (*domain.Analytics, error) {
	records, err := s.analyticsRepo.ListByAdminName(ctx, adminName)
	if err != nil {
		return nil, err
	}

	// Aggregate all metrics
	aggregated := &domain.Analytics{
		AdminName: adminName,
	}

	for _, record := range records {
		aggregated.TotalEmails += record.TotalEmails
		aggregated.AccumulatedEmail += record.AccumulatedEmail
		aggregated.Delivered += record.Delivered
		aggregated.Bounced += record.Bounced
		aggregated.Complaints += record.Complaints
		aggregated.Rejected += record.Rejected
	}

	aggregated.CalculateMetrics()
	return aggregated, nil
}

// RecordDelivery records successful email delivery for a sender.
func (s *AnalyticsService) RecordDelivery(ctx context.Context, senderID uint) error {
	analytics, err := s.analyticsRepo.GetBySenderID(ctx, senderID)
	if err != nil {
		if apperror.IsType(err, apperror.NotFoundError) {
			// Analytics doesn't exist yet, should not happen in normal flow
			sender, err := s.senderRepo.GetByID(ctx, senderID)
			if err != nil {
				return err
			}
			newAnalytics := &domain.Analytics{
				SenderID:  senderID,
				AdminName: sender.AdminName,
				Delivered: 1,
			}
			return s.analyticsRepo.Create(ctx, newAnalytics)
		}
		return err
	}

	analytics.Delivered++
	return s.analyticsRepo.Update(ctx, analytics)
}

// RecordBounce records a bounced email.
func (s *AnalyticsService) RecordBounce(ctx context.Context, senderID uint) error {
	analytics, err := s.analyticsRepo.GetBySenderID(ctx, senderID)
	if err != nil {
		return err
	}

	analytics.Bounced++
	return s.analyticsRepo.Update(ctx, analytics)
}

// RecordComplaint records a complaint for an email.
func (s *AnalyticsService) RecordComplaint(ctx context.Context, senderID uint) error {
	analytics, err := s.analyticsRepo.GetBySenderID(ctx, senderID)
	if err != nil {
		return err
	}

	analytics.Complaints++
	return s.analyticsRepo.Update(ctx, analytics)
}

// RecordRejection records a rejected email.
func (s *AnalyticsService) RecordRejection(ctx context.Context, senderID uint) error {
	analytics, err := s.analyticsRepo.GetBySenderID(ctx, senderID)
	if err != nil {
		return err
	}

	analytics.Rejected++
	return s.analyticsRepo.Update(ctx, analytics)
}

// GetDeliveryRate calculates the delivery rate for a sender.
func (s *AnalyticsService) GetDeliveryRate(ctx context.Context, senderID uint) (float64, error) {
	analytics, err := s.GetSenderAnalytics(ctx, senderID)
	if err != nil {
		return 0, err
	}

	return analytics.DeliveryRate, nil
}

// GetAdminDeliveryRate calculates the delivery rate for an admin.
func (s *AnalyticsService) GetAdminDeliveryRate(ctx context.Context, adminName string) (float64, error) {
	analytics, err := s.GetAdminAnalytics(ctx, adminName)
	if err != nil {
		return 0, err
	}

	return analytics.DeliveryRate, nil
}
