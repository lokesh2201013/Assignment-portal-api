package repositories

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/atgsgrouptest/genet-microservice/RAG-service/apperrors"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/models"
	"github.com/google/uuid"
)

// VectorRepository defines the contract for vector database persistence.
type VectorRepository interface {
	UpsertPoints(ctx context.Context, collection string, points []models.VectorPoint) error
	SearchPoints(ctx context.Context, collection string, vector []float64, limit int, tenantID string) ([]models.SearchResult, error)
	Health(ctx context.Context) error
}

// QdrantRepository implements VectorRepository using Qdrant REST API.
type QdrantRepository struct {
	baseURL    string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewQdrantRepository constructs a new QdrantRepository.
func NewQdrantRepository(baseURL string, httpClient *http.Client, logger *slog.Logger) *QdrantRepository {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &QdrantRepository{
		baseURL:    baseURL,
		httpClient: httpClient,
		logger:     logger,
	}
}

type qdrantUpsertPayload struct {
	Points []qdrantPointItem `json:"points"`
}

type qdrantPointItem struct {
	ID      string                 `json:"id"`
	Vector  []float64              `json:"vector"`
	Payload map[string]interface{} `json:"payload"`
}

// UpsertPoints inserts or updates vector points in a Qdrant collection.
func (r *QdrantRepository) UpsertPoints(ctx context.Context, collection string, points []models.VectorPoint) error {
	if len(points) == 0 {
		return nil
	}

	var items []qdrantPointItem
	for _, p := range points {
		id := p.ID
		if id == "" {
			id = uuid.New().String()
		}

		payload := p.Payload
		if payload == nil {
			payload = make(map[string]interface{})
		}
		if p.TenantID != "" {
			payload["tenant_id"] = p.TenantID
		}

		items = append(items, qdrantPointItem{
			ID:      id,
			Vector:  p.Vector,
			Payload: payload,
		})
	}

	payload := qdrantUpsertPayload{Points: items}
	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrInternal, "Failed to encode Qdrant payload", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points?wait=true", r.baseURL, collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return apperrors.Wrap(apperrors.ErrInternal, "Failed to create Qdrant request", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrExternal, "Failed to communicate with Qdrant", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		r.logger.Error("Qdrant upsert returned non-2xx status",
			slog.Int("status", resp.StatusCode),
			slog.String("response", string(respBody)),
		)
		return apperrors.Wrap(apperrors.ErrDatabase, fmt.Sprintf("Qdrant upsert error: status %d", resp.StatusCode), fmt.Errorf("%s", respBody))
	}

	r.logger.Debug("Successfully upserted points to Qdrant",
		slog.String("collection", collection),
		slog.Int("count", len(points)),
	)
	return nil
}

type qdrantFilterMatch struct {
	Value string `json:"value"`
}

type qdrantFilterCondition struct {
	Key   string            `json:"key"`
	Match qdrantFilterMatch `json:"match"`
}

type qdrantFilter struct {
	Must []qdrantFilterCondition `json:"must"`
}

type qdrantSearchPayload struct {
	Vector      []float64              `json:"vector"`
	Limit       int                    `json:"limit"`
	WithPayload bool                   `json:"with_payload"`
	Filter      *qdrantFilter          `json:"filter,omitempty"`
	Params      map[string]interface{} `json:"params,omitempty"`
}

type qdrantSearchResponse struct {
	Result []struct {
		ID      interface{}            `json:"id"`
		Score   float64                `json:"score"`
		Payload map[string]interface{} `json:"payload"`
	} `json:"result"`
	Status string `json:"status"`
}

// SearchPoints performs semantic search in Qdrant given a query vector with optional tenant_id isolation.
func (r *QdrantRepository) SearchPoints(ctx context.Context, collection string, vector []float64, limit int, tenantID string) ([]models.SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}

	searchReq := qdrantSearchPayload{
		Vector:      vector,
		Limit:       limit,
		WithPayload: true,
		Params: map[string]interface{}{
			"exact": true,
		},
	}

	if tenantID != "" && tenantID != "*" {
		searchReq.Filter = &qdrantFilter{
			Must: []qdrantFilterCondition{
				{
					Key: "tenant_id",
					Match: qdrantFilterMatch{
						Value: tenantID,
					},
				},
			},
		}
	}

	body, err := json.Marshal(searchReq)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternal, "Failed to marshal search request", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/search", r.baseURL, collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternal, "Failed to create search request", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrExternal, "Failed to communicate with Qdrant", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		r.logger.Error("Qdrant search returned non-200 status",
			slog.Int("status", resp.StatusCode),
			slog.String("response", string(respBody)),
		)
		return nil, apperrors.Wrap(apperrors.ErrDatabase, fmt.Sprintf("Qdrant search error: status %d", resp.StatusCode), fmt.Errorf("%s", respBody))
	}

	var searchResp qdrantSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternal, "Failed to decode Qdrant search response", err)
	}

	var results []models.SearchResult
	for _, hit := range searchResp.Result {
		content := ""
		for _, key := range []string{"text", "content"} {
			if val, ok := hit.Payload[key].(string); ok && val != "" {
				content = val
				break
			}
		}

		pointTenant := ""
		if tVal, ok := hit.Payload["tenant_id"].(string); ok {
			pointTenant = tVal
		}

		results = append(results, models.SearchResult{
			ID:       fmt.Sprintf("%v", hit.ID),
			Content:  content,
			Score:    hit.Score,
			Payload:  hit.Payload,
			TenantID: pointTenant,
		})
	}

	return results, nil
}

// Health checks connectivity to the Qdrant service.
func (r *QdrantRepository) Health(ctx context.Context) error {
	url := fmt.Sprintf("%s/healthz", r.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrInternal, "Failed to create health request", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrExternal, "Failed to ping Qdrant", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return apperrors.Wrap(apperrors.ErrDatabase, fmt.Sprintf("Qdrant health check returned status %d", resp.StatusCode), nil)
	}
	return nil
}
