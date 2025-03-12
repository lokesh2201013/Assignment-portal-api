package main

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/valyala/fasthttp/fasthttpadaptor"

	"github.com/lokesh2201013/email-service/database"
	"github.com/lokesh2201013/email-service/routes"
)

func getCPUUsage() float64 {
	percentages, err := cpu.Percent(0, false) // Get CPU usage of all cores as a single percentage
	if err != nil {
		return 0
	}
	return percentages[0]
}

func main() {
	database.InitDB()
	app := fiber.New()

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

	// goroutine to update CPU usage periodically
	go func() {
		for {
			gauge.Set(getCPUUsage()) // Update CPU usage every 5 sec
			time.Sleep(5 * time.Second) 
		}
	}()

	// Middleware for Metrics Tracking
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()

		counter.WithLabelValues(c.Method(), c.Path()).Inc() // Count requests

		err := c.Next()

		duration := time.Since(start).Seconds()
		histogram.Observe(duration) // Observe request duration
		summary.Observe(duration)

		return err
	})

	app.Get("/metrics", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/plain")
		handler := promhttp.Handler()

		fasthttpadaptor.NewFastHTTPHandler(handler)(c.Context())

		return nil
	})

	routes.SetupRoutes(app)

	log.Fatal(app.Listen(":3000"))
}
