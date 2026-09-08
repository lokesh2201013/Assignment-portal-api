package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration settings for the RAG service.
type Config struct {
	Port             string
	GRPCPort         string
	OllamaURL        string
	QdrantURL        string
	EmbeddingModel   string
	LLMModel         string
	QdrantCollection string
	ChunkSize        int
	ChunkOverlap     int
	TopK             int
	AllowedOrigins   string
	MaxFileSize      int64
	LogLevel         string
	HTTPTimeout      time.Duration
	DatabaseDSN      string
	DefaultTenantID  string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Port:             getEnv("APP_PORT", "8001"),
		GRPCPort:         getEnv("GRPC_PORT", "50052"),
		OllamaURL:        getEnv("OLLAMA_URL", "http://localhost:11434"),
		QdrantURL:        getEnv("QDRANT_URL", "http://localhost:6333"),
		EmbeddingModel:   getEnv("EMBEDDING_MODEL", "nomic-embed-text"),
		LLMModel:         getEnv("LLM_MODEL", "llama3.1:8b"),
		QdrantCollection: getEnv("QDRANT_COLLECTION", "rag-corpus"),
		ChunkSize:        getEnvAsInt("CHUNK_SIZE", 250),
		ChunkOverlap:     getEnvAsInt("CHUNK_OVERLAP", 20),
		TopK:             getEnvAsInt("TOP_K", 5),
		AllowedOrigins:   getEnv("ALLOWED_ORIGINS", "*"),
		MaxFileSize:      getEnvAsInt64("MAX_FILE_SIZE_BYTES", 20*1024*1024), // 20 MB
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		HTTPTimeout:      time.Duration(getEnvAsInt("HTTP_TIMEOUT_SEC", 30)) * time.Second,
		DatabaseDSN:      getEnv("DATABASE_DSN", getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/assignment_portal?sslmode=disable")),
		DefaultTenantID:  getEnv("DEFAULT_TENANT_ID", "default"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

func getEnvAsInt64(key string, defaultVal int64) int64 {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseInt(valStr, 10, 64)
	if err != nil {
		return defaultVal
	}
	return val
}
