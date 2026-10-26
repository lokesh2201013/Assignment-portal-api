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

type AssignmentNotifier interface {
	SendAssignmentNotification(ctx context.Context, req *pb.AssignmentEmailRequest) (string, error)
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
	FindAssignmentInTenant(tenantID string, assignmentID uuid.UUID) (models.Assignment, error)
	FindSubmissionForUserInTenant(tenantID string, assignmentID uuid.UUID, userID uuid.UUID) (models.SubmitAssignment, error)
	SaveStudentAttemptInTenant(tenantID string, submission models.SubmitAssignment, archivePrevious bool) (models.SubmitAssignment, error)
	GradeSubmissionInTenant(tenantID string, submission models.SubmitAssignment, scores []models.RubricScore) error
	FindRevisionsInTenant(tenantID string, submissionID uuid.UUID, userID uuid.UUID) ([]models.SubmissionRevision, error)
	FindDashboardItemsInTenant(tenantID string, userID uuid.UUID) ([]models.DashboardItem, error)
}

type tenantAssignmentUserRepository interface {
	FindStudentsInTenant(tenantID string, branch string, semester int) ([]models.User, error)
	FindByIDsInTenant(tenantID string, ids []uuid.UUID) ([]models.User, error)
	FindStudentsByCourseInTenant(tenantID string, courseID uuid.UUID) ([]models.User, error)
}

type uploadedFileSaver interface {
	Save(file *multipart.FileHeader, dir string) (string, error)
}

type tenantUploadedFileSaver interface {
	Delete(ctx context.Context, objectRef string) error
	SignedDownloadURLForTenant(tenantID string, objectRef string, expires time.Duration) (string, error)
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

func (s *AssignmentService) ListAssignmentsInTenant(tenantID string, filters dto.AssignmentFilters) ([]models.Assignment, dto.PageMetadata, error) {
	repo, ok := s.assignments.(tenantAssignmentRepository)
	if !ok {
		assignments, err := s.assignments.List(filters)
		return assignments, pageMetadata(1, len(assignments), len(assignments)), err
	}
	page, limit := filters.Page, filters.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 25
	} else if limit > 100 {
		limit = 100
	}
	filters.Page, filters.Limit = page, limit
	assignments, total, err := repo.ListPageInTenant(tenantID, filters)
	return assignments, pageMetadata(page, limit, total), err
}

