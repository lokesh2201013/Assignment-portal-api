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
	return r.ListInTenant("default", filters)
}

func (r *AssignmentRepository) ListInTenant(tenantID string, filters dto.AssignmentFilters) ([]models.Assignment, error) {
	assignments, _, err := r.ListPageInTenant(tenantID, filters)
	return assignments, err
}

func (r *AssignmentRepository) ListPageInTenant(tenantID string, filters dto.AssignmentFilters) ([]models.Assignment, int, error) {
	var assignments []models.Assignment
	query := `SELECT assignment_id, tenant_id, email, admin_id, task, created_at, updated_at, due_date, branch, semester, subject_code FROM assignments`
	countQuery := `SELECT COUNT(*) FROM assignments`
	conditions := []string{"tenant_id = :tenant_id"}
	args := map[string]interface{}{"tenant_id": tenantID}
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
		where := " WHERE " + strings.Join(conditions, " AND ")
		query += where
		countQuery += where
	}
	var total int
	countRows, err := r.db.NamedQuery(countQuery, args)
	if err != nil {
		return nil, 0, apperrors.Wrap(apperrors.ErrDatabase, "Error counting assignments", err)
	}
	if countRows.Next() {
		if err := countRows.Scan(&total); err != nil {
			countRows.Close()
			return nil, 0, apperrors.Wrap(apperrors.ErrDatabase, "Error counting assignments", err)
		}
	}
	if err := countRows.Err(); err != nil {
		countRows.Close()
		return nil, 0, apperrors.Wrap(apperrors.ErrDatabase, "Error counting assignments", err)
	}
	countRows.Close()
	page, limit := normalizePage(filters.Page, filters.Limit)
	query += " ORDER BY created_at DESC LIMIT :limit OFFSET :offset"
	args["limit"] = limit
	args["offset"] = (page - 1) * limit
	rows, err := r.db.NamedQuery(query, args)
	if err != nil {
		return nil, 0, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching assignments", err)
	}
	defer rows.Close()
	for rows.Next() {
		var assignment models.Assignment
		if err := rows.Scan(&assignment.AssignmentID, &assignment.TenantID, &assignment.Email, &assignment.AdminID, &assignment.Task, &assignment.CreatedAt, &assignment.UpdatedAt, &assignment.DueDate, &assignment.Branch, &assignment.Semester, &assignment.SubjectCode); err != nil {
			return nil, 0, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching assignments", err)
		}
		assignments = append(assignments, assignment)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching assignments", err)
	}
	return assignments, total, nil
}

func normalizePage(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}

func (r *AssignmentRepository) CreateWithSubmissions(assignment *models.Assignment, submissions []models.SubmitAssignment) error {
	assignment.TenantID = "default"
	for i := range submissions {
		submissions[i].TenantID = "default"
	}
	return r.CreateWithSubmissionsInTenant("default", assignment, submissions)
}

