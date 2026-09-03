package services

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lokesh2201013/apperrors"
	"github.com/lokesh2201013/dto"
	"github.com/lokesh2201013/models"
	"github.com/lokesh2201013/utils"
)

type fakeUserRepo struct {
	byEmail map[string]models.User
	admins  []models.User
	created *models.User
	err     error
}

func (r *fakeUserRepo) Create(user *models.User) error {
	r.created = user
	return r.err
}

func (r *fakeUserRepo) FindByEmail(email string) (models.User, error) {
	if r.err != nil {
		return models.User{}, r.err
	}
	user, ok := r.byEmail[email]
	if !ok {
		return models.User{}, apperrors.New(apperrors.ErrNotFound, "User not found")
	}
	return user, nil
}

func (r *fakeUserRepo) FindAdmins() ([]models.User, error) {
	return r.admins, r.err
}

func TestRegisterRejectsAdminRole(t *testing.T) {
	service := NewUserService(&fakeUserRepo{byEmail: map[string]models.User{}}, utils.NewJWTManager("secret", "test", time.Hour))

	_, err := service.Register(dto.RegisterRequest{
		Name:     "Student",
		Email:    "student@example.com",
		Password: "password123",
		Role:     RoleAdmin,
		Branch:   "cse",
		Semester: 3,
	})

	if !errors.Is(err, apperrors.ErrForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestRegisterReturnsDTOAndStoresHash(t *testing.T) {
	repo := &fakeUserRepo{byEmail: map[string]models.User{}}
	service := NewUserService(repo, utils.NewJWTManager("secret", "test", time.Hour))

	res, err := service.Register(dto.RegisterRequest{
		Name:     "Student",
		Email:    "Student@Example.com",
		Password: "password123",
		Branch:   "cse",
		Semester: 3,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Email != "student@example.com" || res.Role != RoleUser || res.Branch != "CSE" {
		t.Fatalf("unexpected response: %+v", res)
	}
	if repo.created.Password == "password123" || strings.Contains(repo.created.Password, "password123") {
		t.Fatalf("password was not hashed")
	}
}

func TestLoginUsesGenericInvalidCredentials(t *testing.T) {
	service := NewUserService(&fakeUserRepo{byEmail: map[string]models.User{}}, utils.NewJWTManager("secret", "test", time.Hour))

	_, err := service.Login(dto.LoginRequest{Email: "missing@example.com", Password: "password123"})

	if !errors.Is(err, apperrors.ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
	if !strings.Contains(err.Error(), "Invalid credentials") {
		t.Fatalf("expected generic invalid credentials message, got %v", err)
	}
}

func TestLoginReturnsTokenForValidPassword(t *testing.T) {
	hash, err := utils.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	manager := utils.NewJWTManager("secret", "test", time.Hour)
	userID := uuid.New()
	service := NewUserService(&fakeUserRepo{byEmail: map[string]models.User{
		"student@example.com": {
			UserID:   userID,
			Email:    "student@example.com",
			Password: hash,
			Role:     RoleUser,
		},
	}}, manager)

	res, err := service.Login(dto.LoginRequest{Email: "student@example.com", Password: "password123"})

	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}
	if res.Token == "" {
		t.Fatalf("expected token")
	}
	claims, err := manager.Parse(res.Token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.UserID != userID.String() || claims.Role != RoleUser {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestRegisterPropagatesRepositoryErrors(t *testing.T) {
	service := NewUserService(&fakeUserRepo{err: errors.New("db down")}, utils.NewJWTManager("secret", "test", time.Hour))

	_, err := service.Register(dto.RegisterRequest{
		Name:     "Student",
		Email:    "student@example.com",
		Password: "password123",
		Branch:   "cse",
		Semester: 3,
	})

	if err == nil {
		t.Fatalf("expected repository error")
	}
}
