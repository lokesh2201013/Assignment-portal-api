package middleware

import (
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jmoiron/sqlx"
	"github.com/lokesh2201013/utils"
)

func AuthMiddleware(jwtManager *utils.JWTManager, logger *slog.Logger, databases ...*sqlx.DB) fiber.Handler {
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
		if len(databases) > 0 && databases[0] != nil {
			var verified bool
			if err := databases[0].Get(&verified, `SELECT EXISTS (SELECT 1 FROM tenant_memberships WHERE tenant_id = $1 AND user_id = $2 AND role = $3 AND status = 'verified')`, claims.TenantID, claims.UserID, claims.Role); err != nil || !verified {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Tenant membership is not active"})
			}
		}

		c.Locals("userID", claims.UserID)
		c.Locals("role", claims.Role)
		c.Locals("tenantID", claims.TenantID)
		c.SetUserContext(utils.WithTenantID(c.UserContext(), claims.TenantID))
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
