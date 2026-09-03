package models

import "github.com/google/uuid"

// Admin-created assignment
type Assignment struct {
	AssignmentID uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	AdminID      uuid.UUID `json:"admin_id"`
	Task         string    `json:"task"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
	DueDate      string    `json:"due_date"`
	Branch       string    `json:"branch"`
	Semester     int       `json:"semester"`
	SubjectCode  string    `json:"subject_code"`

	// one-to-many relation
	// Submissions []SubmitAssignment `json:"submissions"`
}

type SubmitAssignment struct {
	SubmissionID uuid.UUID `json:"submission_id"`

	AssignmentID   uuid.UUID `json:"assignment_id"`
	UserID         uuid.UUID `json:"user_id"`
	Status         string    `json:"status"`
	File           string    `json:"file"`
	Image          string    `json:"image"`
	Comments       string    `json:"comments"`
	LateSubmission bool      `json:"late"`
	CreatedAt      string    `json:"created_at"`
}
