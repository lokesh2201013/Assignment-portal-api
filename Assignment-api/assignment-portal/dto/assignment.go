package dto

import "github.com/google/uuid"

type AssignmentCreateRequest struct {
	Email              string          `json:"email"`
	Task               string          `json:"task"`
	DueDate            string          `json:"due_date"`
	Branch             string          `json:"branch"`
	Semester           int             `json:"semester"`
	SubjectCode        string          `json:"subject_code"`
	CourseID           *uuid.UUID      `json:"course_id,omitempty"`
	MaxScore           float64         `json:"max_score"`
	LatePolicy         string          `json:"late_policy"`
	LatePenaltyPercent float64         `json:"late_penalty_percent"`
	AllowResubmission  bool            `json:"allow_resubmission"`
	MaxAttempts        int             `json:"max_attempts"`
	ReminderHours      int             `json:"reminder_hours"`
	Rubrics            []RubricRequest `json:"rubrics"`
}

type RubricRequest struct {
	Criterion   string  `json:"criterion"`
	Description string  `json:"description"`
	MaxPoints   float64 `json:"max_points"`
}

type GradeSubmissionRequest struct {
	AssignmentID uuid.UUID          `json:"assignment_id"`
	UserID       uuid.UUID          `json:"user_id"`
	Status       string             `json:"status"`
	Score        *float64           `json:"score"`
	Feedback     string             `json:"feedback"`
	RubricScores []RubricScoreInput `json:"rubric_scores"`
}

type RubricScoreInput struct {
	RubricID uuid.UUID `json:"rubric_id"`
	Points   float64   `json:"points"`
}

type AssignmentFilters struct {
	Branch      string
	Semester    string
	SubjectCode string
	Page        int
	Limit       int
}

type PageMetadata struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type PaginatedResponse[T any] struct {
	Data []T          `json:"data"`
	Page PageMetadata `json:"page"`
}

type AssignmentSubmissionUpload struct {
	TenantID     string
	AssignmentID uuid.UUID
	UserID       uuid.UUID
	DueDate      string
	Comments     string
	FilePath     string
	ImagePath    string
}

type AssignmentStatusRequest struct {
	StudentIDs []uuid.UUID
	Reason     string
	Status     string
}

type AssignToStudentsResponse struct {
	Message    string      `json:"message"`
	Assignment interface{} `json:"assignment"`
	StudentCnt int         `json:"student_cnt"`
	MailStatus string      `json:"mail_status"`
}

type CourseCreateRequest struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Branch   string `json:"branch"`
	Semester int    `json:"semester"`
}

type BulkInvitationRequest struct {
	Invitations []InvitationEntry `json:"invitations"`
}

type InvitationEntry struct {
	Email    string     `json:"email"`
	Role     string     `json:"role"`
	Name     string     `json:"name"`
	Branch   string     `json:"branch"`
	Semester int        `json:"semester"`
	CourseID *uuid.UUID `json:"course_id,omitempty"`
}

type RosterImportResult struct {
	Invited int      `json:"invited"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}

type MembershipStatusRequest struct {
	Status string `json:"status"`
}
