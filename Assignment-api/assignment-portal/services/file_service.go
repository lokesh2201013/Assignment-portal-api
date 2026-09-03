package services

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"
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
