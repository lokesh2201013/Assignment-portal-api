package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	"github.com/lokesh2201013/apperrors"
	"github.com/lokesh2201013/config"
	"github.com/lokesh2201013/dto"
	"github.com/lokesh2201013/models"
	pb "github.com/lokesh2201013/proto"
	"github.com/lokesh2201013/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type videoRepository interface {
	Create(video *models.Video) error
	UpdateStatus(id string, status string) error
}

type videoIndexer interface {
	IndexVideo(ctx context.Context, video models.Video) error
}

type messagePublisher interface {
	PublishMessage(id string) error
}

type AidService struct {
	cfg       config.Config
	db        *sql.DB
	videos    videoRepository
	indexer   videoIndexer
	publisher messagePublisher
	http      *http.Client
}

func NewAidService(cfg config.Config, db *sql.DB, videos videoRepository, indexer videoIndexer, publisher messagePublisher) *AidService {
	return &AidService{
		cfg:       cfg,
		db:        db,
		videos:    videos,
		indexer:   indexer,
		publisher: publisher,
		http:      &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *AidService) UploadFiles(ctx context.Context, files []*multipart.FileHeader) error {
	if len(files) == 0 {
		return apperrors.New(apperrors.ErrValidation, "No files uploaded")
	}

	conn, err := grpc.NewClient(s.cfg.RAGServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return apperrors.Wrap(apperrors.ErrExternal, "Failed to connect to RAG service", err)
	}
	defer conn.Close()

	client := pb.NewRAGServiceClient(conn)
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			return apperrors.Wrap(apperrors.ErrUnexpected, "Failed to open file", err)
		}
		content, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if readErr != nil {
			return apperrors.Wrap(apperrors.ErrUnexpected, "Failed to read file", readErr)
		}
		if closeErr != nil {
			return apperrors.Wrap(apperrors.ErrUnexpected, "Failed to close file", closeErr)
		}

		callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		res, err := client.UploadFile(callCtx, &pb.FileUploadRequest{
			Filename: fileHeader.Filename,
			Content:  content,
		})
		cancel()
		if err != nil {
			return apperrors.Wrap(apperrors.ErrExternal, "gRPC upload failed", err)
		}
		if res == nil || !res.Message {
			return apperrors.New(apperrors.ErrExternal, "gRPC upload failed")
		}
	}
	return nil
}

func (s *AidService) GetHelp(ctx context.Context, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", apperrors.New(apperrors.ErrValidation, "Query cannot be empty")
	}

	conn, err := grpc.NewClient(s.cfg.RAGServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", apperrors.Wrap(apperrors.ErrExternal, "Failed to connect to RAG service", err)
	}
	defer conn.Close()

	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	res, err := pb.NewRAGServiceClient(conn).QueryWithContext(callCtx, &pb.QueryRequest{Query: query})
	if err != nil {
		return "", apperrors.Wrap(apperrors.ErrExternal, "gRPC call failed", err)
	}
	if res == nil {
		return "", apperrors.New(apperrors.ErrExternal, "RAG service returned no response")
	}
	return res.Answer, nil
}

