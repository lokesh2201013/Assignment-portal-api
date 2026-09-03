package services

import (
	"context"
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

type uploadedFileSaver interface {
	Save(file *multipart.FileHeader, dir string) (string, error)
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

func (s *AssignmentService) AssignToStudents(ctx context.Context, adminID uuid.UUID, req dto.AssignmentCreateRequest) (map[string]interface{}, error) {
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

	now := time.Now().Format(time.RFC3339)
	assignment := models.Assignment{
		AssignmentID: uuid.New(),
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

	students, err := s.users.FindStudents(assignment.Branch, assignment.Semester)
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
			AssignmentID: assignment.AssignmentID,
			UserID:       student.UserID,
			Status:       "pending",
			CreatedAt:    now,
		})
	}

	if err := s.assignments.CreateWithSubmissions(&assignment, submissions); err != nil {
		return nil, err
	}

	mailStatus, err := s.notifier.SendAssignmentNotification(ctx, assignmentAssignedEmail(assignment))
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
	dueDate, err := time.Parse(time.RFC3339, data.DueDate)
	if err != nil {
		return models.SubmitAssignment{}, apperrors.New(apperrors.ErrValidation, "Invalid due_date format, must be RFC3339")
	}

	submission := models.SubmitAssignment{
		SubmissionID:   uuid.New(),
		AssignmentID:   data.AssignmentID,
		UserID:         data.UserID,
		File:           data.FilePath,
		Image:          data.ImagePath,
		Comments:       data.Comments,
		LateSubmission: time.Now().After(dueDate),
		CreatedAt:      time.Now().Format(time.RFC3339),
	}
	if err := s.assignments.CreateSubmission(&submission); err != nil {
		return models.SubmitAssignment{}, err
	}
	return submission, nil
}

func (s *AssignmentService) SaveSubmissionFiles(image *multipart.FileHeader, file *multipart.FileHeader, imageDir string, fileDir string) (string, string, error) {
	if image == nil {
		return "", "", apperrors.New(apperrors.ErrValidation, "Image file is required")
	}
	if file == nil {
		return "", "", apperrors.New(apperrors.ErrValidation, "File upload is required")
	}
	imagePath, err := s.fileSaver.Save(image, imageDir)
	if err != nil {
		return "", "", apperrors.Wrap(apperrors.ErrUnexpected, "Failed to save image", err)
	}
	filePath, err := s.fileSaver.Save(file, fileDir)
	if err != nil {
		return "", "", apperrors.Wrap(apperrors.ErrUnexpected, "Failed to save file", err)
	}
	return imagePath, filePath, nil
}

func (s *AssignmentService) UpdateSubmissions(ctx context.Context, userIDs []uuid.UUID, status string, reason string) error {
	if status != "accepted" && status != "rejected" {
		return apperrors.New(apperrors.ErrValidation, "Invalid assignment status")
	}
	if len(userIDs) == 0 {
		return apperrors.New(apperrors.ErrValidation, "No student IDs provided")
	}

	submissions, err := s.assignments.FindSubmissionsByUsers(userIDs)
	if err != nil {
		return err
	}
	if len(submissions) == 0 {
		return apperrors.New(apperrors.ErrNotFound, "No submitted assignments found for these users")
	}

	if err := s.assignments.UpdateSubmissionStatus(userIDs, status); err != nil {
		return err
	}

	users, err := s.users.FindByIDs(userIDs)
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
	if userID == uuid.Nil {
		return nil, apperrors.New(apperrors.ErrValidation, "No userID sent")
	}
	return s.assignments.FindSubmissionsByUser(userID)
}

func (s *AssignmentService) GetPendingSubmissions() ([]models.SubmitAssignment, error) {
	return s.assignments.FindPendingSubmissions()
}

func assignmentAssignedEmail(a models.Assignment) *pb.AssignmentEmailRequest {
	return &pb.AssignmentEmailRequest{
		Subject:    fmt.Sprintf("You have a task assigned by Admin %s", a.AdminID.String()),
		Body:       fmt.Sprintf("You have been assigned a new task.\nTask: %s\nBranch: %s\nSemester: %d\nCourseCode: %s\nDue Date: %s", a.Task, a.Branch, a.Semester, a.SubjectCode, a.DueDate),
		Recipients: []string{a.Email},
	}
}

func SafeUploadedFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "\\", "_")
	return strings.ReplaceAll(name, "/", "_")
}
