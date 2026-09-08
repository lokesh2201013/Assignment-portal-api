package repositories

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atgsgrouptest/genet-microservice/RAG-service/models"
	"github.com/stretchr/testify/assert"
)

func TestQdrantRepository_UpsertAndSearch(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/collections/test-coll/points":
			var req qdrantUpsertPayload
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.NotEmpty(t, req.Points)
			assert.NotEmpty(t, req.Points[0].ID)
			assert.Equal(t, "tenant_1", req.Points[0].Payload["tenant_id"])
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "ok"}`))

		case r.Method == http.MethodPost && r.URL.Path == "/collections/test-coll/points/search":
			var searchReq qdrantSearchPayload
			_ = json.NewDecoder(r.Body).Decode(&searchReq)
			assert.NotNil(t, searchReq.Filter)
			assert.Equal(t, "tenant_id", searchReq.Filter.Must[0].Key)
			assert.Equal(t, "tenant_1", searchReq.Filter.Must[0].Match.Value)

			resp := qdrantSearchResponse{
				Result: []struct {
					ID      interface{}            `json:"id"`
					Score   float64                `json:"score"`
					Payload map[string]interface{} `json:"payload"`
				}{
					{
						ID:    "123",
						Score: 0.99,
						Payload: map[string]interface{}{
							"text":      "Retrieved passage content",
							"tenant_id": "tenant_1",
						},
					},
				},
				Status: "ok",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)

		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			w.WriteHeader(http.StatusOK)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	repo := NewQdrantRepository(server.URL, server.Client(), logger)

	t.Run("UpsertPoints with tenant_id", func(t *testing.T) {
		err := repo.UpsertPoints(context.Background(), "test-coll", []models.VectorPoint{
			{
				Vector:   []float64{0.1, 0.2},
				Payload:  map[string]interface{}{"text": "test content"},
				TenantID: "tenant_1",
			},
		})
		assert.NoError(t, err)
	})

	t.Run("SearchPoints with tenant filter", func(t *testing.T) {
		results, err := repo.SearchPoints(context.Background(), "test-coll", []float64{0.1, 0.2}, 5, "tenant_1")
		assert.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "Retrieved passage content", results[0].Content)
		assert.Equal(t, "tenant_1", results[0].TenantID)
		assert.Equal(t, 0.99, results[0].Score)
	})

	t.Run("Health check success", func(t *testing.T) {
		err := repo.Health(context.Background())
		assert.NoError(t, err)
	})
}
