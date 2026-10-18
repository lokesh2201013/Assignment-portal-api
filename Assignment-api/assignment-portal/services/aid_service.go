package services

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lokesh2201013/apperrors"
	"github.com/lokesh2201013/config"
	"github.com/lokesh2201013/dto"
	"github.com/lokesh2201013/models"
	pb "github.com/lokesh2201013/proto"
	"github.com/lokesh2201013/utils"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type videoRepository interface {
	Create(video *models.Video) error
	UpdateStatus(id string, status string) error
	UpdateStatusInTenant(tenantID string, id string, status string) error
}

type videoIndexer interface {
	IndexVideo(ctx context.Context, video models.Video) error
}

type messagePublisher interface {
	PublishMessage(id string) error
}

type AidService struct {
	cfg       config.Config
	db        *sqlx.DB
	storage   *storage.Client
	scanner   VirusScanner
	maxUpload int64
	videos    videoRepository
	indexer   videoIndexer
	publisher messagePublisher
	http      *http.Client
}

func (s *AidService) ConfigureUploadSecurity(client *storage.Client, scanner VirusScanner, maxUpload int64) {
	s.storage = client
	s.scanner = scanner
	s.maxUpload = maxUpload
}

func NewAidService(cfg config.Config, db *sqlx.DB, videos videoRepository, indexer videoIndexer, publisher messagePublisher) *AidService {
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

	conn, err := grpc.NewClient(s.cfg.RAGServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrExternal, "Failed to connect to RAG service", err)
	}
	defer conn.Close()

	client := pb.NewRAGServiceClient(conn)
	for _, fileHeader := range files {
		if fileHeader == nil || fileHeader.Size <= 0 || fileHeader.Size > s.maxUpload || s.storage == nil || s.scanner == nil {
			return apperrors.New(apperrors.ErrValidation, "RAG upload is empty, too large, or upload scanning is unavailable")
		}
		file, err := fileHeader.Open()
		if err != nil {
			return apperrors.Wrap(apperrors.ErrUnexpected, "Failed to open file", err)
		}
		content, readErr := io.ReadAll(io.LimitReader(file, s.maxUpload+1))
		closeErr := file.Close()
		if readErr != nil {
			return apperrors.Wrap(apperrors.ErrUnexpected, "Failed to read file", readErr)
		}
		if closeErr != nil {
			return apperrors.Wrap(apperrors.ErrUnexpected, "Failed to close file", closeErr)
		}
		if int64(len(content)) > s.maxUpload {
			return apperrors.New(apperrors.ErrValidation, "RAG upload exceeds the configured size limit")
		}
		contentType := http.DetectContentType(content)
		if !allowedUpload("submission", filepath.Ext(fileHeader.Filename), contentType) || !validDocumentSignature("submission", filepath.Ext(fileHeader.Filename), content) {
			return apperrors.New(apperrors.ErrValidation, "Unsupported or invalid RAG document type")
		}
		if err := s.scanner.Scan(ctx, bytes.NewReader(content)); err != nil {
			return apperrors.Wrap(apperrors.ErrForbidden, "RAG document failed malware scanning", err)
		}
		tenantID := utils.TenantIDFromContext(ctx)
		objectName := "tenant/" + tenantID + "/rag/" + uuid.NewString() + "_" + SafeUploadedFilename(fileHeader.Filename)
		writer := s.storage.Bucket(s.cfg.GCSBucketName).Object(objectName).NewWriter(ctx)
		writer.ContentType = contentType
		writer.CustomTime = time.Now().UTC()
		writer.Metadata = map[string]string{"tenant_id": tenantID, "scan_status": "clean", "retention_days": fmt.Sprintf("%d", s.cfg.UploadRetention)}
		if _, err := writer.Write(content); err != nil {
			_ = writer.Close()
			return apperrors.Wrap(apperrors.ErrExternal, "Could not retain RAG document", err)
		}
		if err := writer.Close(); err != nil {
			return apperrors.Wrap(apperrors.ErrExternal, "Could not retain RAG document", err)
		}

		callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		callCtx = metadata.AppendToOutgoingContext(callCtx, "tenant-id", utils.TenantIDFromContext(ctx))
		res, err := client.UploadFile(callCtx, &pb.FileUploadRequest{
			Filename: fileHeader.Filename,
			Content:  content,
		})
		cancel()
		if err != nil {
			_ = s.storage.Bucket(s.cfg.GCSBucketName).Object(objectName).Delete(ctx)
			return apperrors.Wrap(apperrors.ErrExternal, "gRPC upload failed", err)
		}
		if res == nil || !res.Message {
			_ = s.storage.Bucket(s.cfg.GCSBucketName).Object(objectName).Delete(ctx)
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

	conn, err := grpc.NewClient(s.cfg.RAGServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return "", apperrors.Wrap(apperrors.ErrExternal, "Failed to connect to RAG service", err)
	}
	defer conn.Close()

	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	callCtx = metadata.AppendToOutgoingContext(callCtx, "tenant-id", utils.TenantIDFromContext(ctx))

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
	report, ok := resolveAnalyticsReport(naturalLanguageQuery)
	if !ok {
		return nil, "", apperrors.New(apperrors.ErrValidation, "Unknown report. Allowed reports: assignments_by_branch, submission_status, overdue_assignments")
	}

	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, report.name, apperrors.Wrap(apperrors.ErrDatabase, "Could not start read-only query", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SET LOCAL statement_timeout = '5s'`); err != nil {
		return nil, report.name, apperrors.Wrap(apperrors.ErrDatabase, "Could not constrain query execution", err)
	}
	rows, err := tx.QueryxContext(ctx, report.query, utils.TenantIDFromContext(ctx))
	if err != nil {
		return nil, report.name, apperrors.Wrap(apperrors.ErrDatabase, "Could not execute analytics report", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, report.name, apperrors.Wrap(apperrors.ErrDatabase, "Failed to get columns", err)
	}

	results := []map[string]interface{}{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		pointers := make([]interface{}, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, report.name, apperrors.Wrap(apperrors.ErrDatabase, "Row scan error", err)
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
		if len(results) == 1000 {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, report.name, apperrors.Wrap(apperrors.ErrDatabase, "Row iteration failed", err)
	}
	if err := rows.Close(); err != nil {
		return nil, report.name, apperrors.Wrap(apperrors.ErrDatabase, "Could not close query results", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, report.name, apperrors.Wrap(apperrors.ErrDatabase, "Could not finish query", err)
	}
	return results, report.name, nil
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
		TenantID:    utils.TenantIDFromContext(ctx),
		URL:         req.URL,
		Title:       req.Title,
		Tags:        req.Tags,
		Description: req.Description,
		Status:      "URLStage",
	}
	url, err := storage.SignedURL(s.cfg.GCSBucketName, "raw/"+video.TenantID+"/"+video.ID.String()+".mp4", &storage.SignedURLOptions{
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

func (s *AidService) StartVideoProcessing(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return apperrors.New(apperrors.ErrValidation, "Video id is required")
	}
	if err := s.scanStagedVideo(ctx, id); err != nil {
		return err
	}
	if s.publisher != nil {
		if err := s.publisher.PublishMessage(id); err != nil {
			return apperrors.Wrap(apperrors.ErrExternal, "Could not publish video processing message", err)
		}
	}
	return s.videos.UpdateStatusInTenant(utils.TenantIDFromContext(ctx), id, "Processing")
}

func (s *AidService) scanStagedVideo(ctx context.Context, id string) error {
	if s.storage == nil || s.scanner == nil || s.maxUpload <= 0 {
		return apperrors.New(apperrors.ErrExternal, "Video upload scanning is not configured")
	}
	if _, err := uuid.Parse(id); err != nil {
		return apperrors.New(apperrors.ErrValidation, "Invalid video id")
	}
	objectName := "raw/" + utils.TenantIDFromContext(ctx) + "/" + id + ".mp4"
	object := s.storage.Bucket(s.cfg.GCSBucketName).Object(objectName)
	attrs, err := object.Attrs(ctx)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrNotFound, "Staged video not found", err)
	}
	if attrs.Size <= 0 || attrs.Size > s.maxUpload || attrs.ContentType != "video/mp4" {
		return apperrors.New(apperrors.ErrValidation, "Video has an invalid size or content type")
	}
	reader, err := object.NewReader(ctx)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrExternal, "Could not read staged video", err)
	}
	scanErr := s.scanner.Scan(ctx, reader)
	closeErr := reader.Close()
	if scanErr != nil {
		return apperrors.Wrap(apperrors.ErrForbidden, "Video failed malware scanning", scanErr)
	}
	if closeErr != nil {
		return apperrors.Wrap(apperrors.ErrExternal, "Could not close staged video", closeErr)
	}
	_, err = object.Update(ctx, storage.ObjectAttrsToUpdate{
		CustomTime: time.Now().UTC(),
		Metadata: map[string]string{
			"tenant_id":      utils.TenantIDFromContext(ctx),
			"scan_status":    "clean",
			"retention_days": fmt.Sprintf("%d", s.cfg.UploadRetention),
		},
	})
	if err != nil {
		return apperrors.Wrap(apperrors.ErrExternal, "Could not mark video as scanned", err)
	}
	return nil
}

type analyticsReport struct {
	name  string
	query string
}

var analyticsReports = map[string]analyticsReport{
	"assignments_by_branch": {
		name:  "assignments_by_branch",
		query: `SELECT branch, semester, COUNT(*) AS assignment_count FROM assignments WHERE tenant_id = $1 GROUP BY branch, semester ORDER BY branch, semester LIMIT 100`,
	},
	"submission_status": {
		name:  "submission_status",
		query: `SELECT status, COUNT(*) AS submission_count FROM submit_assignments WHERE tenant_id = $1 GROUP BY status ORDER BY status`,
	},
	"overdue_assignments": {
		name:  "overdue_assignments",
		query: `SELECT assignment_id, task, branch, semester, due_date FROM assignments WHERE tenant_id = $1 AND due_date::timestamptz < NOW() ORDER BY due_date DESC LIMIT 100`,
	},
}

func resolveAnalyticsReport(input string) (analyticsReport, bool) {
	key := strings.ToLower(strings.TrimSpace(input))
	if report, ok := analyticsReports[key]; ok {
		return report, true
	}
	aliases := map[string]string{
		"assignments by branch": "assignments_by_branch",
		"submission status":     "submission_status",
		"overdue assignments":   "overdue_assignments",
	}
	if reportName, ok := aliases[key]; ok {
		report, exists := analyticsReports[reportName]
		return report, exists
	}
	return analyticsReport{}, false
}