func (r *AssignmentRepository) CreateWithSubmissionsInTenant(tenantID string, assignment *models.Assignment, submissions []models.SubmitAssignment) error {
	assignment.TenantID = tenantID
	for i := range submissions {
		submissions[i].TenantID = tenantID
	}
	tx, err := r.db.Beginx()
	if err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Error creating assignment", err)
	}
	query := `
		INSERT INTO assignments (
			assignment_id, tenant_id, email, admin_id, task, created_at,
			updated_at, due_date, branch, semester, subject_code
		) VALUES (
			:assignment_id, :tenant_id, :email, :admin_id, :task, :created_at,
			:updated_at, :due_date, :branch, :semester, :subject_code
		)
	`

	if _, err := tx.NamedExec(query, assignment); err != nil {
		tx.Rollback()
		return apperrors.Wrap(apperrors.ErrDatabase, "Error creating assignment", err)
	}
	query = `
		INSERT INTO submit_assignments (
			submission_id, tenant_id, assignment_id, user_id, status,
			file, image, comments, late_submission, created_at
		) VALUES (
			:submission_id, :tenant_id, :assignment_id, :user_id, :status,
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
	submission.TenantID = "default"
	return r.CreateSubmissionInTenant("default", submission)
}

func (r *AssignmentRepository) CreateSubmissionInTenant(tenantID string, submission *models.SubmitAssignment) error {
	query := `INSERT INTO submit_assignments
			 (submission_id, 
			 tenant_id,
			 assignment_id, 
			 user_id, 
			 status, 
			 file, 
			 image, 
			 comments, 
			 late_submission, 
			 created_at)
			 VALUES (:submission_id,
				:tenant_id,
			 		:assignment_id, 
					:user_id, 
					:status, 
					:file, 
					:image, 
					:comments, 
					:late_submission, :created_at)`
	params := map[string]interface{}{
		"submission_id":   submission.SubmissionID,
		"tenant_id":       tenantID,
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
	return r.FindSubmissionsByUsersInTenant("default", uuid.Nil, userIDs)
}

func (r *AssignmentRepository) FindSubmissionsByUsersInTenant(tenantID string, assignmentID uuid.UUID, userIDs []uuid.UUID) ([]models.SubmitAssignment, error) {
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
	query := `SELECT submission_id, tenant_id,
					 assignment_id, 
					 user_id, 
					 status, 
					 file, 
					 image, 
					 comments, 
					 late_submission, 
					 created_at 
					 FROM submit_assignments 
					 WHERE tenant_id = :tenant_id AND status = 'pending'
					 AND file IS NOT NULL AND file <> '' AND user_id IN (` + strings.Join(placeholders, ",") + `)`
	args["tenant_id"] = tenantID
	if assignmentID != uuid.Nil {
		query += " AND assignment_id = :assignment_id"
		args["assignment_id"] = assignmentID
	}

	rows, err := r.db.NamedQuery(query, args)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error getting submitted assignments", err)
	}
	defer rows.Close()
	for rows.Next() {
		var submission models.SubmitAssignment
		if err := rows.Scan(&submission.SubmissionID, &submission.TenantID, &submission.AssignmentID, &submission.UserID, &submission.Status, &submission.File, &submission.Image, &submission.Comments, &submission.LateSubmission, &submission.CreatedAt); err != nil {
			return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error getting submitted assignments", err)
		}
		submissions = append(submissions, submission)
	}
	return submissions, nil
}

func (r *AssignmentRepository) UpdateSubmissionStatus(userIDs []uuid.UUID, status string) error {
	return r.UpdateSubmissionStatusInTenant("default", uuid.Nil, userIDs, status)
}

func (r *AssignmentRepository) UpdateSubmissionStatusInTenant(tenantID string, assignmentID uuid.UUID, userIDs []uuid.UUID, status string) error {
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
	args["tenant_id"] = tenantID
	query := `UPDATE submit_assignments SET status = :status WHERE tenant_id = :tenant_id
		AND status = 'pending' AND file IS NOT NULL AND file <> '' AND user_id IN (` + strings.Join(placeholders, ",") + `)`
	if assignmentID != uuid.Nil {
		query += " AND assignment_id = :assignment_id"
		args["assignment_id"] = assignmentID
	}
	if _, err := r.db.NamedExec(query, args); err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Error updating assignment status", err)
	}
	return nil
}

func (r *AssignmentRepository) FindSubmissionsByUser(userID uuid.UUID) ([]models.SubmitAssignment, error) {
	return r.FindSubmissionsByUserInTenant("default", userID)
}

func (r *AssignmentRepository) FindSubmissionsByUserInTenant(tenantID string, userID uuid.UUID) ([]models.SubmitAssignment, error) {
	var submissions []models.SubmitAssignment
	query := `	SELECT 
				submission_id,
				tenant_id,
				assignment_id, 
				user_id, status, 
				file, 
				image, 
				comments, 
				late_submission, 
				created_at 
				FROM submit_assignments 
				WHERE tenant_id = :tenant_id AND user_id = :user_id`

	params := map[string]interface{}{"tenant_id": tenantID, "user_id": userID}

	rows, err := r.db.NamedQuery(query, params)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching user assignments", err)
	}
	defer rows.Close()
	for rows.Next() {
		var submission models.SubmitAssignment
		if err := rows.Scan(&submission.SubmissionID, &submission.TenantID, &submission.AssignmentID, &submission.UserID, &submission.Status, &submission.File, &submission.Image, &submission.Comments, &submission.LateSubmission, &submission.CreatedAt); err != nil {
			return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching user assignments", err)
		}
		submissions = append(submissions, submission)
	}
	return submissions, nil
}

func (r *AssignmentRepository) FindSubmissionsByUserPageInTenant(tenantID string, userID uuid.UUID, page int, limit int) ([]models.SubmitAssignment, int, error) {
	page, limit = normalizePage(page, limit)
	var total int
	if err := r.db.Get(&total, `SELECT COUNT(*) FROM submit_assignments WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID); err != nil {
		return nil, 0, apperrors.Wrap(apperrors.ErrDatabase, "Error counting user submissions", err)
	}
	var submissions []models.SubmitAssignment
	rows, err := r.db.Queryx(`SELECT submission_id, tenant_id, assignment_id, user_id, status, file, image, comments, late_submission, created_at FROM submit_assignments WHERE tenant_id = $1 AND user_id = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`, tenantID, userID, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching user assignments", err)
	}
	defer rows.Close()
	for rows.Next() {
		var submission models.SubmitAssignment
		if err := rows.Scan(&submission.SubmissionID, &submission.TenantID, &submission.AssignmentID, &submission.UserID, &submission.Status, &submission.File, &submission.Image, &submission.Comments, &submission.LateSubmission, &submission.CreatedAt); err != nil {
			return nil, 0, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching user assignments", err)
		}
		submissions = append(submissions, submission)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching user assignments", err)
	}
	return submissions, total, nil
}

