package logger

import (
	"time"
	"go.uber.org/zap"
	"github.com/gofiber/fiber/v2"

)

var Log *zap.Logger

func InitLogger() {
	var err error
	Log, err = zap.NewProduction()
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

		// Log request and response info
		Log.Info("HTTP Request",
			zap.String("timestamp", time.Now().Format("15:04:05")),
			zap.Int("status", c.Response().StatusCode()),
			zap.Duration("duration", time.Since(start)),
			zap.String("ip", c.IP()),
			zap.String("method", c.Method()),
			zap.String("path", c.OriginalURL()),
		)

		return err
	}
}
