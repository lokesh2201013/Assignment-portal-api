package services

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	"github.com/lokesh2201013/apperrors"
	"os"
)

type LocalFileSaver struct{}

func NewLocalFileSaver() *LocalFileSaver {
	return &LocalFileSaver{}
}

func (s *LocalFileSaver) Save(fileHeader *multipart.FileHeader, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	src, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	path := filepath.Join(dir, fmt.Sprintf("%d_%s", time.Now().UnixNano(), SafeUploadedFilename(fileHeader.Filename)))
	dst, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}
	return path, nil
}

type VirusScanner interface {
	Scan(context.Context, io.Reader) error
}

type ClamAVScanner struct {
	Address string
}

func (s ClamAVScanner) Scan(ctx context.Context, content io.Reader) error {
	if strings.TrimSpace(s.Address) == "" {
		return fmt.Errorf("virus scanner is not configured")
	}
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", s.Address)
	if err != nil {
		return fmt.Errorf("connect to virus scanner: %w", err)
	}
	defer conn.Close()
	deadline := time.Now().Add(30 * time.Second)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	_ = conn.SetDeadline(deadline)
	stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopClose()
	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return fmt.Errorf("start virus scan stream: %w", err)
	}
	chunk := make([]byte, 32*1024)
	for {
		count, readErr := content.Read(chunk)
		if count > 0 {
			var length [4]byte
			binary.BigEndian.PutUint32(length[:], uint32(count))
			if _, err := conn.Write(length[:]); err != nil {
				return fmt.Errorf("send file to virus scanner: %w", err)
			}
			if _, err := conn.Write(chunk[:count]); err != nil {
				return fmt.Errorf("send file to virus scanner: %w", err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read file for virus scan: %w", readErr)
		}
	}
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return fmt.Errorf("finish virus scan stream: %w", err)
	}
	response, err := io.ReadAll(io.LimitReader(conn, 4096))
	if err != nil {
		return fmt.Errorf("read virus scan result: %w", err)
	}
	result := strings.TrimSpace(string(response))
	if !strings.HasSuffix(result, "OK") {
		return fmt.Errorf("virus scan failed: %s", result)
	}
	return nil
}

type GCSFileSaver struct {
	client        *storage.Client
	bucket        string
	scanner       VirusScanner
	maxBytes      int64
	retentionDays int
	accessID      string
	privateKey    []byte
}

func NewGCSFileSaver(client *storage.Client, bucket string, scanner VirusScanner, maxBytes int64, retentionDays int, accessID string, privateKey string) *GCSFileSaver {
	return &GCSFileSaver{client: client, bucket: bucket, scanner: scanner, maxBytes: maxBytes, retentionDays: retentionDays, accessID: accessID, privateKey: []byte(privateKey)}
}

func (s *GCSFileSaver) Save(fileHeader *multipart.FileHeader, category string) (string, error) {
	return s.SaveForTenant(context.Background(), "default", fileHeader, category)
}

