package main

import (
	//"fmt"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/valyala/fasthttp/fasthttpadaptor"
     "runtime"
	pb "github.com/lokesh2201013/email-service/proto"
	"google.golang.org/grpc"

	"github.com/lokesh2201013/email-service/database"
	"github.com/lokesh2201013/email-service/routes"
	  "github.com/joho/godotenv"
)

type emailServiceServer struct {
	pb.UnimplementedEmailServiceServer
}

func getCPUUsage() float64 {
	percentages, err := cpu.Percent(0, false)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer storage.Close(db)
	logger.Info("database connected")

	// Initialize repositories
	repos := &repository.Repositories{
		User:         repository.NewPostgresUserRepository(db),
		Sender:       repository.NewPostgresSenderRepository(db),
		Template:     repository.NewPostgresTemplateRepository(db),
		Analytics:    repository.NewPostgresAnalyticsRepository(db),
		EmailMessage: repository.NewPostgresEmailMessageRepository(db),
	}
	logger.Info("repositories initialized")

	// Initialize services
	services := &route.Services{
		Auth:      service.NewAuthService(repos.User, cfg, logger),
		Sender:    service.NewSenderService(repos.Sender, logger),
		Template:  service.NewTemplateService(repos.Template, logger),
		Email:     service.NewEmailService(repos.Sender, repos.Analytics, repos.EmailMessage, cfg, logger),
		Analytics: service.NewAnalyticsService(repos.Analytics, repos.Sender, logger),
	}
	logger.Info("services initialized")

	// Start background email queue worker
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	go services.Email.StartQueueWorker(workerCtx)

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

	// Setup routes
	route.Setup(app, *services, logger)

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for {
			gauge.Set(getCPUUsage())
			time.Sleep(5 * time.Second)
		}
	}()

	return counter, gauge, histogram, summary
}


func setupMiddleware(app *fiber.App, counter *prometheus.CounterVec, histogram prometheus.Histogram, summary prometheus.Summary) {
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()
		counter.WithLabelValues(c.Method(), c.Path()).Inc()
		err := c.Next()
		duration := time.Since(start).Seconds()
		histogram.Observe(duration)
		summary.Observe(duration)
		return err
	})
}

func setupMetricsRoute(app *fiber.App) {
	app.Get("/metrics", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/plain")
		handler := promhttp.Handler()
		fasthttpadaptor.NewFastHTTPHandler(handler)(c.Context())
		return nil
	})
}

func startGRPCServer() {
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterEmailServiceServer(grpcServer, &emailServiceServer{})

	log.Println("gRPC Email Service is running on port 50051")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
	fmt.Println("gRPC server stopped on port 50051")
	grpcServer.Stop()
}

func main() {
	runtime.GOMAXPROCS(2)
	  err := godotenv.Load()
    if err != nil {
        log.Println("Warning: .env file not found, using system environment variables")
        // Don't exit - continue with system env vars
    }
	database.InitDB()

	go startGRPCServer()
 
	app := fiber.New()
	counter, _, histogram, summary := setupMetrics()
    
	setupMiddleware(app, counter, histogram, summary)
	setupMetricsRoute(app)
	routes.SetupRoutes(app)

	log.Fatal(app.Listen(":3000"))
}
