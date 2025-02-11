package models

import (
	"gorm.io/gorm"
	"time"
)

type Sender struct {
	ID        uint `gorm:"primarykey"`
	AdminName  string `json:"admin_name" gorm:"not null"`
	Email      string `json:"email" gorm:"unique;not null"`
	SMTPHost   string `json:"smtp_host" gorm:"not null"`
	SMTPPort   int    `json:"smtp_port" gorm:"not null"`
	Username   string `json:"username" gorm:"not null"`
   AppPassword string `json:"password" gorm:"column:password"`
	Verified   bool   `json:"verified" gorm:"default:false"`
}

type Template struct {
	ID      uint   `gorm:"primaryKey"`
	Name    string `gorm:"unique"`
	Subject string
	Body    string
	Format  string `gorm:"default:'text'"`
}

type User struct {
	ID        uint           `gorm:"primaryKey"`
	SandboxTime time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
	Username   string          `json:"username" gorm:"unique;not null"`
	Password   string          `json:"password" gorm:"not null"`
}

type Analytics struct {
	ID             uint      `gorm:"primaryKey"`
	AdminName      string    `json:"admin_name" gorm:"not null"` // foreign key
	SenderID       uint      `gorm:"not null"`                  // foreign key
	Sender         Sender    `gorm:"foreignKey:SenderID"`
	TotalEmails    int       `json:"total_emails" gorm:"default:0"`
	AccumulatedEmail int     `json:"accumulatedemails" gorm:"default:0"`
	Delivered      int       `json:"delivered" gorm:"default:0"`
	Bounced        int       `json:"bounced" gorm:"default:0"`
	Complaints     int       `json:"complaints" gorm:"default:0"`
	Rejected       int       `json:"rejected" gorm:"default:0"`
	DeliveryRate   float64   `json:"delivery_rate" gorm:"-"`
	BounceRate     float64   `json:"bounce_rate" gorm:"-"`
	ComplaintRate  float64   `json:"complaint_rate" gorm:"-"`
	RejectRate     float64   `json:"reject_rate" gorm:"-"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
