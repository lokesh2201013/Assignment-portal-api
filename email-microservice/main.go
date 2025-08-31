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
