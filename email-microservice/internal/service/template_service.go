package service

import (
	"context"
	"log/slog"

	"github.com/lokesh2201013/email-service/internal/domain"
	"github.com/lokesh2201013/email-service/internal/dto"
	"github.com/lokesh2201013/email-service/internal/errors/apperror"
	"github.com/lokesh2201013/email-service/internal/repository"
)

// TemplateService handles email template operations.
type TemplateService struct {
	templateRepo repository.TemplateRepository
	logger       *slog.Logger
}

// NewTemplateService creates a new template service.
func NewTemplateService(templateRepo repository.TemplateRepository, logger *slog.Logger) *TemplateService {
	return &TemplateService{
		templateRepo: templateRepo,
		logger:       logger,
	}
}

// CreateTemplate creates a new email template.
func (s *TemplateService) CreateTemplate(ctx context.Context, req *dto.CreateTemplateRequest) (*dto.TemplateResponse, error) {
	// Validate input
	if err := s.validateTemplateRequest(req); err != nil {
		return nil, err
	}

	// Create template
	template := &domain.Template{
		Name:    req.Name,
		Subject: req.Subject,
		Body:    req.Body,
		Format:  req.Format,
	}

	if err := s.templateRepo.Create(ctx, template); err != nil {
		return nil, err
	}

	s.logger.Info("template created", "name", template.Name)
	return s.templateToDTO(template), nil
}

// GetTemplate retrieves a template by ID.
func (s *TemplateService) GetTemplate(ctx context.Context, templateID uint) (*domain.Template, error) {
	return s.templateRepo.GetByID(ctx, templateID)
}

// GetTemplateByName retrieves a template by name.
func (s *TemplateService) GetTemplateByName(ctx context.Context, name string) (*domain.Template, error) {
	return s.templateRepo.GetByName(ctx, name)
}

// ListTemplates returns all available templates.
func (s *TemplateService) ListTemplates(ctx context.Context) ([]dto.TemplateResponse, error) {
	templates, err := s.templateRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]dto.TemplateResponse, 0, len(templates))
	for i := range templates {
		result = append(result, *s.templateToDTO(&templates[i]))
	}

	return result, nil
}

// UpdateTemplate updates an existing template.
func (s *TemplateService) UpdateTemplate(ctx context.Context, templateID uint, req *dto.CreateTemplateRequest) (*dto.TemplateResponse, error) {
	// Validate input
	if err := s.validateTemplateRequest(req); err != nil {
		return nil, err
	}

	// Get existing template
	template, err := s.templateRepo.GetByID(ctx, templateID)
	if err != nil {
		return nil, err
	}

	// Update fields
	template.Name = req.Name
	template.Subject = req.Subject
	template.Body = req.Body
	template.Format = req.Format

	if err := s.templateRepo.Update(ctx, template); err != nil {
		return nil, err
	}

	s.logger.Info("template updated", "id", templateID, "name", template.Name)
	return s.templateToDTO(template), nil
}

// DeleteTemplate removes a template.
func (s *TemplateService) DeleteTemplate(ctx context.Context, templateID uint) error {
	if err := s.templateRepo.Delete(ctx, templateID); err != nil {
		return err
	}

	s.logger.Info("template deleted", "id", templateID)
	return nil
}

// templateToDTO converts a domain template to response DTO.
func (s *TemplateService) templateToDTO(template *domain.Template) *dto.TemplateResponse {
	return &dto.TemplateResponse{
		ID:        template.ID,
		Name:      template.Name,
		Subject:   template.Subject,
		Body:      template.Body,
		Format:    template.Format,
		CreatedAt: template.CreatedAt,
		UpdatedAt: template.UpdatedAt,
	}
}

// validateTemplateRequest validates template creation/update input.
func (s *TemplateService) validateTemplateRequest(req *dto.CreateTemplateRequest) error {
	if len(req.Name) == 0 {
		return apperror.Validation("name is required")
	}
	if len(req.Name) > 255 {
		return apperror.Validation("name must not exceed 255 characters")
	}
	if len(req.Subject) == 0 {
		return apperror.Validation("subject is required")
	}
	if len(req.Subject) > 255 {
		return apperror.Validation("subject must not exceed 255 characters")
	}
	if len(req.Body) == 0 {
		return apperror.Validation("body is required")
	}
	if req.Format != "text" && req.Format != "html" {
		return apperror.Validation("format must be 'text' or 'html'")
	}
	return nil
}