func (s *AssignmentService) AssignToStudents(ctx context.Context, tenantID string, adminID uuid.UUID, req dto.AssignmentCreateRequest) (map[string]interface{}, error) {
	req.Branch = strings.ToUpper(strings.TrimSpace(req.Branch))
	req.Email = strings.TrimSpace(req.Email)
	req.Task = strings.TrimSpace(req.Task)
	req.SubjectCode = strings.TrimSpace(req.SubjectCode)

	if req.Task == "" || req.SubjectCode == "" || req.Email == "" {
		return nil, apperrors.New(apperrors.ErrValidation, "Email, task, and subject_code are required")
	}
	if req.CourseID == nil {
		if req.Branch == "" {
			return nil, apperrors.New(apperrors.ErrValidation, "Branch and semester are required unless course_id is provided")
		}
		if req.Semester < 1 || req.Semester > 20 {
			return nil, apperrors.New(apperrors.ErrValidation, "Invalid semester. Must be between 1 and 20.")
		}
	}
	if _, err := time.Parse(time.RFC3339, req.DueDate); err != nil {
		return nil, apperrors.New(apperrors.ErrValidation, "Invalid due_date format, must be RFC3339")
	}
	latePolicy, maxScore, maxAttempts, reminderHours, penalty, err := normalizeAssignmentPolicy(req)
	if err != nil {
		return nil, err
	}
	rubrics, err := normalizeRubrics(req.Rubrics, maxScore)
	if err != nil {
		return nil, err
	}

	now := time.Now().Format(time.RFC3339)
	assignment := models.Assignment{
		AssignmentID:       uuid.New(),
		TenantID:           tenantID,
		Email:              req.Email,
		AdminID:            adminID,
		Task:               req.Task,
		CreatedAt:          now,
		UpdatedAt:          now,
		DueDate:            req.DueDate,
		Branch:             req.Branch,
		Semester:           req.Semester,
		SubjectCode:        req.SubjectCode,
		CourseID:           req.CourseID,
		MaxScore:           maxScore,
		LatePolicy:         latePolicy,
		LatePenaltyPercent: penalty,
		AllowResubmission:  req.AllowResubmission,
		MaxAttempts:        maxAttempts,
		ReminderHours:      reminderHours,
		Rubrics:            rubrics,
	}

	var students []models.User
	users, ok := s.users.(tenantAssignmentUserRepository)
	if req.CourseID != nil {
		if !ok {
			return nil, apperrors.New(apperrors.ErrUnexpected, "Course-scoped assignment is unavailable")
		}
		students, err = users.FindStudentsByCourseInTenant(tenantID, *req.CourseID)
	} else if ok {
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
			SubmissionID:  uuid.New(),
			TenantID:      tenantID,
			AssignmentID:  assignment.AssignmentID,
			UserID:        student.UserID,
			Status:        "pending",
			CreatedAt:     now,
			AttemptNumber: 1,
		})
	}

	if repo, ok := s.assignments.(tenantAssignmentRepository); ok {
		if err := repo.CreateWithSubmissionsInTenant(tenantID, &assignment, submissions); err != nil {
			return nil, err
		}
	} else if err := s.assignments.CreateWithSubmissions(&assignment, submissions); err != nil {
		return nil, err
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
	now := time.Now()
	submission := models.SubmitAssignment{
		SubmissionID:  uuid.New(),
		TenantID:      data.TenantID,
		AssignmentID:  data.AssignmentID,
		UserID:        data.UserID,
		File:          data.FilePath,
		Image:         data.ImagePath,
		Comments:      data.Comments,
		CreatedAt:     now.Format(time.RFC3339),
		Status:        "pending",
		AttemptNumber: 1,
	}
	repo, ok := s.assignments.(tenantAssignmentRepository)
	if !ok {
		if err := s.assignments.CreateSubmission(&submission); err != nil {
			return models.SubmitAssignment{}, err
		}
		return submission, nil
	}
	assignment, err := repo.FindAssignmentInTenant(data.TenantID, data.AssignmentID)
	if err != nil {
		s.deleteUploaded(data.FilePath, data.ImagePath)
		return models.SubmitAssignment{}, err
	}
	late, err := evaluateLatePolicy(assignment.LatePolicy, assignment.DueDate, now)
	if err != nil {
		s.deleteUploaded(data.FilePath, data.ImagePath)
		return models.SubmitAssignment{}, err
	}
	current, err := repo.FindSubmissionForUserInTenant(data.TenantID, data.AssignmentID, data.UserID)
	if err != nil {
		s.deleteUploaded(data.FilePath, data.ImagePath)
		return models.SubmitAssignment{}, err
	}
	hasFile := strings.TrimSpace(current.File) != ""
	if err := canResubmit(assignment.AllowResubmission, assignment.MaxAttempts, current.AttemptNumber, hasFile, current.Status); err != nil {
		s.deleteUploaded(data.FilePath, data.ImagePath)
		return models.SubmitAssignment{}, err
	}
	submission.LateSubmission = late
	submission.AttemptNumber = current.AttemptNumber
	if hasFile {
		submission.AttemptNumber = current.AttemptNumber + 1
	}
	saved, err := repo.SaveStudentAttemptInTenant(data.TenantID, submission, hasFile)
	if err != nil {
		s.deleteUploaded(data.FilePath, data.ImagePath)
		return models.SubmitAssignment{}, err
	}
	return saved, nil
}