func (s *AidService) QueryData(ctx context.Context, naturalLanguageQuery string) ([]map[string]interface{}, string, error) {
	naturalLanguageQuery = strings.TrimSpace(naturalLanguageQuery)
	if naturalLanguageQuery == "" {
		return nil, "", apperrors.New(apperrors.ErrValidation, "Query cannot be empty")
	}

	generatedSQL, err := s.generateSQL(ctx, naturalLanguageQuery)
	if err != nil {
		return nil, "", err
	}
	if !isReadOnlySQL(generatedSQL) {
		return nil, generatedSQL, apperrors.New(apperrors.ErrForbidden, "Only read-only SELECT queries are allowed")
	}

	rows, err := s.db.QueryContext(ctx, generatedSQL)
	if err != nil {
		return nil, generatedSQL, apperrors.Wrap(apperrors.ErrValidation, "SQL query could not be executed", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, generatedSQL, apperrors.Wrap(apperrors.ErrDatabase, "Failed to get columns", err)
	}

	results := []map[string]interface{}{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		pointers := make([]interface{}, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, generatedSQL, apperrors.Wrap(apperrors.ErrDatabase, "Row scan error", err)
		}

		row := map[string]interface{}{}
		for i, column := range columns {
			if b, ok := values[i].([]byte); ok {
				row[column] = string(b)
			} else {
				row[column] = values[i]
			}
		}
		results = append(results, row)
	}
	return results, generatedSQL, nil
}

func (s *AidService) CreatePresignedVideoURL(ctx context.Context, req dto.VideoPresignRequest) (string, uuid.UUID, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	if req.Title == "" || req.Description == "" {
		return "", uuid.Nil, apperrors.New(apperrors.ErrValidation, "Title and Description are required")
	}
	if !strings.HasSuffix(strings.ToLower(req.Title), ".mp4") {
		return "", uuid.Nil, apperrors.New(apperrors.ErrValidation, "Only .mp4 files are allowed")
	}
	if s.cfg.GCSPrivateKey == "" {
		return "", uuid.Nil, apperrors.New(apperrors.ErrUnexpected, "Video upload signing is not configured")
	}

	video := models.Video{
		ID:          uuid.New(),
		URL:         req.URL,
		Title:       req.Title,
		Tags:        req.Tags,
		Description: req.Description,
		Status:      "URLStage",
	}
	url, err := storage.SignedURL(s.cfg.GCSBucketName, "raw/"+req.Title, &storage.SignedURLOptions{
		GoogleAccessID: s.cfg.GCSAccessID,
		PrivateKey:     []byte(s.cfg.GCSPrivateKey),
		Method:         http.MethodPut,
		Expires:        time.Now().Add(15 * time.Minute),
		ContentType:    "video/mp4",
	})
	if err != nil {
		return "", uuid.Nil, apperrors.Wrap(apperrors.ErrExternal, "Failed to generate signed URL", err)
	}

	if err := s.videos.Create(&video); err != nil {
		return "", uuid.Nil, err
	}
	if s.indexer != nil {
		if err := s.indexer.IndexVideo(ctx, video); err != nil {
			return "", uuid.Nil, apperrors.Wrap(apperrors.ErrExternal, "Failed to index video", err)
		}
	}
	return url, video.ID, nil
}

func (s *AidService) StartVideoProcessing(id string) error {
	if strings.TrimSpace(id) == "" {
		return apperrors.New(apperrors.ErrValidation, "Video id is required")
	}
	if s.publisher != nil {
		if err := s.publisher.PublishMessage(id); err != nil {
			return apperrors.Wrap(apperrors.ErrExternal, "Could not publish video processing message", err)
		}
	}
	return s.videos.UpdateStatus(id, "Processing")
}

func (s *AidService) generateSQL(ctx context.Context, query string) (string, error) {
	schema := `assignments {
    assignment_id: uuid
    email: string
    admin_id: uuid
    task: string
    updated_at: string
    due_date: string
    branch: string
    semester: int
    subject_code: string
}
users {
    user_id: string
    name: string
    email: string
    password: string
    role: string
    branch: string
    semester: int
}
submit_assignments {
    submission_id: uuid
    assignment_id: uuid
    user_id: uuid
    status: string
    file: string
    image: string
    comments: string
    late_submission: bool
    created_at: string
}`

	prompt := "Return ONLY a raw read-only SELECT SQL query without explanations or fences. Never select password columns. These are the DB models: " + schema + ". Query: " + query
	payload := map[string]interface{}{
		"model":  "llama3.1:8b",
		"prompt": prompt,
		"stream": false,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", apperrors.Wrap(apperrors.ErrUnexpected, "Error marshaling Ollama payload", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.OllamaURL, bytes.NewReader(data))
	if err != nil {
		return "", apperrors.Wrap(apperrors.ErrUnexpected, "Failed to create Ollama request", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return "", apperrors.Wrap(apperrors.ErrExternal, "Failed to call Ollama", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", apperrors.Wrap(apperrors.ErrExternal, "Ollama API error", fmt.Errorf("status %d", resp.StatusCode))
	}

	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", apperrors.Wrap(apperrors.ErrExternal, "Failed to decode Ollama response", err)
	}
	return utils.CleanSQL(result.Response), nil
}

func isReadOnlySQL(query string) bool {
	normalized := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(query, ";")))
	if !strings.HasPrefix(normalized, "select ") {
		return false
	}
	blocked := []string{" insert ", " update ", " delete ", " drop ", " alter ", " truncate ", " create ", " grant ", " revoke ", " password"}
	padded := " " + normalized + " "
	for _, keyword := range blocked {
		if strings.Contains(padded, keyword) {
			return false
		}
	}
	return true
}
