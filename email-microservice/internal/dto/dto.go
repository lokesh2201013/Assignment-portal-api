// Package dto defines request and response DTOs for HTTP handlers.
package dto

import "time"

// --- Authentication DTOs ---

type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// --- Sender DTOs ---

type VerifyEmailRequest struct {
	AdminName   string `json:"admin_name" validate:"required"`
	Email       string `json:"email" validate:"required,email"`
	SMTPHost    string `json:"smtp_host" validate:"required"`
	SMTPPort    int    `json:"smtp_port" validate:"required,min=1,max=65535"`
	Username    string `json:"username" validate:"required"`
	AppPassword string `json:"password" validate:"required"`
}

type SenderResponse struct {
	ID        uint      `json:"id"`
	Email     string    `json:"email"`
	AdminName string    `json:"admin_name"`
	SMTPHost  string    `json:"smtp_host"`
	SMTPPort  int       `json:"smtp_port"`
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ListEmailResponse struct {
	Emails []string `json:"emails"`
}

// --- Template DTOs ---

type CreateTemplateRequest struct {
	Name    string `json:"name" validate:"required"`
	Subject string `json:"subject" validate:"required"`
	Body    string `json:"body" validate:"required"`
	Format  string `json:"format" validate:"required,oneof=text html"`
}

type TemplateResponse struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	Format    string    `json:"format"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// --- Email DTOs ---

type SendEmailRequest struct {
	From    string   `json:"from" validate:"required,email"`
	To      []string `json:"to" validate:"required,min=1,dive,email"`
	Subject string   `json:"subject" validate:"required,min=1,max=255"`
	Body    string   `json:"body" validate:"required"`
	Format  string   `json:"format" validate:"oneof=text html"`
}

type SendEmailResponse struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// --- Analytics DTOs ---

type AnalyticsResponse struct {
	SenderEmail   string  `json:"sender_email,omitempty"`
	TotalEmails   int     `json:"total_emails"`
	Delivered     int     `json:"delivered"`
	Bounced       int     `json:"bounced"`
	Complaints    int     `json:"complaints"`
	Rejected      int     `json:"rejected"`
	DeliveryRate  float64 `json:"delivery_rate"`
	BounceRate    float64 `json:"bounce_rate"`
	ComplaintRate float64 `json:"complaint_rate"`
	RejectRate    float64 `json:"reject_rate"`
}

type AdminAnalyticsResponse struct {
	Senders []AnalyticsResponse `json:"senders"`
}

// --- Error Response ---

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

// --- Generic Response ---

type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
