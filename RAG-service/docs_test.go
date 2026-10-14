package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gopkg.in/yaml.v3"
)

func TestAPIDocumentationRoutes(t *testing.T) {
	app := fiber.New()
	registerAPIDocs(app)

	specResponse, err := app.Test(httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer specResponse.Body.Close()
	spec, err := io.ReadAll(specResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	if specResponse.StatusCode != http.StatusOK || !strings.Contains(string(spec), "title: RAG Service API") {
		t.Fatalf("unexpected OpenAPI response: status=%d", specResponse.StatusCode)
	}
	var document map[string]interface{}
	if err := yaml.Unmarshal(spec, &document); err != nil {
		t.Fatalf("OpenAPI YAML is invalid: %v", err)
	}
	if document["openapi"] != "3.0.3" || len(document["paths"].(map[string]interface{})) == 0 {
		t.Fatal("OpenAPI document must declare version 3.0.3 and at least one path")
	}

	uiResponse, err := app.Test(httptest.NewRequest(http.MethodGet, "/docs", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer uiResponse.Body.Close()
	ui, err := io.ReadAll(uiResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	if uiResponse.StatusCode != http.StatusOK || !strings.Contains(string(ui), "SwaggerUIBundle") {
		t.Fatalf("unexpected Swagger UI response: status=%d", uiResponse.StatusCode)
	}
}
