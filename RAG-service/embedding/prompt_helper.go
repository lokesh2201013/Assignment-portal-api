package embedding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// SearchInQdrant queries Qdrant with an embedded query and returns the topK text results
func SearchInQdrant(query string, collectionName string, topK int) ([]string, error) {
	queryEmbedding, err := EmbedText(query)
	if err != nil {
		return nil, fmt.Errorf("embedding error: %w", err)
	}

	searchReq := map[string]interface{}{
		"vector":       queryEmbedding,
		"limit":        topK,
		"with_payload": true,
		"params": map[string]interface{}{
			"exact": true,
		},
	}

	data, _ := json.Marshal(searchReq)
	url := fmt.Sprintf("http://localhost:6333/collections/%s/points/search", collectionName)
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Qdrant error: %s", body)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	fmt.Println("========================================")
	fmt.Println("RAW QDRANT RESPONSE:")
	fmt.Println(string(bodyBytes))
	fmt.Println("========================================")

	bodyReader := bytes.NewReader(bodyBytes)

	var result struct {
		Result []struct {
			Payload map[string]interface{} `json:"payload"`
		} `json:"result"`
	}

	if err := json.NewDecoder(bodyReader).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	var hits []string
	for _, r := range result.Result {
		// Try both "text" and "content" fields
		for _, key := range []string{"text", "content"} {
			if val, ok := r.Payload[key].(string); ok {
				hits = append(hits, val)
				break
			}
		}
	}

	fmt.Println("EXTRACTED HITS:", hits)
	return hits, nil
}

// AskLlama takes the retrieved context and asks Ollama's model
func AskLlama(contexts []string, question string) (string, error) {
	fullContext := ""
	for _, c := range contexts {
		fullContext += c + "\n"
	}

	prompt := fmt.Sprintf("Context:\n%s\n\nQuestion: %s\nAnswer:", fullContext, question)

	payload := map[string]interface{}{
		"model":  "llama3.1:8b",
		"prompt": prompt,
		"stream": false,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error marshaling llama payload: %w", err)
	}

	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama api error: %s", body)
	}

	var result struct {
		Response string `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Response, nil
}