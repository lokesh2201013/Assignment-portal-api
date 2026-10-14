package controllers

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/atgsgrouptest/genet-microservice/RAG-service/models"
	pb "github.com/atgsgrouptest/genet-microservice/RAG-service/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestRAGGRPCServer_UploadFile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("nil request returns InvalidArgument", func(t *testing.T) {
		mockSvc := new(MockRAGService)
		server := NewRAGGRPCServer(mockSvc, logger)

		res, err := server.UploadFile(context.Background(), nil)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.False(t, res.Message)
	})

	t.Run("empty filename returns InvalidArgument", func(t *testing.T) {
		mockSvc := new(MockRAGService)
		server := NewRAGGRPCServer(mockSvc, logger)

		res, err := server.UploadFile(context.Background(), &pb.FileUploadRequest{
			Filename: "",
			Content:  []byte("test content"),
		})
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.False(t, res.Message)
	})

	t.Run("successful upload with metadata tenant", func(t *testing.T) {
		mockSvc := new(MockRAGService)
		server := NewRAGGRPCServer(mockSvc, logger)

		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("tenant-id", "tenant_xyz"))
		doc := &models.RAGDocument{DocumentID: "doc-1", ChunkCount: 1}
		mockSvc.On("ProcessDocument", mock.Anything, "tenant_xyz", "rag-corpus", "file.txt", []byte("valid content")).Return(doc, nil)

		res, err := server.UploadFile(ctx, &pb.FileUploadRequest{
			Filename: "file.txt",
			Content:  []byte("valid content"),
		})
		assert.NoError(t, err)
		assert.True(t, res.Message)
		mockSvc.AssertExpectations(t)
	})
}

func TestRAGGRPCServer_QueryWithContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("empty query returns InvalidArgument", func(t *testing.T) {
		mockSvc := new(MockRAGService)
		server := NewRAGGRPCServer(mockSvc, logger)

		res, err := server.QueryWithContext(context.Background(), &pb.QueryRequest{Query: "  "})
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("successful query returns answer with tenant isolation", func(t *testing.T) {
		mockSvc := new(MockRAGService)
		server := NewRAGGRPCServer(mockSvc, logger)

		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("tenant-id", "tenant_abc"))
		mockSvc.On("ProcessQuery", mock.Anything, "tenant_abc", "rag-corpus", "How does RAG work?").Return("RAG retrieves context before generation.", nil)

		res, err := server.QueryWithContext(ctx, &pb.QueryRequest{Query: "How does RAG work?"})
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, "RAG retrieves context before generation.", res.Answer)
		mockSvc.AssertExpectations(t)
	})
}
