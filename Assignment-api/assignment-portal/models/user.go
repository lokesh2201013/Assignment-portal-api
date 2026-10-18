package models

import "github.com/google/uuid"

type User struct {
	UserID   uuid.UUID `json:"user_id"  gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	TenantID string    `json:"tenant_id" db:"tenant_id" gorm:"column:tenant_id"`
	Name     string    `json:"name" validate:"required"`
	Email    string    `json:"email" validate:"required,email"`
	Password string    `json:"password" validate:"required"`
	Role     string    `json:"role" validate:"required,oneof=user admin"`
	Branch   string    `json:"branch,omitempty"`
	Semester int       `json:"semester,omitempty"`
}
