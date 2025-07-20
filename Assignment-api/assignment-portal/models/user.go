package models


type User struct {
	UserID       int    `json:"user_id"`
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
	Role     string `json:"role" validate:"required,oneof=user admin"`
	Branch   string `json:"branch,omitempty"`
	Semester int    `json:"semester,omitempty"`
}


