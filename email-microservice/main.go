package main

import (
	"fmt"
	"log"
	"net"
	"time"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
    "runtime"

	pb "github.com/lokesh2201013/email-service/proto"
	"google.golang.org/grpc"

	"github.com/lokesh2201013/email-service/database"
	"github.com/lokesh2201013/email-service/routes"
	"github.com/joho/godotenv"
)

var Log *zap.Logger

func initLogger() {
	var err error
	levelStr := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	var level zapcore.Level
	switch levelStr {
	case "DEBUG": level = zapcore.DebugLevel
	case "INFO":  level = zapcore.InfoLevel
	case "WARN":  level = zapcore.WarnLevel
	case "ERROR": level = zapcore.ErrorLevel
	default:      level = zapcore.InfoLevel
	}

	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(level)

	Log, err = config.Build()
	if err != nil {
		log.Fatalf("failed to initialize zap logger: %v", err)
	}
}

type emailServiceServer struct {
	pb.UnimplementedEmailServiceServer
}

func getCPUUsage() float64 {
	percentages, err := cpu.Percent(0, false)
	if err != nil {
		return 0
	}
	return percentages[0]
}

func setupMetrics() (*prometheus.CounterVec, prometheus.Gauge, prometheus.Histogram, prometheus.Summary) {
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "route"},
	)

	gauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "cpu_usage_percentage",
			Help: "Current CPU usage in percentage",
		},
	)

	histogram := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Histogram for request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	summary := prometheus.NewSummary(
		prometheus.SummaryOpts{
			Name:       "request_duration_seconds",
			Help:       "Summary of request durations",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
	)

	prometheus.MustRegister(counter, gauge, histogram, summary)

	go func() {
		for {
			gauge.Set(getCPUUsage())
			time.Sleep(5 * time.Second)
		}
	}()

	return counter, gauge, histogram, summary
}

func setupLoggingMiddleware(app *fiber.App) {
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		reqID := c.Locals("requestid")
		if reqID == nil {
			reqID = "none"
		}
		Log.Info("HTTP Request",
			zap.String("timestamp", time.Now().Format("15:04:05")),
			zap.Any("request_id", reqID),
			zap.Int("status", c.Response().StatusCode()),
			zap.Duration("duration", time.Since(start)),
			zap.String("ip", c.IP()),
			zap.String("method", c.Method()),
			zap.String("path", c.OriginalURL()),
		)
		return err
	})
}

func setupMetricsMiddleware(app *fiber.App, counter *prometheus.CounterVec, histogram prometheus.Histogram, summary prometheus.Summary) {
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
		Log.Fatal("Failed to listen for gRPC", zap.Error(err))
	}

	grpcServer := grpc.NewServer()
	pb.RegisterEmailServiceServer(grpcServer, &emailServiceServer{})

	Log.Info("gRPC Email Service is running on port 50051")
	if err := grpcServer.Serve(listener); err != nil {
		Log.Fatal("Failed to serve gRPC", zap.Error(err))
	}
	Log.Info("gRPC server stopped on port 50051")
}

func main() {
	runtime.GOMAXPROCS(2)
	err := godotenv.Load()
	
	initLogger()
	defer Log.Sync()

	if err != nil {
		Log.Warn("Warning: .env file not found, using system environment variables")
	}

	database.InitDB()

	go startGRPCServer()
 
	app := fiber.New()
	
	// Prepend RequestID Middleware
	app.Use(requestid.New())
	setupLoggingMiddleware(app)

	counter, _, histogram, summary := setupMetrics()
	setupMetricsMiddleware(app, counter, histogram, summary)
	
	setupMetricsRoute(app)
	routes.SetupRoutes(app)

	port := ":3000"
	Log.Info("HTTP Server listening", zap.String("port", port))
	if err := app.Listen(port); err != nil {
		Log.Fatal("Failed to start server", zap.Error(err))
	}
}
