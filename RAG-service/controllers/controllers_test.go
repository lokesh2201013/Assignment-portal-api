package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/atgsgrouptest/genet-microservice/RAG-service/logger"
	pb "github.com/atgsgrouptest/genet-microservice/RAG-service/proto"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type controllerRoundTripper func(*http.Request) (*http.Response, error)

func (roundTripper controllerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return roundTripper(req)
}

func mockedResponse(req *http.Request, statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

func withHTTPMock(t *testing.T, handler func(*http.Request) *http.Response) {
	t.Helper()
	originalTransport := http.DefaultTransport
	http.DefaultTransport = controllerRoundTripper(func(req *http.Request) (*http.Response, error) {
		return handler(req), nil
	})
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
}

func TestGRPCUploadUsesRequestBytesAndConfiguredCollection(t *testing.T) {
	t.Setenv("RAG_COLLECTION", "test-documents")
	var upserted bool
	var searched bool
	embeddingCalls := 0
	withHTTPMock(t, func(req *http.Request) *http.Response {
		switch {
		case req.URL.Host == "localhost:11434" && req.URL.Path == "/api/embeddings":
			var payload struct {
				Prompt string `json:"prompt"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Errorf("decode embedding request: %v", err)
				return mockedResponse(req, http.StatusBadRequest, `{}`)
			}
			embeddingCalls++
			switch embeddingCalls {
			case 1:
				if payload.Prompt != "gRPC request content" {
					t.Errorf("embedded prompt = %q, want original gRPC bytes", payload.Prompt)
				}
			case 2:
				if payload.Prompt != "find notes" {
					t.Errorf("query embedding prompt = %q, want %q", payload.Prompt, "find notes")
				}
			}
			return mockedResponse(req, http.StatusOK, `{"embedding":[0.1,0.2]}`)
		case req.URL.Host == "localhost:6333" && req.Method == http.MethodPut:
			if req.URL.Path != "/collections/test-documents/points" {
				t.Errorf("upsert used collection path %q", req.URL.Path)
			}
			upserted = true
			return mockedResponse(req, http.StatusOK, `{}`)
		case req.URL.Host == "localhost:6333" && req.URL.Path == "/collections/test-documents/points/search":
			searched = true
			return mockedResponse(req, http.StatusOK, `{"result":[]}`)
		case req.URL.Host == "localhost:11434" && req.URL.Path == "/api/generate":
			return mockedResponse(req, http.StatusOK, `{"response":"ok"}`)
		default:
			t.Errorf("unexpected request %s %s", req.Method, req.URL)
			return mockedResponse(req, http.StatusInternalServerError, `{}`)
		}
	})

	server := &RAGServiceServer{}
	response, err := server.UploadFile(context.Background(), &pb.FileUploadRequest{
		Filename: "notes.txt",
		Content:  []byte("gRPC request content"),
	})
	if err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}
	if response.GetMessage() != true {
		t.Fatalf("UploadFile() response = %#v, want successful upload", response)
	}
	if !upserted {
		t.Fatal("expected uploaded bytes to be embedded and stored")
	}
	if _, err := ProcessQueryWithContext("find notes"); err != nil {
		t.Fatalf("ProcessQueryWithContext() error = %v", err)
	}
	if !searched {
		t.Fatal("expected query to search the same configured collection")
	}
}

func TestHTTPUploadReportsPartialIngestionErrors(t *testing.T) {
	t.Setenv("RAG_COLLECTION", "http-documents")
	previousLogger := logger.Log
	logger.Log = zap.NewNop()
	t.Cleanup(func() {
		logger.Log = previousLogger
	})

	var upserted bool
	withHTTPMock(t, func(req *http.Request) *http.Response {
		if req.URL.Host == "localhost:11434" && req.URL.Path == "/api/embeddings" {
			var payload struct {
				Prompt string `json:"prompt"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Errorf("decode embedding request: %v", err)
				return mockedResponse(req, http.StatusBadRequest, `{}`)
			}
			if len(payload.Prompt) == 21 {
				return mockedResponse(req, http.StatusServiceUnavailable, `{"error":"embedding unavailable"}`)
			}
			return mockedResponse(req, http.StatusOK, `{"embedding":[0.1,0.2]}`)
		}
		if req.URL.Host == "localhost:6333" && req.Method == http.MethodPut {
			if req.URL.Path != "/collections/http-documents/points" {
				t.Errorf("upsert used collection path %q", req.URL.Path)
			}
			upserted = true
			return mockedResponse(req, http.StatusOK, `{}`)
		}
		t.Errorf("unexpected request %s %s", req.Method, req.URL)
		return mockedResponse(req, http.StatusInternalServerError, `{}`)
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files", "partial.txt")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := io.WriteString(part, strings.Repeat("x", 251)); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	app := fiber.New()
	app.Post("/sendFiles", SendFilesHTTP)
	req := httptest.NewRequest(http.MethodPost, "/sendFiles", &body)
	req.Header.Set(fiber.HeaderContentType, writer.FormDataContentType())
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusMultiStatus {
		responseBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, fiber.StatusMultiStatus, responseBody)
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if !strings.Contains(string(responseBody), `"failed":1`) ||
		!strings.Contains(string(responseBody), "failed to embed 1 of 2 chunks") {
		t.Fatalf("response did not report partial ingestion error: %s", responseBody)
	}
	if !upserted {
		t.Fatal("expected successfully embedded chunks to be stored")
	}
}
