package controllers

import (
	"context"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	//"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/lokesh2201013/apperrors"
	"github.com/lokesh2201013/database"
	"github.com/lokesh2201013/dto"
	"github.com/lokesh2201013/models"
	pb "github.com/lokesh2201013/proto"
)

type AssignmentService interface {
	ListAssignments(filters dto.AssignmentFilters) ([]models.Assignment, error)
	ListAssignmentsInTenant(tenantID string, filters dto.AssignmentFilters) ([]models.Assignment, dto.PageMetadata, error)
	AssignToStudents(ctx context.Context, tenantID string, adminID uuid.UUID, req dto.AssignmentCreateRequest) (map[string]interface{}, error)
	SubmitAssignment(data dto.AssignmentSubmissionUpload) (models.SubmitAssignment, error)
	SaveSubmissionFiles(image *multipart.FileHeader, file *multipart.FileHeader, imageDir string, fileDir string) (string, string, error)
	UpdateSubmissions(ctx context.Context, userIDs []uuid.UUID, status string, reason string) error
	GetUserAssignments(userID uuid.UUID) ([]models.SubmitAssignment, error)
	GetPendingSubmissions() ([]models.SubmitAssignment, error)
	GetPendingSubmissionsInTenant(tenantID string, assignmentID uuid.UUID) ([]models.SubmitAssignment, error)
	GetPendingSubmissionsPageInTenant(tenantID string, assignmentID uuid.UUID, page int, limit int) ([]models.SubmitAssignment, dto.PageMetadata, error)
	SubmissionDownloadURL(tenantID string, userID uuid.UUID, submissionID uuid.UUID, category string) (string, error)
	GradeSubmissionInTenant(tenantID string, graderID uuid.UUID, req dto.GradeSubmissionRequest) (models.SubmitAssignment, error)
	GetLearnerDashboardInTenant(tenantID string, userID uuid.UUID) (models.LearnerDashboard, error)
	GetSubmissionRevisionsInTenant(tenantID string, userID uuid.UUID, submissionID uuid.UUID) ([]models.SubmissionRevision, error)
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
	assignments, page, err := h.service.ListAssignmentsInTenant(tenantID, dto.AssignmentFilters{
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

func (h *AssignmentController) AssignToStudents(c *fiber.Ctx) error {
	var req dto.AssignmentCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	adminID, err := parseActorID(c)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	tenantID, _ := c.Locals("tenantID").(string)
	result, err := h.service.AssignToStudents(c.UserContext(), tenantID, adminID, req)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(result)
}

func (h *AssignmentController) GetPendingSubmissions(c *fiber.Ctx) error {
	assignmentID, err := uuid.Parse(c.Query("assignment_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid assignment_id"})
	}
	tenantID, _ := c.Locals("tenantID").(string)
	submissions, page, err := h.service.GetPendingSubmissionsPageInTenant(
		tenantID,
		assignmentID,
		c.QueryInt("page", 1),
		c.QueryInt("limit", 25),
	)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(dto.PaginatedResponse[models.SubmitAssignment]{Data: submissions, Page: page})
}

func (h *AssignmentController) UploadSubmission(c *fiber.Ctx) error {
	assignmentID, err := uuid.Parse(c.FormValue("assignment_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid assignment_id"})
	}
	userID, err := parseActorID(c)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	image, err := c.FormFile("image")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Image file is required"})
	}
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "File upload is required"})
	}
	imagePath, filePath, err := h.service.SaveSubmissionFiles(image, file, h.imageDir, h.fileDir)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	tenantID, _ := c.Locals("tenantID").(string)
	submission, err := h.service.SubmitAssignment(dto.AssignmentSubmissionUpload{
		TenantID:     tenantID,
		AssignmentID: assignmentID,
		UserID:       userID,
		FilePath:     filePath,
		ImagePath:    imagePath,
		Comments:     c.FormValue("comments"),
	})
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":    "Assignment submitted successfully",
		"submission": submission,
	})
}

func AcceptAssignment(c *fiber.Ctx) error {
	idList := c.Query("id")
	if idList == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "No student IDs provided"})
	}

	idStrs := strings.Split(idList, ",")
	studentIDs := make([]string, 0, len(idStrs))
	for _, s := range idStrs {
		studentIDs = append(studentIDs, strings.TrimSpace(s))
	}

	// First, find the submitted assignments that match the user IDs.
	var submittedAssignments []models.SubmitAssignment
	if err := database.DB.Where("user_id IN ?", studentIDs).Find(&submittedAssignments).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error getting submitted assignments"})
	}

	if len(submittedAssignments) == 0 {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "No submitted assignments found for these users"})
	}

	assignmentIDs := make([]string, 0)
	for _, submission := range submittedAssignments {
		assignmentIDs = append(assignmentIDs, submission.AssignmentID.String())
	}

	// Now, update the main Assignment records' status.
	if err := database.DB.Model(&models.SubmitAssignment{}).
		Where("user_id IN ?", studentIDs).
		Update("status", "accepted").Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error updating assignment status"})
	}

	// Fetch user info for email notification.
	var UserInfo []models.User
	if err := database.DB.Where("user_id IN ?", studentIDs).Find(&UserInfo).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error getting user info for emails"})
	}

	var emails []string
	for _, user := range UserInfo {
		emails = append(emails, user.Email)
	}

	req := &pb.AssignmentEmailRequest{
		Subject:    fmt.Sprintf("Assignment Accepted"),
		Body:       fmt.Sprintf("Your assignment has been Accepted"),
		Recipients: emails,
	}

	// This function is assumed to be defined elsewhere.
	SendAssignmentNotification(req)

	return c.JSON(fiber.Map{"message": "Assignments Accepted"})
}

