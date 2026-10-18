package models

import "github.com/google/uuid"

const (
	LatePolicyAllow   = "allow"
	LatePolicyReject  = "reject"
	LatePolicyPenalty = "penalty"
)

// Admin-created assignment
type Assignment struct {
	AssignmentID       uuid.UUID  `json:"id" db:"assignment_id"`
	TenantID           string     `json:"tenant_id" db:"tenant_id"`
	Email              string     `json:"email" db:"email"`
	AdminID            uuid.UUID  `json:"admin_id" db:"admin_id"`
	Task               string     `json:"task" db:"task"`
	CreatedAt          string     `json:"created_at" db:"created_at"`
	UpdatedAt          string     `json:"updated_at" db:"updated_at"`
	DueDate            string     `json:"due_date" db:"due_date"`
	Branch             string     `json:"branch" db:"branch"`
	Semester           int        `json:"semester" db:"semester"`
	SubjectCode        string     `json:"subject_code" db:"subject_code"`
	CourseID           *uuid.UUID `json:"course_id,omitempty" db:"course_id"`
	MaxScore           float64    `json:"max_score" db:"max_score"`
	LatePolicy         string     `json:"late_policy" db:"late_policy"`
	LatePenaltyPercent float64    `json:"late_penalty_percent" db:"late_penalty_percent"`
	AllowResubmission  bool       `json:"allow_resubmission" db:"allow_resubmission"`
	MaxAttempts        int        `json:"max_attempts" db:"max_attempts"`
	ReminderHours      int        `json:"reminder_hours" db:"reminder_hours"`
	Rubrics            []Rubric   `json:"rubrics,omitempty" db:"-"`
}

type Rubric struct {
	RubricID     uuid.UUID `json:"rubric_id" db:"rubric_id"`
	TenantID     string    `json:"tenant_id" db:"tenant_id"`
	AssignmentID uuid.UUID `json:"assignment_id" db:"assignment_id"`
	Criterion    string    `json:"criterion" db:"criterion"`
	Description  string    `json:"description" db:"description"`
	MaxPoints    float64   `json:"max_points" db:"max_points"`
	SortOrder    int       `json:"sort_order" db:"sort_order"`
}

type SubmitAssignment struct {
	SubmissionID   uuid.UUID  `json:"submission_id" db:"submission_id"`
	TenantID       string     `json:"tenant_id" db:"tenant_id"`
	AssignmentID   uuid.UUID  `json:"assignment_id" db:"assignment_id"`
	UserID         uuid.UUID  `json:"user_id" db:"user_id"`
	Status         string     `json:"status" db:"status"`
	File           string     `json:"file" db:"file"`
	Image          string     `json:"image" db:"image"`
	Comments       string     `json:"comments" db:"comments"`
	LateSubmission bool       `json:"late" db:"late_submission"`
	CreatedAt      string     `json:"created_at" db:"created_at"`
	Score          *float64   `json:"score,omitempty" db:"score"`
	Feedback       string     `json:"feedback,omitempty" db:"feedback"`
	AttemptNumber  int        `json:"attempt_number" db:"attempt_number"`
	GradedAt       string     `json:"graded_at,omitempty" db:"graded_at"`
	GradedBy       *uuid.UUID `json:"graded_by,omitempty" db:"graded_by"`
}

type SubmissionRevision struct {
	RevisionID     uuid.UUID `json:"revision_id" db:"revision_id"`
	TenantID       string    `json:"tenant_id" db:"tenant_id"`
	SubmissionID   uuid.UUID `json:"submission_id" db:"submission_id"`
	AssignmentID   uuid.UUID `json:"assignment_id" db:"assignment_id"`
	UserID         uuid.UUID `json:"user_id" db:"user_id"`
	AttemptNumber  int       `json:"attempt_number" db:"attempt_number"`
	Status         string    `json:"status" db:"status"`
	File           string    `json:"file" db:"file"`
	Image          string    `json:"image" db:"image"`
	Comments       string    `json:"comments" db:"comments"`
	Feedback       string    `json:"feedback" db:"feedback"`
	Score          *float64  `json:"score,omitempty" db:"score"`
	LateSubmission bool      `json:"late" db:"late_submission"`
	CreatedAt      string    `json:"created_at" db:"created_at"`
	ReplacedAt     string    `json:"replaced_at" db:"replaced_at"`
}

type RubricScore struct {
	RubricID  uuid.UUID `json:"rubric_id" db:"rubric_id"`
	Criterion string    `json:"criterion,omitempty" db:"criterion"`
	MaxPoints float64   `json:"max_points,omitempty" db:"max_points"`
	Points    float64   `json:"points" db:"points"`
}

type DashboardItem struct {
	SubmissionID   uuid.UUID `json:"submission_id" db:"submission_id"`
	AssignmentID   uuid.UUID `json:"assignment_id" db:"assignment_id"`
	Task           string    `json:"task" db:"task"`
	SubjectCode    string    `json:"subject_code" db:"subject_code"`
	DueDate        string    `json:"due_date" db:"due_date"`
	Status         string    `json:"status" db:"status"`
	Submitted      bool      `json:"submitted" db:"submitted"`
	Late           bool      `json:"late" db:"late"`
	Score          *float64  `json:"score,omitempty" db:"score"`
	MaxScore       float64   `json:"max_score" db:"max_score"`
	Feedback       string    `json:"feedback,omitempty" db:"feedback"`
	HoursUntilDue  float64   `json:"hours_until_due" db:"hours_until_due"`
	ReminderHours  int       `json:"reminder_hours" db:"reminder_hours"`
	AttemptNumber  int       `json:"attempt_number" db:"attempt_number"`
	AllowResubmit  bool      `json:"allow_resubmission" db:"allow_resubmission"`
}

type LearnerDashboard struct {
	Summary       DashboardSummary `json:"summary"`
	Upcoming      []DashboardItem  `json:"upcoming"`
	Overdue       []DashboardItem  `json:"overdue"`
	PendingReview []DashboardItem  `json:"pending_review"`
	Graded        []DashboardItem  `json:"graded"`
	Reminders     []DashboardItem  `json:"reminders"`
}

type DashboardSummary struct {
	Upcoming      int `json:"upcoming"`
	Overdue       int `json:"overdue"`
	PendingReview int `json:"pending_review"`
	Graded        int `json:"graded"`
}
