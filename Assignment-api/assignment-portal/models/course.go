package models

import "github.com/google/uuid"

type Course struct {
	CourseID  uuid.UUID `json:"course_id" db:"course_id"`
	TenantID  string    `json:"tenant_id" db:"tenant_id"`
	Code      string    `json:"code" db:"code"`
	Name      string    `json:"name" db:"name"`
	Branch    string    `json:"branch" db:"branch"`
	Semester  int       `json:"semester" db:"semester"`
	CreatedBy uuid.UUID `json:"created_by" db:"created_by"`
	CreatedAt string    `json:"created_at" db:"created_at"`
}

type TenantMember struct {
	UserID   uuid.UUID `json:"user_id" db:"user_id"`
	TenantID string    `json:"tenant_id" db:"tenant_id"`
	Name     string    `json:"name" db:"name"`
	Email    string    `json:"email" db:"email"`
	Role     string    `json:"role" db:"role"`
	Status   string    `json:"status" db:"status"`
	Branch   string    `json:"branch,omitempty" db:"branch"`
	Semester int       `json:"semester,omitempty" db:"semester"`
}
