package embedding

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/atgsgrouptest/genet-microservice/RAG-service/models"
)

type EmbeddedDocument struct {
	Content   string
	Embedding []float64
}

type errorHandler struct{}

var Error errorHandler

func (e errorHandler) ReturnError(location string, err error, msg string) models.Error {
	fmt.Printf("[%s] %s: %v\n", location, msg, err)
	return models.Error{Message: msg}
}

func SplitDocument(content string, chunkSize, overlap int) []string {
	var chunks []string
	for i := 0; i < len(content); i += chunkSize - overlap {
		end := i + chunkSize
		if end > len(content) {
			end = len(content)
		}
		chunks = append(chunks, content[i:end])
	}
	return chunks
}

func EmbedText(chunk string) ([]float64, error) {
	payload := map[string]string{
		"model":  "nomic-embed-text",
		"prompt": chunk,
	}
	data, _ := json.Marshal(payload)
	resp, err := http.Post("http://localhost:11434/api/embeddings", "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API error: %s: %s", resp.Status, body)
	}

	var result struct {
		Embedding []float64 `json:"embedding"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Generated embedding length: %d\n", len(result.Embedding))
	return result.Embedding, nil
}

func EmbedFileToCorpus(file *multipart.FileHeader) ([]EmbeddedDocument, error) {
	if file == nil {
		return nil, errors.New("file is required")
	}

	openedFile, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer openedFile.Close()

	return EmbedReaderToCorpus(openedFile)
}

func EmbedReaderToCorpus(reader io.Reader) ([]EmbeddedDocument, error) {
	contentBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	if len(contentBytes) == 0 {
		return nil, errors.New("file is empty")
	}

	content := string(contentBytes)
	chunks := SplitDocument(content, 250, 20)

	var corpus []EmbeddedDocument
	var embeddingErrors []error
	for i, chunk := range chunks {
		vec, err := EmbedText(chunk)
		if err != nil {
			embeddingErrors = append(embeddingErrors, fmt.Errorf("chunk %d: %w", i+1, err))
			continue
		}
		corpus = append(corpus, EmbeddedDocument{
			Content:   chunk,
			Embedding: vec,
		})
	}

	if len(embeddingErrors) > 0 {
		return corpus, fmt.Errorf("failed to embed %d of %d chunks: %w", len(embeddingErrors), len(chunks), errors.Join(embeddingErrors...))
	}
	return corpus, nil
}
