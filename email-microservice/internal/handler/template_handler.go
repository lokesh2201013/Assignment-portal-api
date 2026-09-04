package handler

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/lokesh2201013/email-service/internal/dto"
	"github.com/lokesh2201013/email-service/internal/errors/apperror"
	"github.com/lokesh2201013/email-service/internal/service"
)

// TemplateHandler handles email template endpoints.
type TemplateHandler struct {
	templateService *service.TemplateService
	logger          *slog.Logger
}

// NewTemplateHandler creates a new template handler.
func NewTemplateHandler(templateService *service.TemplateService, logger *slog.Logger) *TemplateHandler {
	return &TemplateHandler{
		templateService: templateService,
		logger:          logger,
	}
}

// CreateTemplate creates a new email template.
// POST /templates
func (h *TemplateHandler) CreateTemplate(c *fiber.Ctx) error {
	var req dto.CreateTemplateRequest

	// Parse request body
	if err := c.BodyParser(&req); err != nil {
		return h.sendError(c, apperror.Validation("invalid request body"))
	}

	// Call service
	resp, err := h.templateService.CreateTemplate(c.Context(), &req)
	if err != nil {
		return h.sendError(c, err)
	}

	h.logger.Info("template created", "name", req.Name)
	return c.Status(201).JSON(fiber.Map{
		"message": "template created successfully",
		"data":    resp,
	})
}

// ListTemplates returns all available templates.
// GET /templates
func (h *TemplateHandler) ListTemplates(c *fiber.Ctx) error {
	// Call service
	templates, err := h.templateService.ListTemplates(c.Context())
	if err != nil {
		return h.sendError(c, err)
	}

	return c.JSON(fiber.Map{
		"message": "templates retrieved successfully",
		"data":    templates,
	})
}

// GetTemplate retrieves a specific template by ID.
// GET /templates/:id
func (h *TemplateHandler) GetTemplate(c *fiber.Ctx) error {
	idStr := c.Params("id")

	// Parse ID
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return h.sendError(c, apperror.Validation("invalid template ID"))
	}

	// Call service
	template, err := h.templateService.GetTemplate(c.Context(), uint(id))
	if err != nil {
		return h.sendError(c, err)
	}

	return c.JSON(fiber.Map{
		"message": "template retrieved successfully",
		"data":    template,
	})
}

// UpdateTemplate updates an existing template.
// PUT /templates/:id
func (h *TemplateHandler) UpdateTemplate(c *fiber.Ctx) error {
	idStr := c.Params("id")

	// Parse ID
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return h.sendError(c, apperror.Validation("invalid template ID"))
	}

	var req dto.CreateTemplateRequest

	// Parse request body
	if err := c.BodyParser(&req); err != nil {
		return h.sendError(c, apperror.Validation("invalid request body"))
	}

	// Call service
	resp, err := h.templateService.UpdateTemplate(c.Context(), uint(id), &req)
	if err != nil {
		return h.sendError(c, err)
	}

	h.logger.Info("template updated", "id", id, "name", req.Name)
	return c.JSON(fiber.Map{
		"message": "template updated successfully",
		"data":    resp,
	})
}

// DeleteTemplate removes a template.
// DELETE /templates/:id
func (h *TemplateHandler) DeleteTemplate(c *fiber.Ctx) error {
	idStr := c.Params("id")

	// Parse ID
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return h.sendError(c, apperror.Validation("invalid template ID"))
	}

	// Call service
	if err := h.templateService.DeleteTemplate(c.Context(), uint(id)); err != nil {
		return h.sendError(c, err)
	}

	h.logger.Info("template deleted", "id", id)
	return c.JSON(fiber.Map{
		"message": "template deleted successfully",
	})
}

// sendError sends an error response with appropriate status code.
func (h *TemplateHandler) sendError(c *fiber.Ctx, err error) error {
	var appErr *apperror.AppError
	statusCode := 500
	message := "An unexpected error occurred"

	if errors.As(err, &appErr) {
		statusCode = appErr.StatusCode
		message = appErr.Message
		h.logger.Error("handler error", "type", appErr.Type, "message", message, "error", appErr.Err)
	} else {
		h.logger.Error("handler error", "error", err)
	}

	errType := "internal_error"
	if apperror.StatusCode(err) != 500 {
		errType = string(apperror.Message(err))
	}
	return c.Status(statusCode).JSON(dto.ErrorResponse{
		Error:   errType,
		Message: message,
		Status:  statusCode,
	})
}
