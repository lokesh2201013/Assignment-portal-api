package embedding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

const DefaultCollectionName = "rag-corpus"

func CollectionName() string {
	if name := strings.TrimSpace(os.Getenv("RAG_COLLECTION")); name != "" {
		return name
	}
	return DefaultCollectionName
}

type QdrantPoint struct {
	ID      int                    `json:"id"`
	Vector  []float64              `json:"vector"`
	Payload map[string]interface{} `json:"payload"`
}

type QdrantUpsertRequest struct {
	Points []QdrantPoint `json:"points"`
}

func StoreInQdrant(docs []EmbeddedDocument, collectionName string) error {
	var points []QdrantPoint
	for i, doc := range docs {
		points = append(points, QdrantPoint{
			ID:     i,
			Vector: doc.Embedding,
			Payload: map[string]interface{}{
				"text": doc.Content,
			},
		})
	}

	body := QdrantUpsertRequest{Points: points}
	bodyData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode Qdrant points: %w", err)
	}

	url := fmt.Sprintf("http://localhost:6333/collections/%s/points?wait=true", collectionName)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(bodyData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("failed to insert points, status: %s", resp.Status)
	}

	fmt.Println("Inserted documents into Qdrant")
	return nil
}