func (r *AssignmentRepository) FindPendingSubmissions() ([]models.SubmitAssignment, error) {
	return r.FindPendingSubmissionsInTenant("default", uuid.Nil)
}

func (r *AssignmentRepository) FindPendingSubmissionsInTenant(tenantID string, assignmentID uuid.UUID) ([]models.SubmitAssignment, error) {
	submissions, _, err := r.FindPendingSubmissionsPageInTenant(tenantID, assignmentID, 1, 100)
	return submissions, err
}

func (r *AssignmentRepository) FindPendingSubmissionsPageInTenant(tenantID string, assignmentID uuid.UUID, page int, limit int) ([]models.SubmitAssignment, int, error) {
	var submissions []models.SubmitAssignment
	page, limit = normalizePage(page, limit)
	where := `WHERE tenant_id = $1 AND status = 'pending' AND file IS NOT NULL AND file <> ''`
	args := []interface{}{tenantID}
	if assignmentID != uuid.Nil {
		where += " AND assignment_id = $2"
		args = append(args, assignmentID)
	}
	var total int
	if err := r.db.Get(&total, `SELECT COUNT(*) FROM submit_assignments `+where, args...); err != nil {
		return nil, 0, apperrors.Wrap(apperrors.ErrDatabase, "Error counting submissions", err)
	}
	limitPosition := len(args) + 1
	offsetPosition := len(args) + 2
	args = append(args, limit, (page-1)*limit)
	query := `SELECT submission_id,
				tenant_id,
				assignment_id, 
				user_id, 
				status, 
				file, 
				image, 
				comments, 
				late_submission, 
				created_at 
				FROM submit_assignments ` + where + ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(limitPosition) + ` OFFSET $` + strconv.Itoa(offsetPosition)
	rows, err := r.db.Queryx(query, args...)
	if err != nil {
		return nil, 0, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching submissions", err)
	}
	defer rows.Close()
	for rows.Next() {
		var submission models.SubmitAssignment
		if err := rows.Scan(&submission.SubmissionID, &submission.TenantID, &submission.AssignmentID, &submission.UserID, &submission.Status, &submission.File, &submission.Image, &submission.Comments, &submission.LateSubmission, &submission.CreatedAt); err != nil {
			return nil, 0, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching submissions", err)
		}
		submissions = append(submissions, submission)
	}
	return submissions, total, nil
}

func (r *AssignmentRepository) SubmitAssignedAssignment(tenantID string, submission models.SubmitAssignment) (models.SubmitAssignment, error) {
	query := `UPDATE submit_assignments AS s
		SET file = :file, image = :image, comments = :comments,
			late_submission = CURRENT_TIMESTAMP > a.due_date::timestamptz,
			created_at = :created_at
		FROM assignments AS a
		WHERE s.tenant_id = :tenant_id
			AND s.assignment_id = :assignment_id
			AND s.user_id = :user_id
			AND s.status = 'pending'
			AND a.tenant_id = s.tenant_id
			AND a.assignment_id = s.assignment_id
		RETURNING s.submission_id, s.tenant_id, s.assignment_id, s.user_id, s.status,
			s.file, s.image, s.comments, s.late_submission, s.created_at`
	rows, err := r.db.NamedQuery(query, map[string]interface{}{
		"tenant_id": tenantID, "assignment_id": submission.AssignmentID,
		"user_id": submission.UserID, "file": submission.File,
		"image": submission.Image, "comments": submission.Comments,
		"created_at": submission.CreatedAt,
	})
	if err != nil {
		return models.SubmitAssignment{}, apperrors.Wrap(apperrors.ErrDatabase, "Error saving submission", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return models.SubmitAssignment{}, apperrors.Wrap(apperrors.ErrDatabase, "Error saving submission", err)
		}
		return models.SubmitAssignment{}, apperrors.New(apperrors.ErrNotFound, "Assignment not found for this user")
	}
	var saved models.SubmitAssignment
	if err := rows.Scan(&saved.SubmissionID, &saved.TenantID, &saved.AssignmentID, &saved.UserID, &saved.Status, &saved.File, &saved.Image, &saved.Comments, &saved.LateSubmission, &saved.CreatedAt); err != nil {
		return models.SubmitAssignment{}, apperrors.Wrap(apperrors.ErrDatabase, "Error saving submission", err)
	}
	return saved, nil
}

func (r *AssignmentRepository) FindSubmissionFiles(tenantID string, submissionID uuid.UUID, userID uuid.UUID) (string, string, error) {
	var result struct {
		File  string `db:"file"`
		Image string `db:"image"`
	}
	if err := r.db.Get(&result, `SELECT file, image FROM submit_assignments WHERE tenant_id = $1 AND submission_id = $2 AND user_id = $3`, tenantID, submissionID, userID); err != nil {
		return "", "", apperrors.Wrap(apperrors.ErrNotFound, "Submission not found", err)
	}
	return result.File, result.Image, nil
}
