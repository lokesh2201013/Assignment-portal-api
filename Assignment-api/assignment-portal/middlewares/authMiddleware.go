package middleware

import (
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/utils"
)

func AuthMiddleware(jwtManager *utils.JWTManager, logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenHeader := c.Get("Authorization")
		if tokenHeader == "" || !strings.HasPrefix(tokenHeader, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}

		claims, err := jwtManager.Parse(strings.TrimSpace(strings.TrimPrefix(tokenHeader, "Bearer ")))
		if err != nil {
			logger.Info("token validation failed", slog.String("path", c.Path()))
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}

		c.Locals("userID", claims.UserID)
		c.Locals("role", claims.Role)
		return c.Next()
	}
}

func AdminOnly(handler fiber.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if role := c.Locals("role"); role != "admin" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden: Admin access required"})
		}
		return handler(c)
	}
}

func UserOnly(handler fiber.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if role := c.Locals("role"); role != "user" && role != "admin" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden: User access required"})
		}
		return handler(c)
	}
}
