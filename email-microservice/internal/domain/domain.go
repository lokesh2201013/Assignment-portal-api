// Package domain contains domain models representing core business entities.
package domain

import "time"

// Sender represents an email sender identity verified with SMTP credentials.
type Sender struct {
	ID        uint
	AdminName string
	Email     string
	SMTPHost  string
	SMTPPort  int
	Username  string
	Password  string // Never expose in responses
	Verified  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Template represents a reusable email template.
type Template struct {
	ID        uint
	Name      string
	Subject   string
	Body      string
	Format    string // "text" or "html"
	CreatedAt time.Time
	UpdatedAt time.Time
}

// User represents an admin user of the email service.
type User struct {
	ID        uint
	Username  string
	Password  string // Never expose in responses; always hashed
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time // Soft delete support
}

// Analytics tracks email delivery metrics for a sender.
type Analytics struct {
	ID               uint
	AdminName        string
	SenderID         uint
	TotalEmails      int
	AccumulatedEmail int
	Delivered        int
	Bounced          int
	Complaints       int
	Rejected         int
	DeliveryRate     float64 // Calculated, not stored
	BounceRate       float64 // Calculated, not stored
	ComplaintRate    float64 // Calculated, not stored
	RejectRate       float64 // Calculated, not stored
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// CalculateMetrics computes delivery, bounce, complaint, and reject rates.
func (a *Analytics) CalculateMetrics() {
	if a.TotalEmails == 0 {
		return
	}
	a.DeliveryRate = float64(a.Delivered) / float64(a.TotalEmails) * 100
	a.BounceRate = float64(a.Bounced) / float64(a.TotalEmails) * 100
	a.ComplaintRate = float64(a.Complaints) / float64(a.TotalEmails) * 100
	a.RejectRate = float64(a.Rejected) / float64(a.TotalEmails) * 100
}
