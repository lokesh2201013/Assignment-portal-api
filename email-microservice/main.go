package main

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/lokesh2201013/email-service/internal/config"
	"github.com/lokesh2201013/email-service/internal/grpc/handler"
	"github.com/lokesh2201013/email-service/internal/repository"
	"github.com/lokesh2201013/email-service/internal/route"
	"github.com/lokesh2201013/email-service/internal/service"
	"github.com/lokesh2201013/email-service/internal/storage"
	pb "github.com/lokesh2201013/email-service/proto"
)

func main() {
	if err := run(); err != nil {
		slog.Error("email service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if err := godotenv.Load(); err != nil {
		slog.Info(".env file not found; using process environment")
	}

	logger := initLogger()
	logger.Info("starting email service")

	cfg := config.Load()
	logger.Info("configuration loaded", "environment", cfg.Environment, "log_level", cfg.LogLevel)

	db, err := storage.Connect(context.Background(), cfg.DSN(), logger)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer storage.Close(db)

	senderRepo := repository.NewPostgresSenderRepository(db, cfg.SMTPCredentialsKey)
	if err := senderRepo.MigrateCredentials(context.Background()); err != nil {
		return fmt.Errorf("SMTP credential migration failed: %w", err)
	}

	repos := &repository.Repositories{
		User:         repository.NewPostgresUserRepository(db),
		Sender:       senderRepo,
		Template:     repository.NewPostgresTemplateRepository(db),
		Analytics:    repository.NewPostgresAnalyticsRepository(db),
		EmailMessage: repository.NewPostgresEmailMessageRepository(db),
	}
	services := &route.Services{
		Auth:      service.NewAuthService(repos.User, cfg, logger),
		Sender:    service.NewSenderService(repos.Sender, logger),
		Template:  service.NewTemplateService(repos.Template, logger),
		Email:     service.NewEmailService(repos.Sender, repos.Analytics, repos.EmailMessage, cfg, logger),
		Analytics: service.NewAnalyticsService(repos.Analytics, repos.Sender, logger),
	}

	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("failed to listen for gRPC on port %s: %w", cfg.GRPCPort, err)
	}
	defer grpcListener.Close()

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(internalGRPCAuthInterceptor(cfg.InternalGRPCToken)))
	pb.RegisterEmailServiceServer(grpcServer, handler.NewEmailServiceServer(services.Email, logger))

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			logger.Error("unhandled HTTP error", "error", err, "path", c.Path())
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "internal_error",
				"message": "an unexpected error occurred",
				"status":  fiber.StatusInternalServerError,
			})
		},
	})
	route.Setup(app, *services, logger)
	app.Get("/metrics", func(c *fiber.Ctx) error {
		fasthttpadaptor.NewFastHTTPHandler(promhttp.Handler())(c.Context())
		return nil
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	workerCtx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	go services.Email.StartQueueWorker(workerCtx)

	errs := make(chan error, 2)
	go func() {
		logger.Info("HTTP server starting", "address", ":"+cfg.HTTPPort)
		errs <- app.Listen(":" + cfg.HTTPPort)
	}()
	go func() {
		logger.Info("gRPC server starting", "address", grpcListener.Addr().String())
		errs <- grpcServer.Serve(grpcListener)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
		stopWorker()
		if err := app.Shutdown(); err != nil {
			logger.Error("HTTP shutdown failed", "error", err)
		}
		grpcServer.GracefulStop()
		return nil
	case err := <-errs:
		stopWorker()
		_ = app.Shutdown()
		grpcServer.Stop()
		if err != nil && !strings.Contains(err.Error(), "Server closed") {
			return fmt.Errorf("HTTP or gRPC server stopped unexpectedly: %w", err)
		}
		return fmt.Errorf("HTTP or gRPC server stopped unexpectedly")
	}
}

func initLogger() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func internalGRPCAuthInterceptor(expectedToken string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if len(expectedToken) < 32 {
			return nil, status.Error(codes.Unauthenticated, "internal authorization required")
		}
		values, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "internal authorization required")
		}
		provided := values.Get("x-internal-token")
		if len(provided) != 1 || subtle.ConstantTimeCompare([]byte(provided[0]), []byte(expectedToken)) != 1 {
			return nil, status.Error(codes.Unauthenticated, "internal authorization required")
		}
		return handler(ctx, req)
	}
}
