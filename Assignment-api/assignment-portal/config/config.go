package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port             string
	DatabaseDSN      string
	JWTSecret        string
	JWTIssuer        string
	JWTTTL           time.Duration
	AllowedOrigins   string
	EmailServiceAddr string
	RAGServiceAddr   string
	OllamaURL        string
	ElasticAddresses []string
	RabbitMQURL      string
	UploadImageDir   string
	UploadFileDir    string
	GCSBucketName    string
	GCSPrivateKey    string
	GCSAccessID      string
}

func Load() Config {
	_ = godotenv.Load()

	return Config{
		Port:             env("PORT", "8080"),
		DatabaseDSN:      databaseDSN(),
		JWTSecret:        env("JWT_SECRET", "your-secret-key"),
		JWTIssuer:        env("JWT_ISSUER", "assignment-portal"),
		JWTTTL:           envDurationHours("JWT_TTL_HOURS", 24),
		AllowedOrigins:   env("ALLOWED_ORIGINS", "*"),
		EmailServiceAddr: env("EMAIL_SERVICE_ADDR", "localhost:50051"),
		RAGServiceAddr:   env("RAG_SERVICE_ADDR", "localhost:50052"),
		OllamaURL:        env("OLLAMA_URL", "http://localhost:11434/api/generate"),
		ElasticAddresses: splitEnv("ELASTIC_ADDRESSES", "http://localhost:9200"),
		RabbitMQURL:      env("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		UploadImageDir:   env("UPLOAD_IMAGE_DIR", "./uploads/images"),
		UploadFileDir:    env("UPLOAD_FILE_DIR", "./uploads/files"),
		GCSBucketName:    env("GCS_BUCKET_NAME", "my-video-bucket"),
		GCSPrivateKey:    os.Getenv("GCS_PRIVATE_KEY"),
		GCSAccessID:      env("GCS_ACCESS_ID", "your-service-account@project-id.iam.gserviceaccount.com"),
	}
}

func databaseDSN() string {
	if dsn := os.Getenv("DATABASE_DSN"); dsn != "" {
		return dsn
	}
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
		env("DB_SSLMODE", "disable"),
	)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envDurationHours(key string, fallback int) time.Duration {
	value, err := strconv.Atoi(env(key, strconv.Itoa(fallback)))
	if err != nil || value <= 0 {
		value = fallback
	}
	return time.Duration(value) * time.Hour
}

func splitEnv(key, fallback string) []string {
	raw := env(key, fallback)
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
