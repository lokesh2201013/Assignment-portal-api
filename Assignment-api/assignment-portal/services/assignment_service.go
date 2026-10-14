package services

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lokesh2201013/apperrors"
	"github.com/lokesh2201013/dto"
	"github.com/lokesh2201013/models"
	pb "github.com/lokesh2201013/proto"
)

type assignmentRepository interface {
	List(filters dto.AssignmentFilters) ([]models.Assignment, error)
	CreateWithSubmissions(assignment *models.Assignment, submissions []models.SubmitAssignment) error
	CreateSubmission(submission *models.SubmitAssignment) error
	FindSubmissionsByUsers(userIDs []uuid.UUID) ([]models.SubmitAssignment, error)
	UpdateSubmissionStatus(userIDs []uuid.UUID, status string) error
	FindSubmissionsByUser(userID uuid.UUID) ([]models.SubmitAssignment, error)
	FindPendingSubmissions() ([]models.SubmitAssignment, error)
}

type assignmentUserRepository interface {
	FindStudents(branch string, semester int) ([]models.User, error)
	FindByIDs(ids []uuid.UUID) ([]models.User, error)
}

type tenantAssignmentRepository interface {
	ListInTenant(tenantID string, filters dto.AssignmentFilters) ([]models.Assignment, error)
	ListPageInTenant(tenantID string, filters dto.AssignmentFilters) ([]models.Assignment, int, error)
	CreateWithSubmissionsInTenant(tenantID string, assignment *models.Assignment, submissions []models.SubmitAssignment) error
	FindSubmissionsByUsersInTenant(tenantID string, assignmentID uuid.UUID, userIDs []uuid.UUID) ([]models.SubmitAssignment, error)
	UpdateSubmissionStatusInTenant(tenantID string, assignmentID uuid.UUID, userIDs []uuid.UUID, status string) error
	FindSubmissionsByUserInTenant(tenantID string, userID uuid.UUID) ([]models.SubmitAssignment, error)
	FindSubmissionsByUserPageInTenant(tenantID string, userID uuid.UUID, page int, limit int) ([]models.SubmitAssignment, int, error)
	FindPendingSubmissionsInTenant(tenantID string, assignmentID uuid.UUID) ([]models.SubmitAssignment, error)
	FindPendingSubmissionsPageInTenant(tenantID string, assignmentID uuid.UUID, page int, limit int) ([]models.SubmitAssignment, int, error)
	SubmitAssignedAssignment(tenantID string, submission models.SubmitAssignment) (models.SubmitAssignment, error)
	FindSubmissionFiles(tenantID string, submissionID uuid.UUID, userID uuid.UUID) (string, string, error)
}

type tenantAssignmentUserRepository interface {
	FindStudentsInTenant(tenantID string, branch string, semester int) ([]models.User, error)
	FindByIDsInTenant(tenantID string, ids []uuid.UUID) ([]models.User, error)
}

type uploadedFileSaver interface {
	Save(file *multipart.FileHeader, dir string) (string, error)
}

type tenantUploadedFileSaver interface {
	SaveForTenant(ctx context.Context, tenantID string, file *multipart.FileHeader, category string) (string, error)
	Delete(ctx context.Context, objectRef string) error
	SignedDownloadURLForTenant(tenantID string, objectRef string, expiry time.Duration) (string, error)
}

type AssignmentService struct {
	assignments assignmentRepository
	users       assignmentUserRepository
	notifier    AssignmentNotifier
	fileSaver   uploadedFileSaver
}

func NewAssignmentService(assignments assignmentRepository, users assignmentUserRepository, notifier AssignmentNotifier, fileSaver uploadedFileSaver) *AssignmentService {
	return &AssignmentService{assignments: assignments, users: users, notifier: notifier, fileSaver: fileSaver}
}

func (s *AssignmentService) ListAssignments(filters dto.AssignmentFilters) ([]models.Assignment, error) {
	return s.assignments.List(filters)
}

func (s *AssignmentService) ListAssignmentsInTenant(tenantID string, filters dto.AssignmentFilters) ([]models.Assignment, error) {
	if repo, ok := s.assignments.(tenantAssignmentRepository); ok {
		return repo.ListInTenant(tenantID, filters)
	}
	return s.assignments.List(filters)
}

func (s *AssignmentService) ListAssignmentsPageInTenant(tenantID string, filters dto.AssignmentFilters) ([]models.Assignment, dto.PageMetadata, error) {
	page, limit := filters.Page, filters.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if repo, ok := s.assignments.(tenantAssignmentRepository); ok {
		assignments, total, err := repo.ListPageInTenant(tenantID, filters)
		return assignments, pageMetadata(page, limit, total), err
	}
	assignments, err := s.assignments.List(filters)
	return assignments, pageMetadata(page, limit, len(assignments)), err
}

