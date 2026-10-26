package controllers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"

	"github.com/atgsgrouptest/genet-microservice/RAG-service/embedding"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/logger"
	pb "github.com/atgsgrouptest/genet-microservice/RAG-service/proto"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RAGServiceServer struct {
	pb.UnimplementedRAGServiceServer
}

func SendFilesHTTP(c *fiber.Ctx) error {
	form, err := c.MultipartForm()
	if err != nil {
		logger.Log.Error("RAG Service Package Controllers", zap.String("Message", "Failed to parse multipart form"), zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"Service Name": "RAG Service Package Controllers",
			"error":        "Unable to receive files",
			"details":      err.Error(),
		})
	}

	files := form.File["files"]
	if len(files) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No files provided",
		})
	}

	type fileError struct {
		Filename string `json:"filename"`
		Error    string `json:"error"`
	}
	var failures []fileError
	processed := 0
	for _, file := range files {
		if err := ProcessFile(file); err != nil {
			logger.Log.Error("Failed to process file", zap.String("filename", file.Filename), zap.Error(err))
			failures = append(failures, fileError{Filename: file.Filename, Error: err.Error()})
			continue
		}
		processed++
	}

	if len(failures) > 0 {
		return c.Status(fiber.StatusMultiStatus).JSON(fiber.Map{
			"message":   "Some files could not be fully embedded and stored",
			"processed": processed,
			"failed":    len(failures),
			"errors":    failures,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message":   "Files embedded and stored successfully",
		"processed": processed,
		"failed":    0,
	})
}

func ProcessFile(file *multipart.FileHeader) error {
	if file == nil {
		return errors.New("file is required")
	}

	openedFile, err := file.Open()
	if err != nil {
		return fmt.Errorf("failed to open file %q: %w", file.Filename, err)
	}
	defer openedFile.Close()

	return processFileReader(file.Filename, openedFile)
}

// ProcessFileContent processes bytes supplied directly by the gRPC upload request.
func ProcessFileContent(filename string, content []byte) error {
	return processFileReader(filename, bytes.NewReader(content))
}

func processFileReader(filename string, reader io.Reader) error {
	corpus, embedErr := embedding.EmbedReaderToCorpus(reader)
	if len(corpus) == 0 && embedErr == nil {
		embedErr = errors.New("no content was embedded")
	}

	var processingErrors []error
	if embedErr != nil {
		processingErrors = append(processingErrors, fmt.Errorf("failed to embed file %q: %w", filename, embedErr))
	}
	if len(corpus) > 0 {
		if err := embedding.StoreInQdrant(corpus, embedding.CollectionName()); err != nil {
			processingErrors = append(processingErrors, fmt.Errorf("failed to store file %q in Qdrant: %w", filename, err))
		}
	}

	return errors.Join(processingErrors...)
}

func (s *RAGServiceServer) UploadFile(ctx context.Context, req *pb.FileUploadRequest) (*pb.FileUploadResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "upload request is required")
	}
	if err := ProcessFileContent(req.Filename, req.Content); err != nil {
		return &pb.FileUploadResponse{
			Message: false,
			Error:   err.Error(),
		}, status.Errorf(codes.Internal, "failed to process upload: %v", err)
	}

	return &pb.FileUploadResponse{
		Message: true,
	}, nil
}

func GetPromptWithContextHTTP(c *fiber.Ctx) error {
	type reqBody struct {
		Query string `json:"query"`
	}

	var body reqBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request",
		})
	}

	answer, err := ProcessQueryWithContext(body.Query)
	if err != nil {
		logger.Log.Error("Failed to process query", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"answer": answer,
	})
}

func (s *RAGServiceServer) QueryWithContext(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	answer, err := ProcessQueryWithContext(req.Query)
	if err != nil {
		logger.Log.Error("Failed to process query", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "error: %v", err)
	}

	return &pb.QueryResponse{
		Answer: answer,
	}, nil
}

func ProcessQueryWithContext(query string) (string, error) {
	chunks, err := embedding.SearchInQdrant(query, embedding.CollectionName(), 5)
	if err != nil {
		return "", fmt.Errorf("search in Qdrant failed: %w", err)
	}

	answer, err := embedding.AskLlama(chunks, query)
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	return answer, nil
}
