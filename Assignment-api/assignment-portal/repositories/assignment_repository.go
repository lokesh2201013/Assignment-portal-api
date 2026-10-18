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

const assignmentColumns = `assignment_id, tenant_id, email, admin_id, task, created_at, updated_at, due_date, branch, semester, subject_code, course_id, max_score, late_policy, late_penalty_percent, allow_resubmission, max_attempts, reminder_hours`

const submissionColumns = `submission_id, tenant_id, assignment_id, user_id, status, file, image, comments, COALESCE(late_submission, false) AS late_submission, created_at, score, COALESCE(feedback, '') AS feedback, COALESCE(attempt_number, 1) AS attempt_number, COALESCE(graded_at, '') AS graded_at, graded_by`

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
	query := `SELECT ` + assignmentColumns + ` FROM assignments`
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
	where := " WHERE " + strings.Join(conditions, " AND ")
	query += where
	countQuery += where
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
	assignments, err := scanAssignments(rows)
	if err != nil {
		return nil, 0, err
	}
	return assignments, total, nil
}

func (r *AssignmentRepository) FindAssignmentInTenant(tenantID string, assignmentID uuid.UUID) (models.Assignment, error) {
	var assignment models.Assignment
	if err := r.db.Get(&assignment, `SELECT `+assignmentColumns+` FROM assignments WHERE tenant_id = $1 AND assignment_id = $2`, tenantID, assignmentID); err != nil {
		return models.Assignment{}, apperrors.Wrap(apperrors.ErrNotFound, "Assignment not found", err)
	}
	rubrics, err := r.FindRubricsInTenant(tenantID, assignmentID)
	if err != nil {
		return models.Assignment{}, err
	}
	assignment.Rubrics = rubrics
	return assignment, nil
}

func (r *AssignmentRepository) FindRubricsInTenant(tenantID string, assignmentID uuid.UUID) ([]models.Rubric, error) {
	var rubrics []models.Rubric
	if err := r.db.Select(&rubrics, `SELECT rubric_id, tenant_id, assignment_id, criterion, description, max_points, sort_order FROM assignment_rubrics WHERE tenant_id = $1 AND assignment_id = $2 ORDER BY sort_order, criterion`, tenantID, assignmentID); err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching rubrics", err)
	}
	return rubrics, nil
}

func (r *AssignmentRepository) CreateWithSubmissions(assignment *models.Assignment, submissions []models.SubmitAssignment) error {
	return r.CreateWithSubmissionsInTenant(assignment.TenantID, assignment, submissions)
}

