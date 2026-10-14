package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"

	"github.com/lokesh2201013/email-service/internal/config"
	"github.com/lokesh2201013/email-service/internal/grpc/handler"
	"github.com/lokesh2201013/email-service/internal/repository"
	"github.com/lokesh2201013/email-service/internal/route"
	"github.com/lokesh2201013/email-service/internal/service"
	"github.com/lokesh2201013/email-service/internal/storage"
	pb "github.com/lokesh2201013/email-service/proto"
)

func main() {
	runtime.GOMAXPROCS(2)

	// Load .env file
	_ = godotenv.Load()

	// Initialize logger
	logger := initLogger()
	logger.Info("starting email service")

	traceProvider, err := initJaegerTracerProvider("email-microservice")
	if err != nil {
		logger.Warn("failed to initialize Jaeger tracer; continuing without export", slog.Any("error", err))
		traceProvider = sdktrace.NewTracerProvider()
	}
	otel.SetTracerProvider(traceProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := traceProvider.Shutdown(ctx); err != nil {
			logger.Warn("trace provider shutdown failed", slog.Any("error", err))
		}
	}()

	// Load configuration
	cfg := config.Load()
	logger.Info("configuration loaded", "environment", cfg.Environment, "log_level", cfg.LogLevel)

	// Initialize database
	ctx := context.Background()
	db, err := storage.Connect(ctx, cfg.DSN(), logger)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer storage.Close(db)
	logger.Info("database connected")

	// Initialize repositories
	repos := &repository.Repositories{
		User:      repository.NewPostgresUserRepository(db),
		Sender:    repository.NewPostgresSenderRepository(db),
		Template:  repository.NewPostgresTemplateRepository(db),
		Analytics: repository.NewPostgresAnalyticsRepository(db),
	}
	logger.Info("repositories initialized")

	// Initialize services
	services := &route.Services{
		Auth:      service.NewAuthService(repos.User, cfg, logger),
		Sender:    service.NewSenderService(repos.Sender, logger),
		Template:  service.NewTemplateService(repos.Template, logger),
		Email:     service.NewEmailService(repos.Sender, repos.Analytics, cfg, logger),
		Analytics: service.NewAnalyticsService(repos.Analytics, repos.Sender, logger),
	}
	logger.Info("services initialized")

	// Start gRPC server in background
	go startGRPCServer(cfg, handler.NewEmailServiceServer(services.Email, cfg.SMTPFrom, logger), logger)

	// Start HTTP server
	startHTTPServer(cfg, services, logger)
}

func initLogger() *slog.Logger {
	logLevel := slog.LevelInfo
	if levelStr := os.Getenv("LOG_LEVEL"); levelStr != "" {
		switch levelStr {
		case "debug", "DEBUG":
			logLevel = slog.LevelDebug
		case "warn", "WARN":
			logLevel = slog.LevelWarn
		case "error", "ERROR":
			logLevel = slog.LevelError
		}
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	return slog.New(handler)
}

func startHTTPServer(cfg *config.Config, services *route.Services, logger *slog.Logger) {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			logger.Error("unhandled error", "error", err, "path", c.Path())
			return c.Status(500).JSON(fiber.Map{
				"error":   "internal_error",
				"message": "an unexpected error occurred",
				"status":  500,
			})
		},
	})

	app.Use(requestTracingMiddleware())
	registerAPIDocs(app)

	// Setup routes
	route.Setup(app, *services, logger)

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Info("shutting down HTTP server")
		_ = app.Shutdown()
	}()

	// Start server
	addr := fmt.Sprintf(":%s", cfg.HTTPPort)
	logger.Info("HTTP server starting", "address", addr)
	if err := app.Listen(addr); err != nil {
		// Shutdown is not an error, it's expected
		if err.Error() != "Server closed" {
			logger.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}
	logger.Info("HTTP server stopped")
}

func startGRPCServer(cfg *config.Config, emailServiceServer pb.EmailServiceServer, logger *slog.Logger) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err != nil {
		logger.Error("failed to listen for gRPC", "error", err)
		return
	}
	defer listener.Close()

	grpcServer := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
	pb.RegisterEmailServiceServer(grpcServer, emailServiceServer)

	logger.Info("gRPC server starting", "port", cfg.GRPCPort)
	if err := grpcServer.Serve(listener); err != nil {
		logger.Error("failed to serve gRPC", "error", err)
	}
	logger.Info("gRPC server stopped")
}

func initJaegerTracerProvider(serviceName string) (*sdktrace.TracerProvider, error) {
	endpoint := strings.TrimSpace(os.Getenv("JAEGER_ENDPOINT"))
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	}
	if endpoint == "" {
		endpoint = "http://localhost:4318"
	}

	exporter, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithEndpointURL(strings.TrimRight(endpoint, "/")+"/v1/traces"),
	)
	if err != nil {
		return nil, err
	}

	serviceResource := resource.NewWithAttributes("", attribute.String("service.name", serviceName))
	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(serviceResource),
	), nil
}

func requestTracingMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		tracer := otel.Tracer("email-service/http")
		parentContext := otel.GetTextMapPropagator().Extract(c.UserContext(), fiberHeaderCarrier{ctx: c})
		ctx, span := tracer.Start(parentContext, c.Method()+" "+c.Path())
		c.SetUserContext(ctx)
		defer span.End()

		if err := c.Next(); err != nil {
			span.RecordError(err)
			return err
		}

		span.SetAttributes(
			attribute.String("http.method", c.Method()),
			attribute.String("http.route", c.Route().Path),
			attribute.Int("http.status_code", c.Response().StatusCode()),
		)
		return nil
	}
}

type fiberHeaderCarrier struct {
	ctx *fiber.Ctx
}

func (h fiberHeaderCarrier) Get(key string) string { return h.ctx.Get(key) }
func (h fiberHeaderCarrier) Set(key, value string) { h.ctx.Set(key, value) }
func (h fiberHeaderCarrier) Keys() []string        { return []string{"traceparent", "tracestate", "baggage"} }