func (s *GCSFileSaver) SaveForTenant(ctx context.Context, tenantID string, fileHeader *multipart.FileHeader, category string) (string, error) {
	if fileHeader == nil || fileHeader.Size <= 0 || fileHeader.Size > s.maxBytes {
		return "", apperrors.New(apperrors.ErrValidation, "Upload is empty or exceeds the configured size limit")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return "", apperrors.Wrap(apperrors.ErrUnexpected, "Could not open upload", err)
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, s.maxBytes+1))
	if err != nil {
		return "", apperrors.Wrap(apperrors.ErrUnexpected, "Could not read upload", err)
	}
	if int64(len(content)) > s.maxBytes {
		return "", apperrors.New(apperrors.ErrValidation, "Upload exceeds the configured size limit")
	}
	contentType := http.DetectContentType(content)
	if !allowedUpload(category, filepath.Ext(fileHeader.Filename), contentType) {
		return "", apperrors.New(apperrors.ErrValidation, "Unsupported file type")
	}
	if !validDocumentSignature(category, filepath.Ext(fileHeader.Filename), content) {
		return "", apperrors.New(apperrors.ErrValidation, "File content does not match its declared type")
	}
	if s.scanner == nil {
		return "", apperrors.New(apperrors.ErrExternal, "Virus scanning is unavailable")
	}
	if err := s.scanner.Scan(ctx, bytes.NewReader(content)); err != nil {
		return "", apperrors.Wrap(apperrors.ErrForbidden, "Upload failed malware scanning", err)
	}
	objectName := fmt.Sprintf("tenant/%s/assignments/%s/%s_%s", tenantID, uuid.NewString(), category, SafeUploadedFilename(fileHeader.Filename))
	writer := s.client.Bucket(s.bucket).Object(objectName).NewWriter(ctx)
	writer.ContentType = contentType
	writer.CustomTime = time.Now().UTC()
	writer.Metadata = map[string]string{"tenant_id": tenantID, "scan_status": "clean", "retention_days": fmt.Sprintf("%d", s.retentionDays)}
	if _, err := writer.Write(content); err != nil {
		_ = writer.Close()
		return "", apperrors.Wrap(apperrors.ErrExternal, "Could not store upload", err)
	}
	if err := writer.Close(); err != nil {
		return "", apperrors.Wrap(apperrors.ErrExternal, "Could not finalize upload", err)
	}
	return "gs://" + s.bucket + "/" + objectName, nil
}

func (s *GCSFileSaver) SignedDownloadURL(objectRef string, expiry time.Duration) (string, error) {
	prefix := "gs://" + s.bucket + "/"
	if !strings.HasPrefix(objectRef, prefix) || s.accessID == "" || len(s.privateKey) == 0 {
		return "", apperrors.New(apperrors.ErrForbidden, "Invalid or unsigned file reference")
	}
	return storage.SignedURL(s.bucket, strings.TrimPrefix(objectRef, prefix), &storage.SignedURLOptions{
		GoogleAccessID: s.accessID,
		PrivateKey:     s.privateKey,
		Method:         http.MethodGet,
		Expires:        time.Now().Add(expiry),
	})
}

func (s *GCSFileSaver) SignedDownloadURLForTenant(tenantID string, objectRef string, expiry time.Duration) (string, error) {
	prefix := "gs://" + s.bucket + "/tenant/" + tenantID + "/assignments/"
	if !strings.HasPrefix(objectRef, prefix) {
		return "", apperrors.New(apperrors.ErrForbidden, "File does not belong to the authenticated tenant")
	}
	return s.SignedDownloadURL(objectRef, expiry)
}

func (s *GCSFileSaver) Delete(ctx context.Context, objectRef string) error {
	prefix := "gs://" + s.bucket + "/"
	if !strings.HasPrefix(objectRef, prefix) {
		return fmt.Errorf("invalid GCS object reference")
	}
	return s.client.Bucket(s.bucket).Object(strings.TrimPrefix(objectRef, prefix)).Delete(ctx)
}

func allowedUpload(category, extension, contentType string) bool {
	extension = strings.ToLower(extension)
	if category == "image" {
		return (contentType == "image/jpeg" && (extension == ".jpg" || extension == ".jpeg")) || (contentType == "image/png" && extension == ".png")
	}
	switch extension {
	case ".pdf":
		return contentType == "application/pdf"
	case ".txt":
		return strings.HasPrefix(contentType, "text/plain")
	case ".doc":
		return contentType == "application/msword" || contentType == "application/octet-stream"
	case ".docx":
		return contentType == "application/zip" || contentType == "application/octet-stream"
	default:
		return false
	}
}

func validDocumentSignature(category, extension string, content []byte) bool {
	if category == "image" || strings.EqualFold(extension, ".txt") {
		return true
	}
	switch strings.ToLower(extension) {
	case ".pdf":
		return bytes.HasPrefix(content, []byte("%PDF-"))
	case ".doc":
		return bytes.HasPrefix(content, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
	case ".docx":
		return bytes.HasPrefix(content, []byte("PK\x03\x04"))
	default:
		return false
	}
}
