package middleware

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/utils"
)

func TestAuthMiddlewareRejectsMissingBearerToken(t *testing.T) {
	app := fiber.New()
	app.Use(AuthMiddleware(utils.NewJWTManager("secret", "test", time.Hour), slog.New(slog.NewTextHandler(io.Discard, nil))))
	app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))

	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", fiber.StatusUnauthorized, resp.StatusCode)
	}
}

func TestAuthMiddlewareSetsUserLocals(t *testing.T) {
	manager := utils.NewJWTManager("secret", "test", time.Hour)
	token, err := manager.Generate("user-1", "admin")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	app := fiber.New()
	app.Use(AuthMiddleware(manager, slog.New(slog.NewTextHandler(io.Discard, nil))))
	app.Get("/", func(c *fiber.Ctx) error {
		if c.Locals("userID") != "user-1" || c.Locals("role") != "admin" {
			t.Fatalf("unexpected locals: %v %v", c.Locals("userID"), c.Locals("role"))
		}
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)

	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected %d, got %d", fiber.StatusOK, resp.StatusCode)
	}
}
