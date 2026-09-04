package handler

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/lokesh2201013/email-service/internal/dto"
	"github.com/lokesh2201013/email-service/internal/errors/apperror"
	"github.com/lokesh2201013/email-service/internal/service"
)

// SenderHandler handles email sender endpoints.
type SenderHandler struct {
	senderService *service.SenderService
	logger        *slog.Logger
}

// NewSenderHandler creates a new sender handler.
func NewSenderHandler(senderService *service.SenderService, logger *slog.Logger) *SenderHandler {
	return &SenderHandler{
		senderService: senderService,
		logger:        logger,
	}
}

// VerifyIdentity verifies and creates a new email sender identity.
// POST /senders/verify
func (h *SenderHandler) VerifyIdentity(c *fiber.Ctx) error {
	var req dto.VerifyEmailRequest

	// Parse request body
	if err := c.BodyParser(&req); err != nil {
		return h.sendError(c, apperror.Validation("invalid request body"))
	}

	// Get admin name from JWT claims
	adminName, ok := c.Locals("username").(string)
	if !ok {
		return h.sendError(c, apperror.Unauthorized("missing user context"))
	}

	// Call service
	resp, err := h.senderService.VerifyIdentity(c.Context(), &req, adminName)
	if err != nil {
		return h.sendError(c, err)
	}

	h.logger.Info("sender verified", "email", req.Email, "admin", adminName)
	return c.Status(201).JSON(fiber.Map{
		"message": "sender identity verified successfully",
		"data":    resp,
	})
}

// ListVerified lists all verified sender identities for the authenticated admin.
// GET /senders
func (h *SenderHandler) ListVerified(c *fiber.Ctx) error {
	// Get admin name from JWT claims
	adminName, ok := c.Locals("username").(string)
	if !ok {
		return h.sendError(c, apperror.Unauthorized("missing user context"))
	}

	// Call service
	senders, err := h.senderService.ListVerifiedSenders(c.Context(), adminName)
	if err != nil {
		return h.sendError(c, err)
	}

	return c.JSON(fiber.Map{
		"message": "senders retrieved successfully",
		"data":    senders,
	})
}

// DeleteSender removes a sender identity.
// DELETE /senders/:email
func (h *SenderHandler) DeleteSender(c *fiber.Ctx) error {
	email := c.Params("email")

	// Get admin name from JWT claims
	adminName, ok := c.Locals("username").(string)
	if !ok {
		return h.sendError(c, apperror.Unauthorized("missing user context"))
	}

	// Call service
	if err := h.senderService.DeleteSender(c.Context(), email, adminName); err != nil {
		return h.sendError(c, err)
	}

	h.logger.Info("sender deleted", "email", email, "admin", adminName)
	return c.JSON(fiber.Map{
		"message": "sender deleted successfully",
	})
}

// sendError sends an error response with appropriate status code.
func (h *SenderHandler) sendError(c *fiber.Ctx, err error) error {
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