func RejectAssignment(c *fiber.Ctx) error {
	// Get comma-separated student IDs from form
	idList := c.Query("id")
	reason := c.Query("reason")
	if idList == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "No student IDs provided"})
	}

	idStrs := strings.Split(idList, ",")
	studentIDs := make([]string, 0, len(idStrs))
	for _, s := range idStrs {
		studentIDs = append(studentIDs, strings.TrimSpace(s))
	}

	// First, find the submitted assignments that match the user IDs.
	var submittedAssignments []models.SubmitAssignment
	if err := database.DB.Where("user_id IN ?", studentIDs).Find(&submittedAssignments).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error getting submitted assignments"})
	}

	if len(submittedAssignments) == 0 {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "No submitted assignments found for these users"})
	}

	assignmentIDs := make([]string, 0)
	for _, submission := range submittedAssignments {
		assignmentIDs = append(assignmentIDs, submission.AssignmentID.String())
	}

	// Now, update the main Assignment records' status.
	if err := database.DB.Model(&models.SubmitAssignment{}).
		Where("user_id IN ?", studentIDs).
		Update("status", "rejected").Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error updating assignment status"})
	}

	// Fetch user info for email notification.
	var UserInfo []models.User
	if err := database.DB.Where("user_id IN ?", studentIDs).Find(&UserInfo).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error getting user info for emails"})
	}

	var emails []string
	for _, user := range UserInfo {
		emails = append(emails, user.Email)
	}

	req := &pb.AssignmentEmailRequest{
		Subject:    fmt.Sprintf("Assignment Rejected"),
		Body:       fmt.Sprintf("Your assignment has been Rejected due to %s", reason),
		Recipients: emails,
	}

	// This function is assumed to be defined elsewhere.
	SendAssignmentNotification(req)

	return c.JSON(fiber.Map{"message": "Assignments Rejected"})
}

func GetUserAssignments(c *fiber.Ctx) error {
	requserID := c.Params("user_id")

	if requserID == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "No userID sent"})
	}

	userID, err := uuid.Parse(requserID)

	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Invalid userID"})
	}

	var assignments []models.SubmitAssignment

	if err := database.DB.Where("user_id = ?", userID).Find(&assignments).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error fetching user assignments"})
	}
	return c.JSON(assignments)
}

func GetSubmittedAssignments(c *fiber.Ctx) error {
	assignmentId := c.Query("assignment_id")
	if assignmentId == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "No assignment ID provided"})
	}

	var submissions []models.SubmitAssignment
	if err := database.DB.Where("status = ?", "pending").Find(&submissions).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error fetching submissions"})
	}
	return c.JSON(dto.PaginatedResponse[models.SubmitAssignment]{Data: submissions, Page: dto.PageMetadata{Page: 1, Limit: len(submissions), Total: len(submissions), TotalPages: 1}})
}

func (h *AssignmentController) GradeSubmission(c *fiber.Ctx) error {
	var req dto.GradeSubmissionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	graderID, err := uuid.Parse(c.Locals("userID").(string))
	if err != nil {
		return respondError(c, h.logger, apperrors.New(apperrors.ErrUnauthorized, "Invalid user identity"))
	}
	tenantID, _ := c.Locals("tenantID").(string)
	submission, err := h.service.GradeSubmissionInTenant(tenantID, graderID, req)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(fiber.Map{"message": "Submission graded", "submission": submission})
}

func (h *AssignmentController) GetLearnerDashboard(c *fiber.Ctx) error {
	userID, err := uuid.Parse(c.Locals("userID").(string))
	if err != nil {
		return respondError(c, h.logger, apperrors.New(apperrors.ErrUnauthorized, "Invalid user identity"))
	}
	tenantID, _ := c.Locals("tenantID").(string)
	dashboard, err := h.service.GetLearnerDashboardInTenant(tenantID, userID)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(dashboard)
}

func (h *AssignmentController) GetSubmissionRevisions(c *fiber.Ctx) error {
	submissionID, err := uuid.Parse(c.Params("submission_id"))
	if err != nil {
		return respondError(c, h.logger, apperrors.New(apperrors.ErrValidation, "Invalid submission_id"))
	}
	userID, err := uuid.Parse(c.Locals("userID").(string))
	if err != nil {
		return respondError(c, h.logger, apperrors.New(apperrors.ErrUnauthorized, "Invalid user identity"))
	}
	tenantID, _ := c.Locals("tenantID").(string)
	revisions, err := h.service.GetSubmissionRevisionsInTenant(tenantID, userID, submissionID)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(fiber.Map{"data": revisions})
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