func (r *AssignmentRepository) CreateWithSubmissionsInTenant(tenantID string, assignment *models.Assignment, submissions []models.SubmitAssignment) error {
	assignment.TenantID = tenantID
	tx, err := r.db.Beginx()
	if err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Error creating assignment", err)
	}
	query := `
		INSERT INTO assignments (
			assignment_id, tenant_id, email, admin_id, task, created_at,
			updated_at, due_date, branch, semester, subject_code, course_id,
			max_score, late_policy, late_penalty_percent, allow_resubmission, max_attempts, reminder_hours
		) VALUES (
			:assignment_id, :tenant_id, :email, :admin_id, :task, :created_at,
			:updated_at, :due_date, :branch, :semester, :subject_code, :course_id,
			:max_score, :late_policy, :late_penalty_percent, :allow_resubmission, :max_attempts, :reminder_hours
		)
	`
	if _, err := tx.NamedExec(query, assignment); err != nil {
		tx.Rollback()
		return apperrors.Wrap(apperrors.ErrDatabase, "Error creating assignment", err)
	}
	for i, rubric := range assignment.Rubrics {
		rubric.RubricID = uuid.New()
		rubric.TenantID = tenantID
		rubric.AssignmentID = assignment.AssignmentID
		rubric.SortOrder = i
		if _, err := tx.NamedExec(`INSERT INTO assignment_rubrics (rubric_id, tenant_id, assignment_id, criterion, description, max_points, sort_order) VALUES (:rubric_id, :tenant_id, :assignment_id, :criterion, :description, :max_points, :sort_order)`, rubric); err != nil {
			tx.Rollback()
			return apperrors.Wrap(apperrors.ErrDatabase, "Error creating rubrics", err)
		}
	}
	query = `
		INSERT INTO submit_assignments (
			submission_id, tenant_id, assignment_id, user_id, status,
			file, image, comments, late_submission, created_at, feedback, attempt_number
		) VALUES (
			:submission_id, :tenant_id, :assignment_id, :user_id, :status,
			:file, :image, :comments, :late_submission, :created_at, :feedback, :attempt_number
		)
	`
	if len(submissions) > 0 {
		if _, err := tx.NamedExec(query, submissions); err != nil {
			tx.Rollback()
			return apperrors.Wrap(apperrors.ErrDatabase, "Error batch creating submissions", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Error creating assignment", err)
	}
	return nil
}

func (r *AssignmentRepository) CreateSubmission(submission *models.SubmitAssignment) error {
	query := `INSERT INTO submit_assignments
			 (submission_id, tenant_id, assignment_id, user_id, status, file, image, comments, late_submission, created_at, feedback, attempt_number)
			 VALUES (:submission_id, :tenant_id, :assignment_id, :user_id, :status, :file, :image, :comments, :late_submission, :created_at, :feedback, :attempt_number)`
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
		"feedback":        submission.Feedback,
		"attempt_number":  submission.AttemptNumber,
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
	if len(userIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(userIDs))
	args := make(map[string]interface{}, len(userIDs)+2)
	for i, userID := range userIDs {
		name := "user_id_" + strconv.Itoa(i)
		placeholders[i] = ":" + name
		args[name] = userID
	}
	query := `SELECT ` + submissionColumns + ` FROM submit_assignments WHERE tenant_id = :tenant_id AND status = 'pending' AND file IS NOT NULL AND file <> '' AND user_id IN (` + strings.Join(placeholders, ",") + `)`
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
	return scanSubmissions(rows)
}

func (r *AssignmentRepository) FindSubmissionForUserInTenant(tenantID string, assignmentID uuid.UUID, userID uuid.UUID) (models.SubmitAssignment, error) {
	var submission models.SubmitAssignment
	if err := r.db.Get(&submission, `SELECT `+submissionColumns+` FROM submit_assignments WHERE tenant_id = $1 AND assignment_id = $2 AND user_id = $3`, tenantID, assignmentID, userID); err != nil {
		return models.SubmitAssignment{}, apperrors.Wrap(apperrors.ErrNotFound, "Assignment not found for this user", err)
	}
	return submission, nil
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

func (r *AssignmentRepository) GradeSubmissionInTenant(tenantID string, submission models.SubmitAssignment, scores []models.RubricScore) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Error grading submission", err)
	}
	result, err := tx.NamedExec(`UPDATE submit_assignments SET status = :status, score = :score, feedback = :feedback, graded_at = :graded_at, graded_by = :graded_by WHERE tenant_id = :tenant_id AND assignment_id = :assignment_id AND user_id = :user_id AND file IS NOT NULL AND file <> ''`, map[string]interface{}{
		"status":        submission.Status,
		"score":         submission.Score,
		"feedback":      submission.Feedback,
		"graded_at":     submission.GradedAt,
		"graded_by":     submission.GradedBy,
		"tenant_id":     tenantID,
		"assignment_id": submission.AssignmentID,
		"user_id":       submission.UserID,
	})
	if err != nil {
		tx.Rollback()
		return apperrors.Wrap(apperrors.ErrDatabase, "Error grading submission", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		tx.Rollback()
		return apperrors.New(apperrors.ErrNotFound, "No submitted assignment found for this user")
	}
	if _, err := tx.Exec(`DELETE FROM rubric_scores WHERE tenant_id = $1 AND submission_id = $2`, tenantID, submission.SubmissionID); err != nil {
		tx.Rollback()
		return apperrors.Wrap(apperrors.ErrDatabase, "Error grading submission", err)
	}
	for _, score := range scores {
		if _, err := tx.Exec(`INSERT INTO rubric_scores (tenant_id, submission_id, rubric_id, points) VALUES ($1, $2, $3, $4)`, tenantID, submission.SubmissionID, score.RubricID, score.Points); err != nil {
			tx.Rollback()
			return apperrors.Wrap(apperrors.ErrDatabase, "Error saving rubric scores", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return apperrors.Wrap(apperrors.ErrDatabase, "Error creating assignment", err)
	}
	return nil
}

func (r *AssignmentRepository) FindSubmissionsByUser(userID uuid.UUID) ([]models.SubmitAssignment, error) {
	return r.FindSubmissionsByUserInTenant("default", userID)
}

func (r *AssignmentRepository) FindSubmissionsByUserInTenant(tenantID string, userID uuid.UUID) ([]models.SubmitAssignment, error) {
	var submissions []models.SubmitAssignment
	if err := r.db.Select(&submissions, `SELECT `+submissionColumns+` FROM submit_assignments WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID); err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching user assignments", err)
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
	if err := r.db.Select(&submissions, `SELECT `+submissionColumns+` FROM submit_assignments WHERE tenant_id = $1 AND user_id = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`, tenantID, userID, limit, (page-1)*limit); err != nil {
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
	var submissions []models.SubmitAssignment
	if err := r.db.Select(&submissions, `SELECT `+submissionColumns+` FROM submit_assignments `+where+` ORDER BY created_at DESC LIMIT $`+strconv.Itoa(limitPosition)+` OFFSET $`+strconv.Itoa(offsetPosition), args...); err != nil {
		return nil, 0, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching submissions", err)
	}
	return submissions, total, nil
}

func (r *AssignmentRepository) SubmitAssignedAssignment(tenantID string, submission models.SubmitAssignment) (models.SubmitAssignment, error) {
	return r.SaveStudentAttemptInTenant(tenantID, submission, false)
}

func (r *AssignmentRepository) SaveStudentAttemptInTenant(tenantID string, submission models.SubmitAssignment, archivePrevious bool) (models.SubmitAssignment, error) {
	tx, err := r.db.Beginx()
	if err != nil {
		return models.SubmitAssignment{}, apperrors.Wrap(apperrors.ErrDatabase, "Error saving submission", err)
	}
	if archivePrevious {
		if _, err := tx.NamedExec(`INSERT INTO submission_revisions (
				revision_id, tenant_id, submission_id, assignment_id, user_id, attempt_number, status, file, image, comments, feedback, score, late_submission, created_at, replaced_at
			) SELECT :revision_id, s.tenant_id, s.submission_id, s.assignment_id, s.user_id, s.attempt_number, s.status, s.file, s.image, s.comments, s.feedback, s.score, s.late_submission, s.created_at, :replaced_at
			FROM submit_assignments s WHERE s.tenant_id = :tenant_id AND s.assignment_id = :assignment_id AND s.user_id = :user_id`, map[string]interface{}{
			"revision_id":   uuid.New(),
			"tenant_id":     tenantID,
			"assignment_id": submission.AssignmentID,
			"user_id":       submission.UserID,
			"replaced_at":   submission.CreatedAt,
		}); err != nil {
			tx.Rollback()
			return models.SubmitAssignment{}, apperrors.Wrap(apperrors.ErrDatabase, "Error saving revision history", err)
		}
	}
	query := `UPDATE submit_assignments AS s
		SET file = :file, image = :image, comments = :comments,
			late_submission = :late_submission,
			created_at = :created_at,
			status = 'pending',
			score = NULL,
			feedback = '',
			graded_at = '',
			graded_by = NULL,
			attempt_number = :attempt_number
		WHERE s.tenant_id = :tenant_id
			AND s.assignment_id = :assignment_id
			AND s.user_id = :user_id
		RETURNING ` + submissionColumns
	rows, err := tx.NamedQuery(query, map[string]interface{}{
		"tenant_id":       tenantID,
		"assignment_id":   submission.AssignmentID,
		"user_id":         submission.UserID,
		"file":            submission.File,
		"image":           submission.Image,
		"comments":        submission.Comments,
		"created_at":      submission.CreatedAt,
		"late_submission": submission.LateSubmission,
		"attempt_number":  submission.AttemptNumber,
	})
	if err != nil {
		tx.Rollback()
		return models.SubmitAssignment{}, apperrors.Wrap(apperrors.ErrDatabase, "Error saving submission", err)
	}
	if !rows.Next() {
		rows.Close()
		tx.Rollback()
		if err := rows.Err(); err != nil {
			return models.SubmitAssignment{}, apperrors.Wrap(apperrors.ErrDatabase, "Error saving submission", err)
		}
		return models.SubmitAssignment{}, apperrors.New(apperrors.ErrNotFound, "Assignment not found for this user")
	}
	var saved models.SubmitAssignment
	if err := rows.StructScan(&saved); err != nil {
		rows.Close()
		tx.Rollback()
		return models.SubmitAssignment{}, apperrors.Wrap(apperrors.ErrDatabase, "Error saving submission", err)
	}
	rows.Close()
	if err := tx.Commit(); err != nil {
		return models.SubmitAssignment{}, apperrors.Wrap(apperrors.ErrDatabase, "Error saving submission", err)
	}
	return saved, nil
}

func (r *AssignmentRepository) FindRevisionsInTenant(tenantID string, submissionID uuid.UUID, userID uuid.UUID) ([]models.SubmissionRevision, error) {
	var revisions []models.SubmissionRevision
	if err := r.db.Select(&revisions, `SELECT revision_id, tenant_id, submission_id, assignment_id, user_id, attempt_number, status, file, image, comments, COALESCE(feedback, '') AS feedback, score, COALESCE(late_submission, false) AS late_submission, created_at, replaced_at FROM submission_revisions WHERE tenant_id = $1 AND submission_id = $2 AND user_id = $3 ORDER BY attempt_number`, tenantID, submissionID, userID); err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching revision history", err)
	}
	return revisions, nil
}

func (r *AssignmentRepository) FindDashboardItemsInTenant(tenantID string, userID uuid.UUID) ([]models.DashboardItem, error) {
	var items []models.DashboardItem
	query := `
		SELECT s.submission_id, s.assignment_id, a.task, a.subject_code, a.due_date, s.status,
			(s.file IS NOT NULL AND s.file <> '') AS submitted,
			COALESCE(s.late_submission, false) AS late,
			s.score, a.max_score, COALESCE(s.feedback, '') AS feedback,
			EXTRACT(EPOCH FROM (a.due_date::timestamptz - NOW())) / 3600.0 AS hours_until_due,
			a.reminder_hours, COALESCE(s.attempt_number, 1) AS attempt_number, a.allow_resubmission
		FROM submit_assignments s
		JOIN assignments a ON a.tenant_id = s.tenant_id AND a.assignment_id = s.assignment_id
		WHERE s.tenant_id = $1 AND s.user_id = $2
		ORDER BY a.due_date::timestamptz ASC`
	if err := r.db.Select(&items, query, tenantID, userID); err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching learner dashboard", err)
	}
	return items, nil
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

func scanAssignments(rows *sqlx.Rows) ([]models.Assignment, error) {
	var assignments []models.Assignment
	for rows.Next() {
		var assignment models.Assignment
		if err := rows.StructScan(&assignment); err != nil {
			return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching assignments", err)
		}
		assignments = append(assignments, assignment)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching assignments", err)
	}
	return assignments, nil
}

func scanSubmissions(rows *sqlx.Rows) ([]models.SubmitAssignment, error) {
	var submissions []models.SubmitAssignment
	for rows.Next() {
		var submission models.SubmitAssignment
		if err := rows.StructScan(&submission); err != nil {
			return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching submissions", err)
		}
		submissions = append(submissions, submission)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.ErrDatabase, "Error fetching submissions", err)
	}
	return submissions, nil
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
