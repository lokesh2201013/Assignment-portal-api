package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/lokesh2201013/config"
	"github.com/lokesh2201013/controllers"
	"github.com/lokesh2201013/database"
	"github.com/lokesh2201013/repositories"
	"github.com/lokesh2201013/routes"
	"github.com/lokesh2201013/search"
	"github.com/lokesh2201013/services"
	"github.com/lokesh2201013/utils"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	slog.SetDefault(logger)
	cfg := config.Load()

	db, err := database.ConnectDB(cfg, logger)
	if err != nil {
		logger.Error("failed to connect database", slog.Any("error", err))
		os.Exit(1)
	}
	es, err := database.InitES(cfg)
	if err != nil {
		logger.Error("failed to initialize Elasticsearch client", slog.Any("error", err))
		os.Exit(1)
	}

	jwtManager := utils.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTTTL)
	userRepo := repositories.NewUserRepository(db)
	assignmentRepo := repositories.NewAssignmentRepository(db)
	videoRepo := repositories.NewVideoRepository(db)
	notifier := services.NewGRPCAssignmentNotifier(cfg.EmailServiceAddr)
	fileSaver := services.NewLocalFileSaver()
	userService := services.NewUserService(userRepo, jwtManager)
	assignmentService := services.NewAssignmentService(assignmentRepo, userRepo, notifier, fileSaver)
	aidService := services.NewAidService(cfg, db, videoRepo, search.NewVideoSearch(es, logger), database.NewRabbitPublisher(cfg))

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			logger.Error("unhandled request error", slog.Any("error", err), slog.String("path", c.Path()))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Internal server error"})
		},
	})

	app.Use(recover.New())
	app.Use(slogRequestLogger(logger))
	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.AllowedOrigins,
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	app.Use(limiter.New(limiter.Config{
		Max:        100,
		Expiration: 60 * time.Second,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many requests. Please try again later.",
			})
		},
	}))

	routes.AuthRoutes(app, routes.Controllers{
		Users:       controllers.NewUserController(userService, logger),
		Assignments: controllers.NewAssignmentController(assignmentService, logger, cfg.UploadImageDir, cfg.UploadFileDir),
		Aid:         controllers.NewAidController(aidService, logger),
		JWT:         jwtManager,
		Logger:      logger,
	})

	go func() {
		logger.Info("server starting", slog.String("addr", ":"+cfg.Port))
		if err := app.Listen(":" + cfg.Port); err != nil {
			logger.Error("server stopped", slog.Any("error", err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(ctx); err != nil {
		logger.Error("server shutdown failed", slog.Any("error", err))
	}
}

func slogRequestLogger(logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		logger.Info("http request",
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", c.Response().StatusCode()),
			slog.Duration("duration", time.Since(start)),
		)
		return err
	}
}
