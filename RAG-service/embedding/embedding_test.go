package embedding

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type embeddingRoundTripper func(*http.Request) (*http.Response, error)

func (roundTripper embeddingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return roundTripper(req)
}

func TestCollectionNameUsesConfiguredValueAndDefault(t *testing.T) {
	t.Setenv("RAG_COLLECTION", "")
	if got := CollectionName(); got != DefaultCollectionName {
		t.Fatalf("CollectionName() = %q, want default %q", got, DefaultCollectionName)
	}

	t.Setenv("RAG_COLLECTION", "assignments")
	if got := CollectionName(); got != "assignments" {
		t.Fatalf("CollectionName() = %q, want configured value %q", got, "assignments")
	}
}

func TestEmbedReaderToCorpusReturnsPartialResultsAndError(t *testing.T) {
	originalTransport := http.DefaultTransport
	http.DefaultTransport = embeddingRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/api/embeddings" {
			t.Fatalf("unexpected request path %q", req.URL.Path)
		}
		var payload struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode embedding request: %v", err)
		}
		statusCode, body := http.StatusOK, `{"embedding":[0.1,0.2]}`
		if len(payload.Prompt) == 21 {
			statusCode, body = http.StatusServiceUnavailable, `{"error":"embedding unavailable"}`
		}
		return &http.Response{
			StatusCode: statusCode,
			Status:     http.StatusText(statusCode),
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	corpus, err := EmbedReaderToCorpus(strings.NewReader(strings.Repeat("x", 251)))
	if err == nil {
		t.Fatal("expected partial embedding error")
	}
	if len(corpus) != 1 {
		t.Fatalf("got %d embedded chunks, want 1", len(corpus))
	}
	if !strings.Contains(err.Error(), "failed to embed 1 of 2 chunks") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEmbedReaderToCorpusRejectsEmptyInput(t *testing.T) {
	corpus, err := EmbedReaderToCorpus(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected empty input error")
	}
	if corpus != nil {
		t.Fatalf("got corpus %#v, want nil", corpus)
	}
}
