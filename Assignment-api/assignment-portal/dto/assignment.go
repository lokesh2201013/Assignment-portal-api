package dto

import "github.com/google/uuid"

type AssignmentCreateRequest struct {
	Email       string `json:"email"`
	Task        string `json:"task"`
	DueDate     string `json:"due_date"`
	Branch      string `json:"branch"`
	Semester    int    `json:"semester"`
	SubjectCode string `json:"subject_code"`
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
