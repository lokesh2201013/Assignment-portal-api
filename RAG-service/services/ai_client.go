package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/atgsgrouptest/genet-microservice/RAG-service/apperrors"
)

// AIClient defines the interface for embedding generation and LLM text completion.
type AIClient interface {
	GetEmbedding(ctx context.Context, text string) ([]float64, error)
	GenerateAnswer(ctx context.Context, contextText, question string) (string, error)
}

// OllamaClient implements AIClient using Ollama's HTTP API.
type OllamaClient struct {
	baseURL        string
	embeddingModel string
	llmModel       string
	httpClient     *http.Client
	logger         *slog.Logger
}

// NewOllamaClient initializes a new OllamaClient.
func NewOllamaClient(baseURL, embeddingModel, llmModel string, httpClient *http.Client, logger *slog.Logger) *OllamaClient {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &OllamaClient{
		baseURL:        strings.TrimRight(baseURL, "/"),
		embeddingModel: embeddingModel,
		llmModel:       llmModel,
		httpClient:     httpClient,
		logger:         logger,
	}
}

type ollamaEmbeddingReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbeddingResp struct {
	Embedding []float64 `json:"embedding"`
}

// GetEmbedding generates a vector embedding for the given text using Ollama.
func (c *OllamaClient) GetEmbedding(ctx context.Context, text string) ([]float64, error) {
	payload := ollamaEmbeddingReq{
		Model:  c.embeddingModel,
		Prompt: text,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternal, "Failed to encode embedding payload", err)
	}

	url := fmt.Sprintf("%s/api/embeddings", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternal, "Failed to create embedding HTTP request", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrExternal, "Failed to call Ollama embedding API", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		c.logger.Error("Ollama embedding API error",
			slog.Int("status", resp.StatusCode),
			slog.String("response", string(respBody)),
		)
		return nil, apperrors.Wrap(apperrors.ErrExternal, fmt.Sprintf("Ollama embedding error (status %d)", resp.StatusCode), fmt.Errorf("%s", respBody))
	}

	var result ollamaEmbeddingResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternal, "Failed to decode embedding response", err)
	}

	if len(result.Embedding) == 0 {
		return nil, apperrors.New(apperrors.ErrExternal, "Ollama returned empty embedding")
	}

	c.logger.Debug("Generated embedding successfully",
		slog.String("model", c.embeddingModel),
		slog.Int("dim", len(result.Embedding)),
	)
	return result.Embedding, nil
}

type ollamaGenerateReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaGenerateResp struct {
	Response string `json:"response"`
}

// GenerateAnswer prompts the LLM with retrieved context and user question.
func (c *OllamaClient) GenerateAnswer(ctx context.Context, contextText, question string) (string, error) {
	prompt := fmt.Sprintf("Context:\n%s\n\nQuestion: %s\nAnswer:", contextText, question)

	payload := ollamaGenerateReq{
		Model:  c.llmModel,
		Prompt: prompt,
		Stream: false,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", apperrors.Wrap(apperrors.ErrInternal, "Failed to encode generate payload", err)
	}

	url := fmt.Sprintf("%s/api/generate", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", apperrors.Wrap(apperrors.ErrInternal, "Failed to create generate HTTP request", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", apperrors.Wrap(apperrors.ErrExternal, "Failed to call Ollama generate API", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		c.logger.Error("Ollama generate API error",
			slog.Int("status", resp.StatusCode),
			slog.String("response", string(respBody)),
		)
		return "", apperrors.Wrap(apperrors.ErrExternal, fmt.Sprintf("Ollama generate error (status %d)", resp.StatusCode), fmt.Errorf("%s", respBody))
	}

	var result ollamaGenerateResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", apperrors.Wrap(apperrors.ErrInternal, "Failed to decode generate response", err)
	}

	return result.Response, nil
}
