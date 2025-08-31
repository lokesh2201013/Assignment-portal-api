package models

import "github.com/google/uuid"

// Admin-created assignment
type Assignment struct {
    AssignmentID uuid.UUID `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
    Email        string    `json:"email" validate:"required,email"`
    AdminID      uuid.UUID `json:"admin_id"`  // admin who created
    Task         string    `json:"task" validate:"required"`
    CreatedAt    string    `json:"created_at"`
    UpdatedAt    string    `json:"updated_at"`
    DueDate      string    `json:"due_date"`
    Branch       string    `json:"branch" validate:"required"`
    Semester     int       `json:"semester" validate:"required"`
    SubjectCode  string    `json:"subject_code" validate:"required"`

    // one-to-many relation
    //Submissions []SubmitAssignment `json:"submissions" gorm:"foreignKey:AssignmentID"`
}

type SubmitAssignment struct {
    SubmissionID uuid.UUID `json:"submission_id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`

    AssignmentID uuid.UUID `json:"assignment_id" gorm:"type:uuid;not null"`
    UserID       uuid.UUID `json:"user_id" gorm:"type:uuid;not null"`
    Status       string    `json:"status" validate:"oneof=pending accepted rejected"`
    File     string `json:"file"`
    Image    string `json:"image"`
    Comments string `json:"comments"`
    LateSubmission bool `json:"late"`
    CreatedAt string `json:"created_at"`
}
