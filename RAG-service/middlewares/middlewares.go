package middlewares

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
)

// SlogRequestLogger logs incoming HTTP requests using standard structured slog.
func SlogRequestLogger(logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		reqID := c.Locals("requestid")
		if reqID == nil {
			reqID = "none"
		}

		logger.Info("HTTP Request",
			slog.String("request_id", reqID.(string)),
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", c.Response().StatusCode()),
			slog.Duration("duration", time.Since(start)),
			slog.String("ip", c.IP()),
		)

		return err
	}
}

// SecurityHeaders adds standard HTTP security headers.
func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		return c.Next()
	}
}