func pageMetadata(page, limit, total int) dto.PageMetadata {
	pages := 0
	if total > 0 {
		pages = (total + limit - 1) / limit
	}
	return dto.PageMetadata{Page: page, Limit: limit, Total: total, TotalPages: pages}
}

func (s *AssignmentService) AssignToStudents(ctx context.Context, adminID uuid.UUID, req dto.AssignmentCreateRequest) (map[string]interface{}, error) {
	return s.AssignToStudentsInTenant(ctx, "default", adminID, req)
}

func (s *AssignmentService) AssignToStudentsInTenant(ctx context.Context, tenantID string, adminID uuid.UUID, req dto.AssignmentCreateRequest) (map[string]interface{}, error) {
	req.Branch = strings.ToUpper(strings.TrimSpace(req.Branch))
	req.Email = strings.TrimSpace(req.Email)
	req.Task = strings.TrimSpace(req.Task)
	req.SubjectCode = strings.TrimSpace(req.SubjectCode)

	if req.Task == "" || req.Branch == "" || req.SubjectCode == "" || req.Email == "" {
		return nil, apperrors.New(apperrors.ErrValidation, "Email, task, branch, semester, and subject_code are required")
	}
	if req.Semester < 1 || req.Semester > 20 {
		return nil, apperrors.New(apperrors.ErrValidation, "Invalid semester. Must be between 1 and 20.")
	}
	if _, err := time.Parse(time.RFC3339, req.DueDate); err != nil {
		return nil, apperrors.New(apperrors.ErrValidation, "Invalid due_date format, must be RFC3339")
	}

	now := time.Now().Format(time.RFC3339)
	assignment := models.Assignment{
		AssignmentID: uuid.New(),
		TenantID:     tenantID,
		Email:        req.Email,
		AdminID:      adminID,
		Task:         req.Task,
		CreatedAt:    now,
		UpdatedAt:    now,
		DueDate:      req.DueDate,
		Branch:       req.Branch,
		Semester:     req.Semester,
		SubjectCode:  req.SubjectCode,
	}

	var students []models.User
	var err error
	if users, ok := s.users.(tenantAssignmentUserRepository); ok {
		students, err = users.FindStudentsInTenant(tenantID, assignment.Branch, assignment.Semester)
	} else {
		students, err = s.users.FindStudents(assignment.Branch, assignment.Semester)
	}
	if err != nil {
		return nil, err
	}
	if len(students) == 0 {
		return nil, apperrors.New(apperrors.ErrNotFound, "No students found for the given criteria")
	}

	submissions := make([]models.SubmitAssignment, 0, len(students))
	for _, student := range students {
		submissions = append(submissions, models.SubmitAssignment{
			SubmissionID: uuid.New(),
			TenantID:     tenantID,
			AssignmentID: assignment.AssignmentID,
			UserID:       student.UserID,
			Status:       "pending",
			CreatedAt:    now,
		})
	}

	var createErr error
	if repo, ok := s.assignments.(tenantAssignmentRepository); ok {
		createErr = repo.CreateWithSubmissionsInTenant(tenantID, &assignment, submissions)
	} else {
		createErr = s.assignments.CreateWithSubmissions(&assignment, submissions)
	}
	if createErr != nil {
		return nil, createErr
	}

	recipients := make([]string, 0, len(students))
	for _, student := range students {
		recipients = append(recipients, student.Email)
	}
	mailStatus, err := s.notifier.SendAssignmentNotification(ctx, assignmentAssignedEmail(assignment, recipients))
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"message":     "Assignment created successfully",
		"assignment":  assignment,
		"student_cnt": len(students),
		"mail_status": mailStatus,
	}, nil
}

func (s *AssignmentService) SubmitAssignment(data dto.AssignmentSubmissionUpload) (models.SubmitAssignment, error) {
	if data.AssignmentID == uuid.Nil || data.UserID == uuid.Nil {
		return models.SubmitAssignment{}, apperrors.New(apperrors.ErrValidation, "assignment_id and user_id are required")
	}
	submission := models.SubmitAssignment{
		SubmissionID: uuid.New(),
		TenantID:     data.TenantID,
		AssignmentID: data.AssignmentID,
		UserID:       data.UserID,
		File:         data.FilePath,
		Image:        data.ImagePath,
		Comments:     data.Comments,
		CreatedAt:    time.Now().Format(time.RFC3339),
	}
	if repo, ok := s.assignments.(tenantAssignmentRepository); ok {
		saved, err := repo.SubmitAssignedAssignment(data.TenantID, submission)
		if err != nil {
			if saver, ok := s.fileSaver.(tenantUploadedFileSaver); ok {
				_ = saver.Delete(context.Background(), data.FilePath)
				_ = saver.Delete(context.Background(), data.ImagePath)
			}
			return models.SubmitAssignment{}, err
		}
		return saved, nil
	}
	submission.Status = "pending"
	if err := s.assignments.CreateSubmission(&submission); err != nil {
		return models.SubmitAssignment{}, err
	}
	return submission, nil
}

