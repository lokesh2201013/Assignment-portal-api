package controllers

import (
	"context"
	"log/slog"
	"mime/multipart"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/lokesh2201013/apperrors"
	"github.com/lokesh2201013/dto"
	"github.com/lokesh2201013/models"
)

type AssignmentService interface {
	ListAssignments(filters dto.AssignmentFilters) ([]models.Assignment, error)
	AssignToStudents(ctx context.Context, adminID uuid.UUID, req dto.AssignmentCreateRequest) (map[string]interface{}, error)
	SubmitAssignment(data dto.AssignmentSubmissionUpload) (models.SubmitAssignment, error)
	SaveSubmissionFiles(image *multipart.FileHeader, file *multipart.FileHeader, imageDir string, fileDir string) (string, string, error)
	UpdateSubmissions(ctx context.Context, userIDs []uuid.UUID, status string, reason string) error
	GetUserAssignments(userID uuid.UUID) ([]models.SubmitAssignment, error)
	GetPendingSubmissions() ([]models.SubmitAssignment, error)
}

type AssignmentController struct {
	service  AssignmentService
	logger   *slog.Logger
	imageDir string
	fileDir  string
}

func NewAssignmentController(service AssignmentService, logger *slog.Logger, imageDir string, fileDir string) *AssignmentController {
	return &AssignmentController{service: service, logger: logger, imageDir: imageDir, fileDir: fileDir}
}

func (h *AssignmentController) GetAdminAssignments(c *fiber.Ctx) error {
	assignments, err := h.service.ListAssignments(dto.AssignmentFilters{
		Branch:      c.Query("branch"),
		Semester:    c.Query("semester"),
		SubjectCode: c.Query("subject_code"),
	})
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(assignments)
}

func (h *AssignmentController) AcceptAssignment(c *fiber.Ctx) error {
	userIDs, err := parseUUIDList(c.Query("id"))
	if err != nil {
		return respondError(c, h.logger, err)
	}
	if err := h.service.UpdateSubmissions(c.UserContext(), userIDs, "accepted", ""); err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(fiber.Map{"message": "Assignments Accepted"})
}

func (h *AssignmentController) RejectAssignment(c *fiber.Ctx) error {
	userIDs, err := parseUUIDList(c.Query("id"))
	if err != nil {
		return respondError(c, h.logger, err)
	}
	if err := h.service.UpdateSubmissions(c.UserContext(), userIDs, "rejected", c.Query("reason")); err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(fiber.Map{"message": "Assignments Rejected"})
}

func (h *AssignmentController) GetUserAssignments(c *fiber.Ctx) error {
	userID, err := uuid.Parse(c.Params("user_id"))
	if err != nil {
		return respondError(c, h.logger, apperrors.New(apperrors.ErrValidation, "Invalid userID"))
	}
	assignments, err := h.service.GetUserAssignments(userID)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(assignments)
}

func (h *AssignmentController) GetSubmittedAssignments(c *fiber.Ctx) error {
	if c.Query("assignment_id") == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No assignment ID provided"})
	}
	submissions, err := h.service.GetPendingSubmissions()
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(submissions)
}

func parseUUIDList(idList string) ([]uuid.UUID, error) {
	if strings.TrimSpace(idList) == "" {
		return nil, apperrors.New(apperrors.ErrValidation, "No student IDs provided")
	}
	parts := strings.Split(idList, ",")
	ids := make([]uuid.UUID, 0, len(parts))
	for _, part := range parts {
		id, err := uuid.Parse(strings.TrimSpace(part))
		if err != nil {
			return nil, apperrors.New(apperrors.ErrValidation, "Invalid student ID")
		}
		ids = append(ids, id)
	}
	return ids, nil
}
