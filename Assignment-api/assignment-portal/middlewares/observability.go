package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type requestMetric struct {
	method string
	route  string
	status int
	count  uint64
}

var requestMetrics = struct {
	sync.Mutex
	values map[string]requestMetric
}{values: make(map[string]requestMetric)}

func RequestObservability(db *sqlx.DB, logger *slog.Logger) fiber.Handler {
	tracer := otel.Tracer("assignment-portal/http")
	return func(c *fiber.Ctx) error {
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Set("X-Request-ID", requestID)
		parentContext := otel.GetTextMapPropagator().Extract(c.UserContext(), fiberHeaderCarrier{ctx: c})
		ctx, span := tracer.Start(parentContext, c.Method()+" "+c.Route().Path)
		start := time.Now()
		c.SetUserContext(ctx)
		err := c.Next()
		status := c.Response().StatusCode()
		route := c.Route().Path
		span.SetAttributes(
			attribute.String("http.request.method", c.Method()),
			attribute.String("http.route", route),
			attribute.Int("http.response.status_code", status),
			attribute.String("tenant.id", safeLocalString(c.Locals("tenantID"))),
		)
		if err != nil {
			span.RecordError(err)
		}
		span.End()
		incrementRequestMetric(c.Method(), route, status)
		if db != nil {
			auditCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_, auditErr := db.ExecContext(auditCtx, `INSERT INTO audit_logs (tenant_id, actor_id, action, resource, request_id, status_code) VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6)`,
				safeLocalString(c.Locals("tenantID")), safeLocalString(c.Locals("userID")),
				c.Method(), route, requestID, status)
			cancel()
			if auditErr != nil {
				logger.Error("audit event write failed", slog.String("request_id", requestID), slog.Any("error", auditErr))
			}
		}
		logger.Info("http request",
			slog.String("request_id", requestID),
			slog.String("method", c.Method()),
			slog.String("route", route),
			slog.Int("status", status),
			slog.Duration("duration", time.Since(start)),
		)
		return err
	}
}

type fiberHeaderCarrier struct {
	ctx *fiber.Ctx
}

func (h fiberHeaderCarrier) Get(key string) string { return h.ctx.Get(key) }
func (h fiberHeaderCarrier) Set(key, value string) { h.ctx.Set(key, value) }
func (h fiberHeaderCarrier) Keys() []string        { return []string{"traceparent", "tracestate", "baggage"} }

func MetricsHandler(c *fiber.Ctx) error {
	requestMetrics.Lock()
	defer requestMetrics.Unlock()
	keys := make([]string, 0, len(requestMetrics.values))
	for key := range requestMetrics.values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	c.Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = c.WriteString("# HELP assignment_portal_http_requests_total Total HTTP requests.\n# TYPE assignment_portal_http_requests_total counter\n")
	for _, key := range keys {
		metric := requestMetrics.values[key]
		_, _ = c.WriteString(fmt.Sprintf("assignment_portal_http_requests_total{method=%q,route=%q,status=%q} %d\n", metric.method, metric.route, strconv.Itoa(metric.status), metric.count))
	}
	return nil
}

func incrementRequestMetric(method, route string, status int) {
	key := strings.Join([]string{method, route, strconv.Itoa(status)}, "|")
	requestMetrics.Lock()
	metric := requestMetrics.values[key]
	metric.method, metric.route, metric.status = method, route, status
	metric.count++
	requestMetrics.values[key] = metric
	requestMetrics.Unlock()
}

func safeLocalString(value interface{}) string {
	result, _ := value.(string)
	return result
}
