package controllers

import (
	// "github.com/atgsgrouptest/genet-microservice/RAG-service/Error"
	"fmt"
	"mime/multipart"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/Logger"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/embedding"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	pb "github.com/atgsgrouptest/genet-microservice/RAG-service/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"os"
	
)

type RAGServiceServer struct {
	pb.UnimplementedRAGServiceServer
}

func SendFilesHTTP(c *fiber.Ctx) error {
	form, err := c.MultipartForm()
	if err != nil {
		logger.Log.Error("RAG Service Package Controllers", zap.String("Message", "Failed to parse multipart form"), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"Service Name": "RAG Service Package Controllers",
			"error":        "Unable to receive files",
			"details":      err.Error(),
		})
	}

	files := form.File["files"]
	//var allDocs []embedding.EmbeddedDocument

	for _, file := range files {
		 err := ProcessFile(file)
		if err != nil {
			logger.Log.Error("Failed to process file", zap.Error(err))
			continue
		}
		//allDocs = append(allDocs, docs...)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Files embedded and stored successfully",
	})
}

func ProcessFile(file *multipart.FileHeader) ( error) {
	corpus, err := embedding.EmbedFileToCorpus(file)
	if err != nil {
		return  fmt.Errorf("failed to embed file: %w", err)
	}

	err = embedding.StoreInQdrant(corpus, "g-corpus")
	if err != nil {
		return fmt.Errorf("failed to store in Qdrant: %w", err)
	}

	return nil
}



func (s *RAGServiceServer) UploadFile(ctx context.Context, req *pb.FileUploadRequest) (*pb.FileUploadResponse, error) {
	// Convert proto bytes to a virtual file header
	file := &multipart.FileHeader{
		Filename: req.Filename,
		Size:     int64(len(req.Content)),
		// Not using Header here – populate only if needed
	}

	tempFile, err := os.CreateTemp("", req.Filename)
	if err != nil {
		return &pb.FileUploadResponse{
			Message: false,
		}, status.Errorf(codes.Internal, "Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	if _, err := tempFile.Write(req.Content); err != nil {
		return &pb.FileUploadResponse{
			Message: false,
		}, status.Errorf(codes.Internal, "Failed to write file: %v", err)
	}
	if err := tempFile.Close(); err != nil {
		return &pb.FileUploadResponse{
			Message: false,
		}, status.Errorf(codes.Internal, "Failed to close file: %v", err)
	}

	file.Filename = tempFile.Name()
     err = ProcessFile(file)
	if err != nil {
		return &pb.FileUploadResponse{
			Message: false,
		}, status.Errorf(codes.Internal, "Failed to embed/store file: %v", err)
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
	// 1. Semantic search in Qdrant
	chunks, err := embedding.SearchInQdrant(query, "rag-corpus", 5)
	if err != nil {
		return "", fmt.Errorf("search in Qdrant failed: %w", err)
	}

	// 2. Ask LLM (e.g. LLaMA) with context
	answer, err := embedding.AskLlama(chunks, query)
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	return answer, nil
}
