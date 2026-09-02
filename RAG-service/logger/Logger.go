package logger

import (
	"os"
	"time"
	"strings"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.Logger

func InitLogger() {
	var err error
	
	// Read LOG_LEVEL from environment
	levelStr := strings.ToUpper(os.Getenv("LOG_LEVEL"))
	var level zapcore.Level
	switch levelStr {
	case "DEBUG":
		level = zapcore.DebugLevel
	case "INFO":
		level = zapcore.InfoLevel
	case "WARN":
		level = zapcore.WarnLevel
	case "ERROR":
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel // default to info
	}

	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(level)

	Log, err = config.Build()
	if err != nil {
		panic("failed to initialize zap logger: " + err.Error())
	}
	defer Log.Sync()
}

func ZapLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Proceed to the next middleware
		err := c.Next()

		// Retrieve Request ID (added by requestid middleware)
		reqID := c.Locals("requestid")
		if reqID == nil {
			reqID = "none"
		}

		// Log request and response info
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
	}
}
