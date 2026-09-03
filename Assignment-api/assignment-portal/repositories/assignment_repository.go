package repositories

import (
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lokesh2201013/apperrors"
	"github.com/lokesh2201013/dto"
	"github.com/lokesh2201013/models"
)

type AssignmentRepository struct {
	db *sqlx.DB
}

func NewAssignmentRepository(db *sqlx.DB) *AssignmentRepository {
	return &AssignmentRepository{db: db}
}

func (r *AssignmentRepository) List(filters dto.AssignmentFilters) ([]models.Assignment, error) {
	var assignments []models.Assignment
	query := `SELECT assignment_id, email, admin_id, task, created_at, updated_at, due_date, branch, semester, subject_code FROM assignments`
	conditions := []string{}
	args := map[string]interface{}{}
	if filters.Branch != "" {
		args["branch"] = filters.Branch
		conditions = append(conditions, "branch = :branch")
	}
	if filters.Semester != "" {
		args["semester"] = filters.Semester
		conditions = append(conditions, "semester = :semester")
	}
	if filters.SubjectCode != "" {
		args["subject_code"] = filters.SubjectCode
		conditions = append(conditions, "subject_code = :subject_code")
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	rows, err := r.db.NamedQuery(query, args)
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
	tx, err := r.db.Beginx()
	if err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Error creating assignment", err)
	}
	query := `
		INSERT INTO assignments (
			assignment_id, email, admin_id, task, created_at, 
			updated_at, due_date, branch, semester, subject_code
		) VALUES (
			:assignment_id, :email, :admin_id, :task, :created_at, 
			:updated_at, :due_date, :branch, :semester, :subject_code
		)
	`

	if _, err := tx.NamedExec(query, assignment); err != nil {
		tx.Rollback()
		return apperrors.Wrap(apperrors.ErrDatabase, "Error creating assignment", err)
	}
	query = `
		INSERT INTO submit_assignments (
			submission_id, assignment_id, user_id, status, 
			file, image, comments, late_submission, created_at
		) VALUES (
			:submission_id, :assignment_id, :user_id, :status, 
			:file, :image, :comments, :late_submission, :created_at
		)
	`

	// sqlx expands slices automatically for NamedExec
	if _, err := tx.NamedExec(query, submissions); err != nil {
		tx.Rollback()
		return apperrors.Wrap(apperrors.ErrDatabase, "Error batch creating submissions", err)
	}
	if err := tx.Commit(); err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Error creating assignment", err)
	}
	return nil
}

func (r *AssignmentRepository) CreateSubmission(submission *models.SubmitAssignment) error {
	query := `INSERT INTO submit_assignments
			 (submission_id, 
			 assignment_id, 
			 user_id, 
			 status, 
			 file, 
			 image, 
			 comments, 
			 late_submission, 
			 created_at)
			 VALUES (:submission_id, 
			 		:assignment_id, 
					:user_id, 
					:status, 
					:file, 
					:image, 
					:comments, 
					:late_submission, :created_at)`
	params := map[string]interface{}{
		"submission_id":   submission.SubmissionID,
		"assignment_id":   submission.AssignmentID,
		"user_id":         submission.UserID,
		"status":          submission.Status,
		"file":            submission.File,
		"image":           submission.Image,
		"comments":        submission.Comments,
		"late_submission": submission.LateSubmission,
		"created_at":      submission.CreatedAt,
	}
	if _, err := r.db.NamedExec(query, params); err != nil {
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
	args := make(map[string]interface{}, len(userIDs))
	for i, userID := range userIDs {
		name := "user_id_" + strconv.Itoa(i)
		placeholders[i] = ":" + name
		args[name] = userID
	}
	query := `SELECT submission_id,
					 assignment_id, 
					 user_id, 
					 status, 
					 file, 
					 image, 
					 comments, 
					 late_submission, 
					 created_at 
					 FROM submit_assignments 
					 WHERE user_id IN (ANY(:user_ids))`
	params := map[string]interface{}{
		"user_ids": userIDs,
	}

	rows, err := r.db.NamedQuery(query, params)
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
	args := map[string]interface{}{"status": status}
	for i, userID := range userIDs {
		name := "user_id_" + strconv.Itoa(i)
		placeholders[i] = ":" + name
		args[name] = userID
	}
	query := `UPDATE submit_assignments 
				SET status = :status 
				WHERE user_id = ANY(:user_ids)`

	params := map[string]interface{}{
		"status":   "approved",
		"user_ids": userIDs,
	}
	if _, err := r.db.NamedExec(query, params); err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Error updating assignment status", err)
	}
	return nil
}

func (r *AssignmentRepository) FindSubmissionsByUser(userID uuid.UUID) ([]models.SubmitAssignment, error) {
	var submissions []models.SubmitAssignment
	query := `	SELECT 
				submission_id,
				assignment_id, 
				user_id, status, 
				file, 
				image, 
				comments, 
				late_submission, 
				created_at 
				FROM submit_assignments 
				WHERE user_id = :user_id`

	params := map[string]interface{}{"user_id": userID}

	rows, err := r.db.NamedQuery(query, params)
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
	query := `SELECT submission_id, 
				assignment_id, 
				user_id, 
				status, 
				file, 
				image, 
				comments, 
				late_submission, 
				created_at 
				FROM submit_assignments 
				WHERE status = :status`
	params := map[string]interface{}{"status": "pending"}
	rows, err := r.db.NamedQuery(query, params)
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