func (s *AssignmentService) SaveSubmissionFiles(image *multipart.FileHeader, file *multipart.FileHeader, imageDir string, fileDir string) (string, string, error) {
	return s.SaveSubmissionFilesForTenant(context.Background(), "default", image, file, imageDir, fileDir)
}

func (s *AssignmentService) SaveSubmissionFilesForTenant(ctx context.Context, tenantID string, image *multipart.FileHeader, file *multipart.FileHeader, imageDir string, fileDir string) (string, string, error) {
	if image == nil {
		return "", "", apperrors.New(apperrors.ErrValidation, "Image file is required")
	}
	if file == nil {
		return "", "", apperrors.New(apperrors.ErrValidation, "File upload is required")
	}
	var imagePath string
	var err error
	if saver, ok := s.fileSaver.(tenantUploadedFileSaver); ok {
		imagePath, err = saver.SaveForTenant(ctx, tenantID, image, "image")
	} else {
		imagePath, err = s.fileSaver.Save(image, imageDir)
	}
	if err != nil {
		if errors.Is(err, apperrors.ErrValidation) || errors.Is(err, apperrors.ErrForbidden) {
			return "", "", err
		}
		return "", "", apperrors.Wrap(apperrors.ErrUnexpected, "Failed to save image", err)
	}
	var filePath string
	if saver, ok := s.fileSaver.(tenantUploadedFileSaver); ok {
		filePath, err = saver.SaveForTenant(ctx, tenantID, file, "submission")
	} else {
		filePath, err = s.fileSaver.Save(file, fileDir)
	}
	if err != nil {
		if saver, ok := s.fileSaver.(tenantUploadedFileSaver); ok {
			_ = saver.Delete(ctx, imagePath)
		}
		if errors.Is(err, apperrors.ErrValidation) || errors.Is(err, apperrors.ErrForbidden) {
			return "", "", err
		}
		return "", "", apperrors.Wrap(apperrors.ErrUnexpected, "Failed to save file", err)
	}
	return imagePath, filePath, nil
}

func (s *AssignmentService) UpdateSubmissions(ctx context.Context, userIDs []uuid.UUID, status string, reason string) error {
	return s.UpdateSubmissionsInTenant(ctx, "default", uuid.Nil, userIDs, status, reason)
}

func (s *AssignmentService) UpdateSubmissionsInTenant(ctx context.Context, tenantID string, assignmentID uuid.UUID, userIDs []uuid.UUID, status string, reason string) error {
	if status != "accepted" && status != "rejected" {
		return apperrors.New(apperrors.ErrValidation, "Invalid assignment status")
	}
	if len(userIDs) == 0 {
		return apperrors.New(apperrors.ErrValidation, "No student IDs provided")
	}

	var submissions []models.SubmitAssignment
	var err error
	if repo, ok := s.assignments.(tenantAssignmentRepository); ok {
		submissions, err = repo.FindSubmissionsByUsersInTenant(tenantID, assignmentID, userIDs)
	} else {
		submissions, err = s.assignments.FindSubmissionsByUsers(userIDs)
	}
	if err != nil {
		return err
	}
	if len(submissions) == 0 {
		return apperrors.New(apperrors.ErrNotFound, "No submitted assignments found for these users")
	}

	var updateErr error
	if repo, ok := s.assignments.(tenantAssignmentRepository); ok {
		updateErr = repo.UpdateSubmissionStatusInTenant(tenantID, assignmentID, userIDs, status)
	} else {
		updateErr = s.assignments.UpdateSubmissionStatus(userIDs, status)
	}
	if updateErr != nil {
		return updateErr
	}

	var users []models.User
	if scopedUsers, ok := s.users.(tenantAssignmentUserRepository); ok {
		users, err = scopedUsers.FindByIDsInTenant(tenantID, userIDs)
	} else {
		users, err = s.users.FindByIDs(userIDs)
	}
	if err != nil {
		return err
	}
	emails := make([]string, 0, len(users))
	for _, user := range users {
		emails = append(emails, user.Email)
	}
	if len(emails) == 0 {
		return nil
	}

	subject := "Assignment Accepted"
	body := "Your assignment has been Accepted"
	if status == "rejected" {
		subject = "Assignment Rejected"
		body = fmt.Sprintf("Your assignment has been Rejected due to %s", reason)
	}
	_, err = s.notifier.SendAssignmentNotification(ctx, &pb.AssignmentEmailRequest{
		Subject:    subject,
		Body:       body,
		Recipients: emails,
	})
	return err
}

