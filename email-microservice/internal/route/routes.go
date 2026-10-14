// Package route defines HTTP route setup.
package route

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/lokesh2201013/email-service/internal/handler"
	"github.com/lokesh2201013/email-service/internal/middleware"
	"github.com/lokesh2201013/email-service/internal/service"
)

// Setup configures all routes with middleware and handlers.
func Setup(app *fiber.App, services Services, appLogger *slog.Logger) {
	// Global middleware
	app.Use(recover.New())
	app.Use(logger.New())

	// Create handlers
	authHandler := handler.NewAuthHandler(services.Auth, appLogger)
	senderHandler := handler.NewSenderHandler(services.Sender, appLogger)
	templateHandler := handler.NewTemplateHandler(services.Template, appLogger)
	emailHandler := handler.NewEmailHandler(services.Email, appLogger)

	// JWT middleware
	jwtMiddleware := middleware.NewJWTMiddleware(services.Auth, appLogger)

	// Public routes - no authentication required
	public := app.Group("/api/v1")
	{
		auth := public.Group("/auth")
		{
			auth.Post("/register", authHandler.Register)
			auth.Post("/login", authHandler.Login)
		}
	}

	// Protected routes - require authentication
	protected := app.Group("/api/v1", jwtMiddleware)
	{
		// Sender routes
		senders := protected.Group("/senders")
		{
			senders.Post("/verify", senderHandler.VerifyIdentity)
			senders.Get("/", senderHandler.ListVerified)
			senders.Delete("/:email", senderHandler.DeleteSender)
		}

		// Template routes
		templates := protected.Group("/templates")
		{
			templates.Post("/", templateHandler.CreateTemplate)
			templates.Get("/", templateHandler.ListTemplates)
			templates.Get("/:id", templateHandler.GetTemplate)
			templates.Put("/:id", templateHandler.UpdateTemplate)
			templates.Delete("/:id", templateHandler.DeleteTemplate)
		}

		// Email routes
		emails := protected.Group("/emails")
		{
			emails.Post("/send", emailHandler.SendEmail)
			emails.Get("/metrics", emailHandler.GetAdminMetrics)
			emails.Get("/metrics/:senderEmail", emailHandler.GetMetrics)
		}
	}

	// Health check - no auth required
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
}

// Services holds all service instances for dependency injection.
type Services struct {
	Auth      *service.AuthService
	Sender    *service.SenderService
	Template  *service.TemplateService
	Email     *service.EmailService
	Analytics *service.AnalyticsService
}
