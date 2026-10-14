// Package repository defines data access interfaces and their implementations.
package repository

import (
	"context"

	"github.com/lokesh2201013/email-service/internal/domain"
)

// UserRepository defines user persistence operations.
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, userID uint) error
}

// SenderRepository defines sender persistence operations.
type SenderRepository interface {
	Create(ctx context.Context, sender *domain.Sender) error
	GetByEmail(ctx context.Context, email string) (*domain.Sender, error)
	GetByID(ctx context.Context, senderID uint) (*domain.Sender, error)
	ListByAdminAndVerified(ctx context.Context, adminName string, verified bool) ([]domain.Sender, error)
	Update(ctx context.Context, sender *domain.Sender) error
	Delete(ctx context.Context, email string) error
	Exists(ctx context.Context, email string) (bool, error)
}

// TemplateRepository defines template persistence operations.
type TemplateRepository interface {
	Create(ctx context.Context, template *domain.Template) error
	GetByID(ctx context.Context, templateID uint) (*domain.Template, error)
	GetByName(ctx context.Context, name string) (*domain.Template, error)
	List(ctx context.Context) ([]domain.Template, error)
	Update(ctx context.Context, template *domain.Template) error
	Delete(ctx context.Context, templateID uint) error
}

// AnalyticsRepository defines analytics persistence operations.
type AnalyticsRepository interface {
	Create(ctx context.Context, analytics *domain.Analytics) error
	GetBySenderID(ctx context.Context, senderID uint) (*domain.Analytics, error)
	GetByAdminName(ctx context.Context, adminName string) ([]domain.Analytics, error)
	Update(ctx context.Context, analytics *domain.Analytics) error
	UpdateBySenderID(ctx context.Context, senderID uint, analytics *domain.Analytics) error
	ListByAdminName(ctx context.Context, adminName string) ([]domain.Analytics, error)
}

// Repositories holds all repository implementations.
type Repositories struct {
	User      UserRepository
	Sender    SenderRepository
	Template  TemplateRepository
	Analytics AnalyticsRepository
}
