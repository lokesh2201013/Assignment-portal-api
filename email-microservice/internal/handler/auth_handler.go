// Package handler contains HTTP request handlers.
package handler

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/lokesh2201013/email-service/internal/dto"
	"github.com/lokesh2201013/email-service/internal/errors/apperror"
	"github.com/lokesh2201013/email-service/internal/service"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	authService *service.AuthService
	logger      *slog.Logger
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(authService *service.AuthService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		logger:      logger,
	}
}

// Register handles user registration.
// POST /auth/register
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req dto.RegisterRequest

	// Parse request body
	if err := c.BodyParser(&req); err != nil {
		return h.sendError(c, apperror.Validation("invalid request body"))
	}

	// Call service
	resp, err := h.authService.Register(c.Context(), &req)
	if err != nil {
		return h.sendError(c, err)
	}

	h.logger.Info("user registration successful", "username", req.Username)
	return c.Status(201).JSON(fiber.Map{
		"message": "user registered successfully",
		"data":    resp,
	})
}

// Login handles user authentication.
// POST /auth/login
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req dto.LoginRequest

	// Parse request body
	if err := c.BodyParser(&req); err != nil {
		return h.sendError(c, apperror.Validation("invalid request body"))
	}

	// Call service
	resp, err := h.authService.Login(c.Context(), &req)
	if err != nil {
		return h.sendError(c, err)
	}

	h.logger.Info("user login successful", "username", req.Username)
	return c.JSON(fiber.Map{
		"message": "login successful",
		"data":    resp,
	})
}

// sendError sends an error response with appropriate status code.
func (h *AuthHandler) sendError(c *fiber.Ctx, err error) error {
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
