package models

type Assignment struct {
    ID        int    `json:"id"`
    Email     string `json:"email" validate:"required,email"`
    UserID    int    `json:"user_id"`
    AdminID   int    `json:"admin_id"`
    Task      string `json:"task" validate:"required"`
    Status    string `json:"status" validate:"oneof=pending accepted rejected"`
    CreatedAt string `json:"created_at"`
    UpdatedAt string `json:"updated_at"`
    Branch   string `json: "branch" validate:"required" `
    Semester int    `json: "semester" validate:"required"`
}