func (s *AssignmentService) GetUserAssignments(userID uuid.UUID) ([]models.SubmitAssignment, error) {
	return s.GetUserAssignmentsInTenant("default", userID)
}

func (s *AssignmentService) GetUserAssignmentsInTenant(tenantID string, userID uuid.UUID) ([]models.SubmitAssignment, error) {
	if userID == uuid.Nil {
		return nil, apperrors.New(apperrors.ErrValidation, "No userID sent")
	}
	if repo, ok := s.assignments.(tenantAssignmentRepository); ok {
		return repo.FindSubmissionsByUserInTenant(tenantID, userID)
	}
	return s.assignments.FindSubmissionsByUser(userID)
}

func (s *AssignmentService) GetUserAssignmentsPageInTenant(tenantID string, userID uuid.UUID, page int, limit int) ([]models.SubmitAssignment, dto.PageMetadata, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if repo, ok := s.assignments.(tenantAssignmentRepository); ok {
		submissions, total, err := repo.FindSubmissionsByUserPageInTenant(tenantID, userID, page, limit)
		return submissions, pageMetadata(page, limit, total), err
	}
	submissions, err := s.assignments.FindSubmissionsByUser(userID)
	return submissions, pageMetadata(page, limit, len(submissions)), err
}

func (s *AssignmentService) GetPendingSubmissions() ([]models.SubmitAssignment, error) {
	return s.assignments.FindPendingSubmissions()
}

func (s *AssignmentService) GetPendingSubmissionsInTenant(tenantID string, assignmentID uuid.UUID) ([]models.SubmitAssignment, error) {
	if repo, ok := s.assignments.(tenantAssignmentRepository); ok {
		return repo.FindPendingSubmissionsInTenant(tenantID, assignmentID)
	}
	return s.assignments.FindPendingSubmissions()
}

func (s *AssignmentService) GetPendingSubmissionsPageInTenant(tenantID string, assignmentID uuid.UUID, page int, limit int) ([]models.SubmitAssignment, dto.PageMetadata, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if repo, ok := s.assignments.(tenantAssignmentRepository); ok {
		submissions, total, err := repo.FindPendingSubmissionsPageInTenant(tenantID, assignmentID, page, limit)
		return submissions, pageMetadata(page, limit, total), err
	}
	submissions, err := s.assignments.FindPendingSubmissions()
	return submissions, pageMetadata(page, limit, len(submissions)), err
}

func (s *AssignmentService) SubmissionDownloadURL(tenantID string, userID uuid.UUID, submissionID uuid.UUID, category string) (string, error) {
	repo, ok := s.assignments.(tenantAssignmentRepository)
	if !ok {
		return "", apperrors.New(apperrors.ErrUnexpected, "Submission storage is unavailable")
	}
	saver, ok := s.fileSaver.(tenantUploadedFileSaver)
	if !ok {
		return "", apperrors.New(apperrors.ErrUnexpected, "Signed downloads are unavailable")
	}
	filePath, imagePath, err := repo.FindSubmissionFiles(tenantID, submissionID, userID)
	if err != nil {
		return "", err
	}
	objectRef := filePath
	if category == "image" {
		objectRef = imagePath
	} else if category != "submission" {
		return "", apperrors.New(apperrors.ErrValidation, "file must be submission or image")
	}
	if objectRef == "" {
		return "", apperrors.New(apperrors.ErrNotFound, "Submission file not found")
	}
	return saver.SignedDownloadURLForTenant(tenantID, objectRef, 5*time.Minute)
}

func assignmentAssignedEmail(a models.Assignment, recipients []string) *pb.AssignmentEmailRequest {
	return &pb.AssignmentEmailRequest{
		Subject:    fmt.Sprintf("You have a task assigned by Admin %s", a.AdminID.String()),
		Body:       fmt.Sprintf("You have been assigned a new task.\nTask: %s\nBranch: %s\nSemester: %d\nCourseCode: %s\nDue Date: %s", a.Task, a.Branch, a.Semester, a.SubjectCode, a.DueDate),
		Recipients: recipients,
	}
}

func SafeUploadedFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "\\", "_")
	return strings.ReplaceAll(name, "/", "_")
}
