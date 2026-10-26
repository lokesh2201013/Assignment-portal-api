package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/lokesh2201013/email-service/internal/credentials"
)

// Config holds all application configuration from environment variables.
type Config struct {
	// Server
	HTTPPort string
	GRPCPort string

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// JWT
	JWTSecret string
	JWTExpiry time.Duration

	// SMTP
	SMTPHost           string
	SMTPPort           int
	SMTPUsername       string
	SMTPPassword       string
	SMTPFrom           string
	SMTPCredentialsKey []byte
	InternalGRPCToken  string

	// Queue and Retry
	EmailMaxRetries      int
	EmailWorkerInterval  time.Duration
	EmailWorkerBatchSize int

	// Logging
	LogLevel string

	// Env
	Environment string
}

// Load reads environment variables and returns a Config.
// Panics if required variables are missing.
func Load() *Config {
	credentialsKey, err := credentials.ParseKey(getEnvRequired("SMTP_CREDENTIALS_KEY"))
	if err != nil {
		panic(err)
	}

	return &Config{
		HTTPPort:             getEnv("HTTP_PORT", "3000"),
		GRPCPort:             getEnv("GRPC_PORT", "50051"),
		DBHost:               getEnvRequired("DB_HOST"),
		DBPort:               getEnv("DB_PORT", "5432"),
		DBUser:               getEnvRequired("DB_USER"),
		DBPassword:           getEnvRequired("DB_PASSWORD"),
		DBName:               getEnvRequired("DB_NAME"),
		DBSSLMode:            getEnv("DB_SSLMODE", "require"),
		JWTSecret:            getEnvRequired("JWT_SECRET"),
		JWTExpiry:            getDurationEnv("JWT_EXPIRY", 24*time.Hour),
		SMTPHost:             getEnvRequired("SMTP_HOST"),
		SMTPPort:             getIntEnv("SMTP_PORT", 587),
		SMTPUsername:         getEnvRequired("SMTP_USERNAME"),
		SMTPPassword:         getEnvRequired("SMTP_PASSWORD"),
		SMTPFrom:             getEnvRequired("SMTP_FROM"),
		SMTPCredentialsKey:   credentialsKey,
		InternalGRPCToken:    getInternalGRPCToken(),
		EmailMaxRetries:      getIntEnv("EMAIL_MAX_RETRIES", 3),
		EmailWorkerInterval:  getDurationEnv("EMAIL_WORKER_INTERVAL", 2*time.Second),
		EmailWorkerBatchSize: getIntEnv("EMAIL_WORKER_BATCH_SIZE", 10),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		Environment:          getEnv("ENVIRONMENT", "production"),
	}
}

func getInternalGRPCToken() string {
	token := getEnvRequired("INTERNAL_GRPC_TOKEN")
	if len(token) < 32 {
		panic("INTERNAL_GRPC_TOKEN must contain at least 32 characters")
	}
	return token
}

// DSN returns the PostgreSQL connection string.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvRequired(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("required environment variable %q not set", key))
	}
	return value
}

func getIntEnv(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intVal, err := strconv.Atoi(value)
	if err != nil {
		panic(fmt.Sprintf("invalid integer for environment variable %q: %v", key, err))
	}
	return intVal
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		panic(fmt.Sprintf("invalid duration for environment variable %q: %v", key, err))
	}
	return duration
}
