package models

type Assignment struct {
    ID        int    `json:"id"`
    UserID    int    `json:"user_id"`
    AdminID   int    `json:"admin_id"`
    Task      string `json:"task" validate:"required"`
    Status    string `json:"status" validate:"oneof=pending accepted rejected"`
    CreatedAt string `json:"created_at"`
    UpdatedAt string `json:"updated_at"`
}