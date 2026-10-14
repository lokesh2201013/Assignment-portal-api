package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/lokesh2201013/config"
	"github.com/lokesh2201013/controllers"
	"github.com/lokesh2201013/database"
	middleware "github.com/lokesh2201013/middlewares"
	"github.com/lokesh2201013/repositories"
	"github.com/lokesh2201013/routes"
	"github.com/lokesh2201013/search"
	"github.com/lokesh2201013/services"
	"github.com/lokesh2201013/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	slog.SetDefault(logger)
	cfg := config.Load()

	traceProvider, err := initJaegerTracerProvider("assignment-api")
	if err != nil {
		logger.Warn("failed to initialize Jaeger tracer; falling back to noop provider", slog.Any("error", err))
		traceProvider = sdktrace.NewTracerProvider()
	}
	otel.SetTracerProvider(traceProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	db, err := database.ConnectDB(cfg, logger)
	if err != nil {
		logger.Error("failed to connect database", slog.Any("error", err))
		os.Exit(1)
	}
	storageClient, err := storage.NewClient(context.Background())
	if err != nil {
		logger.Error("failed to initialize object storage client", slog.Any("error", err))
		os.Exit(1)
	}
	defer storageClient.Close()
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
	fileSaver := services.NewGCSFileSaver(storageClient, cfg.GCSBucketName, services.ClamAVScanner{Address: cfg.ClamAVAddress}, cfg.UploadMaxBytes, cfg.UploadRetention, cfg.GCSAccessID, cfg.GCSPrivateKey)
	userService := services.NewUserService(userRepo, jwtManager)
	tenantService := services.NewTenantService(db, jwtManager, notifier, cfg.PublicAppURL)
	assignmentService := services.NewAssignmentService(assignmentRepo, userRepo, notifier, fileSaver)
	aidService := services.NewAidService(cfg, db, videoRepo, search.NewVideoSearch(es, logger), database.NewRabbitPublisher(cfg))
	aidService.ConfigureUploadSecurity(storageClient, services.ClamAVScanner{Address: cfg.ClamAVAddress}, cfg.UploadMaxBytes)

	app := fiber.New(fiber.Config{
		BodyLimit: int(cfg.UploadMaxBytes*2 + 1<<20),
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			logger.Error("unhandled request error", slog.Any("error", err), slog.String("path", c.Path()))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Internal server error"})
		},
	})

	app.Use(recover.New())
	app.Use(middleware.RequestObservability(db, logger))
	registerAPIDocs(app)
	app.Get("/metrics", middleware.MetricsHandler)
	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.AllowedOrigins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
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
		Tenants:     controllers.NewTenantController(tenantService, logger),
		JWT:         jwtManager,
		Logger:      logger,
		DB:          db,
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
	if err := traceProvider.Shutdown(ctx); err != nil {
		logger.Error("trace provider shutdown failed", slog.Any("error", err))
	}
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
