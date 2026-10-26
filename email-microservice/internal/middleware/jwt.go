// Package middleware provides HTTP middleware functions.
package middleware

import (
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/lokesh2201013/email-service/internal/errors/apperror"
	"github.com/lokesh2201013/email-service/internal/service"
)

// NewJWTMiddleware creates JWT authentication middleware.
func NewJWTMiddleware(authService *service.AuthService, logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get Authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return sendError(c, apperror.Unauthorized("missing authorization header"), logger)
		}

		// Extract token from "Bearer <token>" format
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return sendError(c, apperror.Unauthorized("invalid authorization header format"), logger)
		}

		token := parts[1]

		// Verify token
		username, err := authService.VerifyToken(token)
		if err != nil {
			return sendError(c, err, logger)
		}

		// Store username in context for downstream handlers
		c.Locals("username", username)

		return c.Next()
	}
}

// sendError sends an error response.
func sendError(c *fiber.Ctx, err error, logger *slog.Logger) error {
	statusCode := apperror.StatusCode(err)
	message := apperror.Message(err)

	logger.Warn("middleware error", "status", statusCode, "message", message)

	return c.Status(statusCode).JSON(fiber.Map{
		"error":   "unauthorized",
		"message": message,
		"status":  statusCode,
	})
}
