package models

type User struct {
    ID       int    `json:"id"`
    Name     string `json:"name" validate:"required"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required"`
    Role     string `json:"role" validate:"required,oneof=user admin"`
}
