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
	ListAssignmentsInTenant(tenantID string, filters dto.AssignmentFilters) ([]models.Assignment, error)
	ListAssignmentsPageInTenant(tenantID string, filters dto.AssignmentFilters) ([]models.Assignment, dto.PageMetadata, error)
	AssignToStudents(ctx context.Context, adminID uuid.UUID, req dto.AssignmentCreateRequest) (map[string]interface{}, error)
	AssignToStudentsInTenant(ctx context.Context, tenantID string, adminID uuid.UUID, req dto.AssignmentCreateRequest) (map[string]interface{}, error)
	SubmitAssignment(data dto.AssignmentSubmissionUpload) (models.SubmitAssignment, error)
	SaveSubmissionFiles(image *multipart.FileHeader, file *multipart.FileHeader, imageDir string, fileDir string) (string, string, error)
	SaveSubmissionFilesForTenant(ctx context.Context, tenantID string, image *multipart.FileHeader, file *multipart.FileHeader, imageDir string, fileDir string) (string, string, error)
	UpdateSubmissions(ctx context.Context, userIDs []uuid.UUID, status string, reason string) error
	UpdateSubmissionsInTenant(ctx context.Context, tenantID string, assignmentID uuid.UUID, userIDs []uuid.UUID, status string, reason string) error
	GetUserAssignments(userID uuid.UUID) ([]models.SubmitAssignment, error)
	GetUserAssignmentsInTenant(tenantID string, userID uuid.UUID) ([]models.SubmitAssignment, error)
	GetUserAssignmentsPageInTenant(tenantID string, userID uuid.UUID, page int, limit int) ([]models.SubmitAssignment, dto.PageMetadata, error)
	GetPendingSubmissions() ([]models.SubmitAssignment, error)
	GetPendingSubmissionsInTenant(tenantID string, assignmentID uuid.UUID) ([]models.SubmitAssignment, error)
	GetPendingSubmissionsPageInTenant(tenantID string, assignmentID uuid.UUID, page int, limit int) ([]models.SubmitAssignment, dto.PageMetadata, error)
	SubmissionDownloadURL(tenantID string, userID uuid.UUID, submissionID uuid.UUID, category string) (string, error)
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
	tenantID, _ := c.Locals("tenantID").(string)
	assignments, page, err := h.service.ListAssignmentsPageInTenant(tenantID, dto.AssignmentFilters{
		Branch:      c.Query("branch"),
		Semester:    c.Query("semester"),
		SubjectCode: c.Query("subject_code"),
		Page:        c.QueryInt("page", 1),
		Limit:       c.QueryInt("limit", 25),
	})
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(dto.PaginatedResponse[models.Assignment]{Data: assignments, Page: page})
}

func (h *AssignmentController) AcceptAssignment(c *fiber.Ctx) error {
	assignmentID, err := uuid.Parse(c.Query("assignment_id"))
	if err != nil {
		return respondError(c, h.logger, apperrors.New(apperrors.ErrValidation, "Invalid assignment_id"))
	}
	userIDs, err := parseUUIDList(c.Query("id"))
	if err != nil {
		return respondError(c, h.logger, err)
	}
	tenantID, _ := c.Locals("tenantID").(string)
	if err := h.service.UpdateSubmissionsInTenant(c.UserContext(), tenantID, assignmentID, userIDs, "accepted", ""); err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(fiber.Map{"message": "Assignments Accepted"})
}

func (h *AssignmentController) RejectAssignment(c *fiber.Ctx) error {
	assignmentID, err := uuid.Parse(c.Query("assignment_id"))
	if err != nil {
		return respondError(c, h.logger, apperrors.New(apperrors.ErrValidation, "Invalid assignment_id"))
	}
	userIDs, err := parseUUIDList(c.Query("id"))
	if err != nil {
		return respondError(c, h.logger, err)
	}
	tenantID, _ := c.Locals("tenantID").(string)
	if err := h.service.UpdateSubmissionsInTenant(c.UserContext(), tenantID, assignmentID, userIDs, "rejected", c.Query("reason")); err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(fiber.Map{"message": "Assignments Rejected"})
}

func (h *AssignmentController) GetUserAssignments(c *fiber.Ctx) error {
	userIDValue, _ := c.Locals("userID").(string)
	if pathUserID := c.Params("user_id"); pathUserID != "" && pathUserID != userIDValue {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot access another user's assignments"})
	}
	userID, err := uuid.Parse(userIDValue)
	if err != nil {
		return respondError(c, h.logger, apperrors.New(apperrors.ErrValidation, "Invalid userID"))
	}
	tenantID, _ := c.Locals("tenantID").(string)
	assignments, page, err := h.service.GetUserAssignmentsPageInTenant(tenantID, userID, c.QueryInt("page", 1), c.QueryInt("limit", 25))
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(dto.PaginatedResponse[models.SubmitAssignment]{Data: assignments, Page: page})
}

func (h *AssignmentController) GetSubmittedAssignments(c *fiber.Ctx) error {
	assignmentID, err := uuid.Parse(c.Query("assignment_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Valid assignment_id is required"})
	}
	tenantID, _ := c.Locals("tenantID").(string)
	submissions, page, err := h.service.GetPendingSubmissionsPageInTenant(tenantID, assignmentID, c.QueryInt("page", 1), c.QueryInt("limit", 25))
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(dto.PaginatedResponse[models.SubmitAssignment]{Data: submissions, Page: page})
}

func (h *AssignmentController) GetSubmissionDownloadURL(c *fiber.Ctx) error {
	submissionID, err := uuid.Parse(c.Params("submission_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid submission_id"})
	}
	userID, err := uuid.Parse(c.Locals("userID").(string))
	if err != nil {
		return respondError(c, h.logger, apperrors.New(apperrors.ErrUnauthorized, "Invalid user identity"))
	}
	tenantID, _ := c.Locals("tenantID").(string)
	url, err := h.service.SubmissionDownloadURL(tenantID, userID, submissionID, c.Query("file", "submission"))
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(fiber.Map{"download_url": url, "expires_in_seconds": 300})
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