func (s *AssignmentService) deleteUploaded(filePath, imagePath string) {
	if saver, ok := s.fileSaver.(tenantUploadedFileSaver); ok {
		_ = saver.Delete(context.Background(), filePath)
		_ = saver.Delete(context.Background(), imagePath)
	}
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
		if saver, ok := s.fileSaver.(interface {
			Delete(context.Context, string) error
		}); ok {
			_ = saver.Delete(context.Background(), imagePath)
		}
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

func pageMetadata(page, limit, total int) dto.PageMetadata {
	pages := 0
	if total > 0 && limit > 0 {
		pages = (total + limit - 1) / limit
	}
	return dto.PageMetadata{Page: page, Limit: limit, Total: total, TotalPages: pages}
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

func (s *AssignmentService) GradeSubmissionInTenant(tenantID string, graderID uuid.UUID, req dto.GradeSubmissionRequest) (models.SubmitAssignment, error) {
	repo, ok := s.assignments.(tenantAssignmentRepository)
	if !ok {
		return models.SubmitAssignment{}, apperrors.New(apperrors.ErrUnexpected, "Grading is unavailable")
	}
	if req.AssignmentID == uuid.Nil || req.UserID == uuid.Nil {
		return models.SubmitAssignment{}, apperrors.New(apperrors.ErrValidation, "assignment_id and user_id are required")
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" {
		status = "accepted"
	}
	if status != "accepted" && status != "rejected" {
		return models.SubmitAssignment{}, apperrors.New(apperrors.ErrValidation, "status must be accepted or rejected")
	}
	assignment, err := repo.FindAssignmentInTenant(tenantID, req.AssignmentID)
	if err != nil {
		return models.SubmitAssignment{}, err
	}
	current, err := repo.FindSubmissionForUserInTenant(tenantID, req.AssignmentID, req.UserID)
	if err != nil {
		return models.SubmitAssignment{}, err
	}
	if strings.TrimSpace(current.File) == "" {
		return models.SubmitAssignment{}, apperrors.New(apperrors.ErrValidation, "Student has not submitted this assignment")
	}
	score, scores, err := resolveGrade(assignment, req)
	if err != nil {
		return models.SubmitAssignment{}, err
	}
	if current.LateSubmission && assignment.LatePolicy == models.LatePolicyPenalty && score != nil {
		adjusted := applyLatePenalty(*score, assignment.LatePenaltyPercent)
		score = &adjusted
	}
	now := time.Now().Format(time.RFC3339)
	current.Status = status
	current.Score = score
	current.Feedback = strings.TrimSpace(req.Feedback)
	current.GradedAt = now
	current.GradedBy = &graderID
	if err := repo.GradeSubmissionInTenant(tenantID, current, scores); err != nil {
		return models.SubmitAssignment{}, err
	}
	return current, nil
}

func (s *AssignmentService) GetLearnerDashboardInTenant(tenantID string, userID uuid.UUID) (models.LearnerDashboard, error) {
	if userID == uuid.Nil {
		return models.LearnerDashboard{}, apperrors.New(apperrors.ErrValidation, "No userID sent")
	}
	repo, ok := s.assignments.(tenantAssignmentRepository)
	if !ok {
		return models.LearnerDashboard{}, apperrors.New(apperrors.ErrUnexpected, "Learner dashboard is unavailable")
	}
	items, err := repo.FindDashboardItemsInTenant(tenantID, userID)
	if err != nil {
		return models.LearnerDashboard{}, err
	}
	return buildLearnerDashboard(items), nil
}

func (s *AssignmentService) GetSubmissionRevisionsInTenant(tenantID string, userID uuid.UUID, submissionID uuid.UUID) ([]models.SubmissionRevision, error) {
	repo, ok := s.assignments.(tenantAssignmentRepository)
	if !ok {
		return nil, apperrors.New(apperrors.ErrUnexpected, "Revision history is unavailable")
	}
	return repo.FindRevisionsInTenant(tenantID, submissionID, userID)
}

func normalizeAssignmentPolicy(req dto.AssignmentCreateRequest) (string, float64, int, int, float64, error) {
	policy := strings.ToLower(strings.TrimSpace(req.LatePolicy))
	if policy == "" {
		policy = models.LatePolicyAllow
	}
	if policy != models.LatePolicyAllow && policy != models.LatePolicyReject && policy != models.LatePolicyPenalty {
		return "", 0, 0, 0, 0, apperrors.New(apperrors.ErrValidation, "late_policy must be allow, reject, or penalty")
	}
	maxScore := req.MaxScore
	if maxScore == 0 {
		maxScore = 100
	}
	if maxScore <= 0 {
		return "", 0, 0, 0, 0, apperrors.New(apperrors.ErrValidation, "max_score must be greater than zero")
	}
	maxAttempts := req.MaxAttempts
	if maxAttempts == 0 {
		if req.AllowResubmission {
			maxAttempts = 3
		} else {
			maxAttempts = 1
		}
	}
	if maxAttempts < 1 || maxAttempts > 20 {
		return "", 0, 0, 0, 0, apperrors.New(apperrors.ErrValidation, "max_attempts must be between 1 and 20")
	}
	reminderHours := req.ReminderHours
	if reminderHours == 0 {
		reminderHours = 48
	}
	if reminderHours < 1 || reminderHours > 720 {
		return "", 0, 0, 0, 0, apperrors.New(apperrors.ErrValidation, "reminder_hours must be between 1 and 720")
	}
	if req.LatePenaltyPercent < 0 || req.LatePenaltyPercent > 100 {
		return "", 0, 0, 0, 0, apperrors.New(apperrors.ErrValidation, "late_penalty_percent must be between 0 and 100")
	}
	if policy == models.LatePolicyPenalty && req.LatePenaltyPercent == 0 {
		return "", 0, 0, 0, 0, apperrors.New(apperrors.ErrValidation, "late_penalty_percent is required when late_policy is penalty")
	}
	return policy, maxScore, maxAttempts, reminderHours, req.LatePenaltyPercent, nil
}

func normalizeRubrics(items []dto.RubricRequest, maxScore float64) ([]models.Rubric, error) {
	rubrics := make([]models.Rubric, 0, len(items))
	var total float64
	for _, item := range items {
		criterion := strings.TrimSpace(item.Criterion)
		if criterion == "" {
			return nil, apperrors.New(apperrors.ErrValidation, "Each rubric criterion requires a name")
		}
		if item.MaxPoints <= 0 {
			return nil, apperrors.New(apperrors.ErrValidation, "Each rubric criterion requires max_points greater than zero")
		}
		total += item.MaxPoints
		rubrics = append(rubrics, models.Rubric{
			RubricID:    uuid.New(),
			Criterion:   criterion,
			Description: strings.TrimSpace(item.Description),
			MaxPoints:   item.MaxPoints,
		})
	}
	if len(rubrics) > 0 && total-maxScore > 0.01 {
		return nil, apperrors.New(apperrors.ErrValidation, "Rubric points cannot exceed max_score")
	}
	return rubrics, nil
}

func evaluateLatePolicy(policy, dueDate string, now time.Time) (bool, error) {
	due, err := time.Parse(time.RFC3339, dueDate)
	if err != nil {
		return false, apperrors.New(apperrors.ErrValidation, "Assignment due date is invalid")
	}
	late := now.After(due)
	if !late {
		return false, nil
	}
	switch policy {
	case models.LatePolicyReject:
		return true, apperrors.New(apperrors.ErrForbidden, "Late submissions are not accepted for this assignment")
	case models.LatePolicyAllow, models.LatePolicyPenalty, "":
		return true, nil
	default:
		return true, nil
	}
}

func canResubmit(allow bool, maxAttempts, attempt int, hasFile bool, status string) error {
	if !hasFile {
		return nil
	}
	if !allow {
		return apperrors.New(apperrors.ErrForbidden, "Resubmission is not allowed for this assignment")
	}
	if status == "accepted" {
		return apperrors.New(apperrors.ErrForbidden, "Accepted work cannot be resubmitted")
	}
	if attempt >= maxAttempts {
		return apperrors.New(apperrors.ErrForbidden, "Maximum submission attempts reached")
	}
	return nil
}

func applyLatePenalty(score, percent float64) float64 {
	adjusted := score * (100 - percent) / 100
	if adjusted < 0 {
		return 0
	}
	return adjusted
}

func resolveGrade(assignment models.Assignment, req dto.GradeSubmissionRequest) (*float64, []models.RubricScore, error) {
	rubricsByID := map[uuid.UUID]models.Rubric{}
	for _, rubric := range assignment.Rubrics {
		rubricsByID[rubric.RubricID] = rubric
	}
	scores := make([]models.RubricScore, 0, len(req.RubricScores))
	var total float64
	for _, item := range req.RubricScores {
		rubric, ok := rubricsByID[item.RubricID]
		if !ok {
			return nil, nil, apperrors.New(apperrors.ErrValidation, "Unknown rubric criterion")
		}
		if item.Points < 0 || item.Points > rubric.MaxPoints {
			return nil, nil, apperrors.New(apperrors.ErrValidation, "Rubric points must be between 0 and the criterion maximum")
		}
		total += item.Points
		scores = append(scores, models.RubricScore{RubricID: item.RubricID, Criterion: rubric.Criterion, MaxPoints: rubric.MaxPoints, Points: item.Points})
	}
	score := req.Score
	if len(scores) > 0 {
		score = &total
	}
	if score != nil && (*score < 0 || *score > assignment.MaxScore) {
		return nil, nil, apperrors.New(apperrors.ErrValidation, "Score must be between 0 and max_score")
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" {
		status = "accepted"
	}
	if status == "accepted" && score == nil {
		return nil, nil, apperrors.New(apperrors.ErrValidation, "Accepted work requires a score or rubric scores")
	}
	return score, scores, nil
}

func buildLearnerDashboard(items []models.DashboardItem) models.LearnerDashboard {
	dashboard := models.LearnerDashboard{
		Upcoming:      []models.DashboardItem{},
		Overdue:       []models.DashboardItem{},
		PendingReview: []models.DashboardItem{},
		Graded:        []models.DashboardItem{},
		Reminders:     []models.DashboardItem{},
	}
	for _, item := range items {
		switch {
		case item.Status == "accepted" || item.Status == "rejected":
			dashboard.Graded = append(dashboard.Graded, item)
			dashboard.Summary.Graded++
		case !item.Submitted && item.HoursUntilDue < 0:
			dashboard.Overdue = append(dashboard.Overdue, item)
			dashboard.Summary.Overdue++
		case item.Submitted && item.Status == "pending":
			dashboard.PendingReview = append(dashboard.PendingReview, item)
			dashboard.Summary.PendingReview++
		default:
			dashboard.Upcoming = append(dashboard.Upcoming, item)
			dashboard.Summary.Upcoming++
		}
		if !item.Submitted && item.HoursUntilDue >= 0 && item.HoursUntilDue <= float64(item.ReminderHours) {
			dashboard.Reminders = append(dashboard.Reminders, item)
		}
	}
	return dashboard
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
