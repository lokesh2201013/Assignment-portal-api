package handler

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/lokesh2201013/email-service/internal/dto"
	"github.com/lokesh2201013/email-service/internal/errors/apperror"
	"github.com/lokesh2201013/email-service/internal/service"
)

// EmailHandler handles email sending and metrics endpoints.
type EmailHandler struct {
	emailService *service.EmailService
	logger       *slog.Logger
}

// NewEmailHandler creates a new email handler.
func NewEmailHandler(emailService *service.EmailService, logger *slog.Logger) *EmailHandler {
	return &EmailHandler{
		emailService: emailService,
		logger:       logger,
	}
}

// SendEmail sends emails through a verified sender identity.
// POST /emails/send
func (h *EmailHandler) SendEmail(c *fiber.Ctx) error {
	var req dto.SendEmailRequest

	// Parse request body
	if err := c.BodyParser(&req); err != nil {
		return h.sendError(c, apperror.Validation("invalid request body"))
	}

	// Call service
	resp, err := h.emailService.SendEmail(c.Context(), &req)
	if err != nil {
		return h.sendError(c, err)
	}

	h.logger.Info("emails sent", "from", req.From, "count", resp.Count)
	return c.JSON(fiber.Map{
		"message": resp.Message,
		"data":    resp,
	})
}

// GetMetrics retrieves analytics for a sender.
// GET /metrics/:senderEmail
func (h *EmailHandler) GetMetrics(c *fiber.Ctx) error {
	senderEmail := c.Params("senderEmail")

	// Call service
	resp, err := h.emailService.GetAnalytics(c.Context(), senderEmail)
	if err != nil {
		return h.sendError(c, err)
	}

	return c.JSON(fiber.Map{
		"message": "metrics retrieved successfully",
		"data":    resp,
	})
}

// GetAdminMetrics retrieves analytics for all senders of an admin.
// GET /metrics
func (h *EmailHandler) GetAdminMetrics(c *fiber.Ctx) error {
	// Get admin name from JWT claims
	adminName, ok := c.Locals("username").(string)
	if !ok {
		return h.sendError(c, apperror.Unauthorized("missing user context"))
	}

	// Call service
	resp, err := h.emailService.GetAdminAnalytics(c.Context(), adminName)
	if err != nil {
		return h.sendError(c, err)
	}

	return c.JSON(fiber.Map{
		"message": "admin metrics retrieved successfully",
		"data":    resp,
	})
}

// sendError sends an error response with appropriate status code.
func (h *EmailHandler) sendError(c *fiber.Ctx, err error) error {
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
