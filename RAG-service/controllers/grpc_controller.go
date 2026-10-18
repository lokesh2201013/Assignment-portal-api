package controllers

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/atgsgrouptest/genet-microservice/RAG-service/apperrors"
	pb "github.com/atgsgrouptest/genet-microservice/RAG-service/proto"
	"github.com/atgsgrouptest/genet-microservice/RAG-service/services"
	"google.golang.org/grpc/metadata"
)

// RAGGRPCServer implements the pb.RAGServiceServer gRPC interface.
type RAGGRPCServer struct {
	pb.UnimplementedRAGServiceServer
	ragService services.RAGService
	logger     *slog.Logger
}

// NewRAGGRPCServer constructs a new RAGGRPCServer.
func NewRAGGRPCServer(ragService services.RAGService, logger *slog.Logger) *RAGGRPCServer {
	return &RAGGRPCServer{
		ragService: ragService,
		logger:     logger,
	}
}

// UploadFile handles gRPC file upload and embedding requests with optional metadata-based multi-tenancy.
func (s *RAGGRPCServer) UploadFile(ctx context.Context, req *pb.FileUploadRequest) (*pb.FileUploadResponse, error) {
	if req == nil {
		return &pb.FileUploadResponse{Message: false, Error: "Empty request"}, apperrors.ToGRPCStatus(apperrors.New(apperrors.ErrValidation, "Empty request"))
	}

	if strings.TrimSpace(req.Filename) == "" {
		err := apperrors.New(apperrors.ErrValidation, "Filename cannot be empty")
		return &pb.FileUploadResponse{Message: false, Error: apperrors.PublicMessage(err)}, apperrors.ToGRPCStatus(err)
	}

	if len(req.Content) == 0 {
		err := apperrors.New(apperrors.ErrValidation, "File content cannot be empty")
		return &pb.FileUploadResponse{Message: false, Error: apperrors.PublicMessage(err)}, apperrors.ToGRPCStatus(err)
	}

	tenantID, collectionName := s.extractTenantFromMetadata(ctx)
	safeFilename := filepath.Base(req.Filename)

	_, err := s.ragService.ProcessDocument(ctx, tenantID, collectionName, safeFilename, req.Content)
	if err != nil {
		s.logger.Error("gRPC UploadFile failed",
			slog.String("tenant_id", tenantID),
			slog.String("filename", safeFilename),
			slog.Any("error", err),
		)
		return &pb.FileUploadResponse{
			Message: false,
			Error:   apperrors.PublicMessage(err),
		}, apperrors.ToGRPCStatus(err)
	}

	return &pb.FileUploadResponse{
		Message: true,
	}, nil
}

// QueryWithContext handles gRPC contextual Q&A requests with optional metadata-based multi-tenancy.
func (s *RAGGRPCServer) QueryWithContext(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	if req == nil {
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.ErrValidation, "Empty query request"))
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.ErrValidation, "Query cannot be empty"))
	}

	tenantID, collectionName := s.extractTenantFromMetadata(ctx)
	answer, err := s.ragService.ProcessQuery(ctx, tenantID, collectionName, query)
	if err != nil {
		s.logger.Error("gRPC QueryWithContext failed",
			slog.String("tenant_id", tenantID),
			slog.Any("error", err),
		)
		return nil, apperrors.ToGRPCStatus(err)
	}

	return &pb.QueryResponse{
		Answer: answer,
	}, nil
}

func (s *RAGGRPCServer) extractTenantFromMetadata(ctx context.Context) (tenantID, collectionName string) {
	tenantID = "default"
	collectionName = "rag-corpus"

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("tenant-id"); len(vals) > 0 && strings.TrimSpace(vals[0]) != "" {
			tenantID = strings.TrimSpace(vals[0])
		} else if vals := md.Get("x-tenant-id"); len(vals) > 0 && strings.TrimSpace(vals[0]) != "" {
			tenantID = strings.TrimSpace(vals[0])
		}

		if vals := md.Get("collection"); len(vals) > 0 && strings.TrimSpace(vals[0]) != "" {
			collectionName = strings.TrimSpace(vals[0])
		} else if vals := md.Get("x-collection-name"); len(vals) > 0 && strings.TrimSpace(vals[0]) != "" {
			collectionName = strings.TrimSpace(vals[0])
		}
	}

	return tenantID, collectionName
}
