package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/atgsgrouptest/genet-microservice/RAG-service/config"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/controllers"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/middlewares"
	pb "github.com/atgsgrouptest/genet-microservice/RAG-service/proto"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/repositories"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/routes"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/services"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
)

func main() {
	// 1. Load .env if present
	_ = godotenv.Load()

	// 2. Load typed configuration
	cfg := config.Load()

	// 3. Initialize structured slog logger
	var level slog.Level
	switch strings.ToUpper(cfg.LogLevel) {
	case "DEBUG":
		level = slog.LevelDebug
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)

	logger.Info("Starting RAG Microservice",
		slog.String("http_port", cfg.Port),
		slog.String("grpc_port", cfg.GRPCPort),
		slog.String("qdrant_url", cfg.QdrantURL),
		slog.String("ollama_url", cfg.OllamaURL),
		slog.String("embedding_model", cfg.EmbeddingModel),
		slog.String("llm_model", cfg.LLMModel),
	)

	// 4. Initialize HTTP client with connection pooling and timeouts
	httpClient := &http.Client{
		Timeout: cfg.HTTPTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// 5. Dependency Injection: Repositories & Clients
	vectorRepo := repositories.NewQdrantRepository(cfg.QdrantURL, httpClient, logger)
	aiClient := services.NewOllamaClient(cfg.OllamaURL, cfg.EmbeddingModel, cfg.LLMModel, httpClient, logger)
	chunker := services.NewDocumentChunker(cfg.ChunkSize, cfg.ChunkOverlap)

	// 6. Services & Controllers
	ragService := services.NewRAGService(vectorRepo, aiClient, chunker, cfg.QdrantCollection, cfg.TopK, logger)
	httpController := controllers.NewRAGHTTPController(ragService, logger, cfg.MaxFileSize)
	grpcServerImpl := controllers.NewRAGGRPCServer(ragService, logger)

	// 7. Start gRPC Server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err != nil {
		logger.Error("Failed to listen on gRPC port", slog.String("port", cfg.GRPCPort), slog.Any("error", err))
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterRAGServiceServer(grpcServer, grpcServerImpl)

	go func() {
		logger.Info("gRPC server listening", slog.String("addr", lis.Addr().String()))
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			logger.Error("gRPC server terminated unexpectedly", slog.Any("error", err))
		}
	}()

	// 8. Configure Fiber HTTP Server
	app := fiber.New(fiber.Config{
		BodyLimit: int(cfg.MaxFileSize),
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			logger.Error("Unhandled request error",
				slog.String("path", c.Path()),
				slog.Any("error", err),
			)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Internal server error",
			})
		},
	})

	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(middlewares.SecurityHeaders())
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
	app.Use(middlewares.SlogRequestLogger(logger))

	// 9. Register HTTP Routes
	routes.RegisterRoutes(app, httpController)

	// 10. Start HTTP Server in Goroutine
	go func() {
		logger.Info("HTTP server listening", slog.String("port", cfg.Port))
		if err := app.Listen(":" + cfg.Port); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server stopped unexpectedly", slog.Any("error", err))
		}
	}()

	// 11. Graceful Shutdown on OS Signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	sig := <-quit

	logger.Info("Shutdown signal received, shutting down gracefully...", slog.String("signal", sig.String()))

	// Gracefully stop gRPC server
	grpcServer.GracefulStop()
	logger.Info("gRPC server stopped cleanly")

	// Gracefully shutdown Fiber HTTP server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		logger.Error("Fiber HTTP server shutdown failed", slog.Any("error", err))
	} else {
		logger.Info("Fiber HTTP server stopped cleanly")
	}

	logger.Info("All servers stopped. Exiting.")
}
