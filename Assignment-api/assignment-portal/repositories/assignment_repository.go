package repositories

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/lokesh2201013/apperrors"
	"github.com/lokesh2201013/dto"
	"github.com/lokesh2201013/models"
)

type AssignmentRepository struct {
	db *sql.DB
}

func NewAssignmentRepository(db *sql.DB) *AssignmentRepository {
	return &AssignmentRepository{db: db}
}

func (r *AssignmentRepository) List(filters dto.AssignmentFilters) ([]models.Assignment, error) {
	var assignments []models.Assignment
	query := `SELECT assignment_id, email, admin_id, task, created_at, updated_at, due_date, branch, semester, subject_code FROM assignments`
	conditions := []string{}
	args := []interface{}{}
	if filters.Branch != "" {
		args = append(args, filters.Branch)
		conditions = append(conditions, "$"+strconv.Itoa(len(args))+" = branch")
	}
	if filters.Semester != "" {
		args = append(args, filters.Semester)
		conditions = append(conditions, "$"+strconv.Itoa(len(args))+" = semester")
	}
	if filters.SubjectCode != "" {
		args = append(args, filters.SubjectCode)
		conditions = append(conditions, "$"+strconv.Itoa(len(args))+" = subject_code")
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching assignments", err)
	}
	defer rows.Close()
	for rows.Next() {
		var assignment models.Assignment
		if err := rows.Scan(&assignment.AssignmentID, &assignment.Email, &assignment.AdminID, &assignment.Task, &assignment.CreatedAt, &assignment.UpdatedAt, &assignment.DueDate, &assignment.Branch, &assignment.Semester, &assignment.SubjectCode); err != nil {
			return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching assignments", err)
		}
		assignments = append(assignments, assignment)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching assignments", err)
	}
	return assignments, nil
}

func (r *AssignmentRepository) CreateWithSubmissions(assignment *models.Assignment, submissions []models.SubmitAssignment) error {
	tx, err := r.db.Begin()
	if err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Error creating assignment", err)
	}
	if _, err := tx.Exec(`INSERT INTO assignments (assignment_id, email, admin_id, task, created_at, updated_at, due_date, branch, semester, subject_code) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`, assignment.AssignmentID, assignment.Email, assignment.AdminID, assignment.Task, assignment.CreatedAt, assignment.UpdatedAt, assignment.DueDate, assignment.Branch, assignment.Semester, assignment.SubjectCode); err != nil {
		tx.Rollback()
		return apperrors.Wrap(apperrors.ErrDatabase, "Error creating assignment", err)
	}
	for _, submission := range submissions {
		if _, err := tx.Exec(`INSERT INTO submit_assignments (submission_id, assignment_id, user_id, status, file, image, comments, late_submission, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, submission.SubmissionID, submission.AssignmentID, submission.UserID, submission.Status, submission.File, submission.Image, submission.Comments, submission.LateSubmission, submission.CreatedAt); err != nil {
			tx.Rollback()
			return apperrors.Wrap(apperrors.ErrDatabase, "Error creating submissions", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Error creating assignment", err)
	}
	return nil
}

func (r *AssignmentRepository) CreateSubmission(submission *models.SubmitAssignment) error {
	if _, err := r.db.Exec(`INSERT INTO submit_assignments (submission_id, assignment_id, user_id, status, file, image, comments, late_submission, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, submission.SubmissionID, submission.AssignmentID, submission.UserID, submission.Status, submission.File, submission.Image, submission.Comments, submission.LateSubmission, submission.CreatedAt); err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Error saving submission", err)
	}
	return nil
}

func (r *AssignmentRepository) FindSubmissionsByUsers(userIDs []uuid.UUID) ([]models.SubmitAssignment, error) {
	var submissions []models.SubmitAssignment
	if len(userIDs) == 0 {
		return submissions, nil
	}
	placeholders := make([]string, len(userIDs))
	args := make([]interface{}, len(userIDs))
	for i, userID := range userIDs {
		placeholders[i] = "$" + strconv.Itoa(i+1)
		args[i] = userID
	}
	rows, err := r.db.Query(`SELECT submission_id, assignment_id, user_id, status, file, image, comments, late_submission, created_at FROM submit_assignments WHERE user_id IN (`+strings.Join(placeholders, ",")+")", args...)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error getting submitted assignments", err)
	}
	defer rows.Close()
	for rows.Next() {
		var submission models.SubmitAssignment
		if err := rows.Scan(&submission.SubmissionID, &submission.AssignmentID, &submission.UserID, &submission.Status, &submission.File, &submission.Image, &submission.Comments, &submission.LateSubmission, &submission.CreatedAt); err != nil {
			return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error getting submitted assignments", err)
		}
		submissions = append(submissions, submission)
	}
	return submissions, nil
}

func (r *AssignmentRepository) UpdateSubmissionStatus(userIDs []uuid.UUID, status string) error {
	if len(userIDs) == 0 {
		return nil
	}
	placeholders := make([]string, len(userIDs))
	args := []interface{}{status}
	for i, userID := range userIDs {
		placeholders[i] = "$" + strconv.Itoa(i+2)
		args = append(args, userID)
	}
	if _, err := r.db.Exec(`UPDATE submit_assignments SET status = $1 WHERE user_id IN (`+strings.Join(placeholders, ",")+")", args...); err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Error updating assignment status", err)
	}
	return nil
}

func (r *AssignmentRepository) FindSubmissionsByUser(userID uuid.UUID) ([]models.SubmitAssignment, error) {
	var submissions []models.SubmitAssignment
	rows, err := r.db.Query(`SELECT submission_id, assignment_id, user_id, status, file, image, comments, late_submission, created_at FROM submit_assignments WHERE user_id = $1`, userID)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching user assignments", err)
	}
	defer rows.Close()
	for rows.Next() {
		var submission models.SubmitAssignment
		if err := rows.Scan(&submission.SubmissionID, &submission.AssignmentID, &submission.UserID, &submission.Status, &submission.File, &submission.Image, &submission.Comments, &submission.LateSubmission, &submission.CreatedAt); err != nil {
			return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching user assignments", err)
		}
		submissions = append(submissions, submission)
	}
	return submissions, nil
}

func (r *AssignmentRepository) FindPendingSubmissions() ([]models.SubmitAssignment, error) {
	var submissions []models.SubmitAssignment
	rows, err := r.db.Query(`SELECT submission_id, assignment_id, user_id, status, file, image, comments, late_submission, created_at FROM submit_assignments WHERE status = $1`, "pending")
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching submissions", err)
	}
	defer rows.Close()
	for rows.Next() {
		var submission models.SubmitAssignment
		if err := rows.Scan(&submission.SubmissionID, &submission.AssignmentID, &submission.UserID, &submission.Status, &submission.File, &submission.Image, &submission.Comments, &submission.LateSubmission, &submission.CreatedAt); err != nil {
			return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching submissions", err)
		}
		submissions = append(submissions, submission)
	}
	return submissions, nil
}
