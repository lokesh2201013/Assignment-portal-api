package controllers

import (
	"context"
	"log/slog"
	"mime/multipart"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/lokesh2201013/dto"
)

type AidService interface {
	UploadFiles(ctx context.Context, files []*multipart.FileHeader) error
	GetHelp(ctx context.Context, query string) (string, error)
	QueryData(ctx context.Context, naturalLanguageQuery string) ([]map[string]interface{}, string, error)
	CreatePresignedVideoURL(ctx context.Context, req dto.VideoPresignRequest) (string, uuid.UUID, error)
	StartVideoProcessing(id string) error
}

type AidController struct {
	service AidService
	logger  *slog.Logger
}

func NewAidController(service AidService, logger *slog.Logger) *AidController {
	return &AidController{service: service, logger: logger}
}

func (h *AidController) UploadFileHandler(c *fiber.Ctx) error {
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse multipart form",
		})
	}

	if err := h.service.UploadFiles(c.UserContext(), form.File["files"]); err != nil {
		return respondError(c, h.logger, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Files processed successfully",
	})
}

func (h *AidController) GetHelp(c *fiber.Ctx) error {
	type QueryRequest struct {
		Query string `json:"query"`
	}
	var queryRequest QueryRequest
	if err := c.BodyParser(&queryRequest); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	answer, err := h.service.GetHelp(c.UserContext(), queryRequest.Query)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(fiber.Map{"answer": answer})
}

func (h *AidController) GetData(c *fiber.Ctx) error {
	type QueryRequest struct {
		Query string `json:"query"`
	}

	var queryRequest QueryRequest

	if err := c.BodyParser(&queryRequest); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	results, query, err := h.service.QueryData(c.UserContext(), queryRequest.Query)
	if err != nil {
		return respondError(c, h.logger, err)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data":  results,
		"query": query,
	})
}

func (h *AidController) GetPresignedURL(c *fiber.Ctx) error {
	var req dto.VideoPresignRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	url, videoID, err := h.service.CreatePresignedVideoURL(c.UserContext(), req)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"presigned_url": url,
		"video_id":      videoID.String(),
	})
}

func (h *AidController) UpdateVideoStatus(c *fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	if id == "" {
		id = strings.TrimSpace(c.Query("id"))
	}
	if err := h.service.StartVideoProcessing(id); err != nil {
		return respondError(c, h.logger, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Video processing started",
	})
}
