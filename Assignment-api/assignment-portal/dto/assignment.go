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
}

type AssignmentSubmissionUpload struct {
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
