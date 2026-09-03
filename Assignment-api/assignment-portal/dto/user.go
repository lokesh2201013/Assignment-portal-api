package dto

import "github.com/google/uuid"

type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
	Branch   string `json:"branch,omitempty"`
	Semester int    `json:"semester,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserResponse struct {
	UserID   uuid.UUID `json:"user_id"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	Branch   string    `json:"branch,omitempty"`
	Semester int       `json:"semester,omitempty"`
}

type LoginResponse struct {
	Message string `json:"message"`
	Token   string `json:"token"`
}
