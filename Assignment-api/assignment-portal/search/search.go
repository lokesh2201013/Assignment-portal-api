package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/lokesh2201013/database"
	"github.com/lokesh2201013/models"
)

type VideoSearch struct {
	client *elasticsearch.Client
	logger *slog.Logger
}

func NewVideoSearch(client *elasticsearch.Client, logger *slog.Logger) *VideoSearch {
	return &VideoSearch{client: client, logger: logger}
}

func (s *VideoSearch) IndexVideo(ctx context.Context, video models.Video) error {
	data, err := json.Marshal(video)
	if err != nil {
		return fmt.Errorf("marshal video: %w", err)
	}

	res, err := s.client.Index(
		"videos",
		bytes.NewReader(data),
		s.client.Index.WithContext(ctx),
		s.client.Index.WithDocumentID(video.ID.String()),
		s.client.Index.WithRefresh("true"),
	)
	if err != nil {
		return fmt.Errorf("index video: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("index video returned status %s", res.Status())
	}
	s.logger.Info("indexed video", slog.String("video_id", video.ID.String()), slog.String("title", video.Title))
	return nil
}

func IndexVideo(video models.Video) {
	if database.Es == nil {
		return
	}
	_ = NewVideoSearch(database.Es, slog.Default()).IndexVideo(context.Background(), video)
}

func (s *VideoSearch) SearchVideos(ctx context.Context, query string) ([]models.Video, error) {
	var buf bytes.Buffer
	searchQuery := map[string]interface{}{
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":     query,
				"fields":    []string{"title^3", "description", "tags^2"},
				"fuzziness": "AUTO",
			},
		},
	}

	if err := json.NewEncoder(&buf).Encode(searchQuery); err != nil {
		return nil, fmt.Errorf("encode search query: %w", err)
	}

	res, err := s.client.Search(
		s.client.Search.WithContext(ctx),
		s.client.Search.WithIndex("videos"),
		s.client.Search.WithBody(&buf),
		s.client.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, fmt.Errorf("search videos: %w", err)
	}
	defer res.Body.Close()

	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("parse search response: %w", err)
	}

	hits, ok := r["hits"].(map[string]interface{})["hits"].([]interface{})
	if !ok {
		return []models.Video{}, nil
	}

	var videos []models.Video
	for _, hit := range hits {
		src, ok := hit.(map[string]interface{})["_source"]
		if !ok {
			continue
		}
		data, _ := json.Marshal(src)
		var v models.Video
		if err := json.Unmarshal(data, &v); err == nil {
			videos = append(videos, v)
		}
	}

	return videos, nil
}

func SearchVideos(query string) []models.Video {
	if database.Es == nil {
		return nil
	}
	videos, _ := NewVideoSearch(database.Es, slog.Default()).SearchVideos(context.Background(), query)
	return videos
}
