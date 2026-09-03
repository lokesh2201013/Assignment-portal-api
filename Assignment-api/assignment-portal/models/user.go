package models

import "github.com/google/uuid"

type User struct {
	UserID   uuid.UUID `json:"user_id"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Password string    `json:"-"`
	Role     string    `json:"role"`
	Branch   string    `json:"branch,omitempty"`
	Semester int       `json:"semester,omitempty"`
}
