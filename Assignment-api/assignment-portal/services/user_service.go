package services

import (
	"errors"
	"net/mail"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/lokesh2201013/apperrors"
	"github.com/lokesh2201013/dto"
	"github.com/lokesh2201013/models"
	"github.com/lokesh2201013/repositories"
	"github.com/lokesh2201013/utils"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

var branchRegex = regexp.MustCompile(`^[A-Z0-9]{1,5}$`)
var tenantIDRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

type userRepository interface {
	Create(user *models.User) error
	FindByEmail(email string) (models.User, error)
	FindAdmins() ([]models.User, error)
}

type tokenGenerator interface {
	Generate(userID string, role string) (string, error)
}

type tenantUserRepository interface {
	CreateInTenant(user *models.User, tenantID string) error
	FindByEmailInTenant(email string, tenantID string) (models.User, error)
	FindAdminsInTenant(tenantID string) ([]models.User, error)
}

type tenantTokenGenerator interface {
	GenerateForTenant(userID string, role string, tenantID string) (string, error)
}

type UserService struct {
	users userRepository
	jwt   tokenGenerator
}

func NewUserService(users userRepository, jwt tokenGenerator) *UserService {
	return &UserService{users: users, jwt: jwt}
}

func (s *UserService) Register(req dto.RegisterRequest) (dto.UserResponse, error) {
	req.TenantID = normalizeTenantID(req.TenantID)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	req.Branch = strings.ToUpper(strings.TrimSpace(req.Branch))
	if !tenantIDRegex.MatchString(req.TenantID) {
		return dto.UserResponse{}, apperrors.New(apperrors.ErrValidation, "Invalid tenant_id")
	}
	if req.TenantID != "default" {
		return dto.UserResponse{}, apperrors.New(apperrors.ErrForbidden, "Tenant accounts require an invitation")
	}

	if req.Name == "" {
		return dto.UserResponse{}, apperrors.New(apperrors.ErrValidation, "Name is required")
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		return dto.UserResponse{}, apperrors.New(apperrors.ErrValidation, "Invalid email format")
	}
	if len(req.Password) < 8 {
		return dto.UserResponse{}, apperrors.New(apperrors.ErrValidation, "Password must be at least 8 characters long")
	}
	if req.Role != "" && req.Role != RoleUser {
		return dto.UserResponse{}, apperrors.New(apperrors.ErrForbidden, "Users cannot register as admins")
	}
	if req.Branch == "" || !branchRegex.MatchString(req.Branch) {
		return dto.UserResponse{}, apperrors.New(apperrors.ErrValidation, "Invalid branch. Must be 1-5 alphanumeric characters long.")
	}
	if req.Semester < 1 || req.Semester > 20 {
		return dto.UserResponse{}, apperrors.New(apperrors.ErrValidation, "Invalid semester. Must be between 1 and 20.")
	}

	var lookupErr error
	if repo, ok := s.users.(tenantUserRepository); ok {
		_, lookupErr = repo.FindByEmailInTenant(req.Email, req.TenantID)
	} else {
		_, lookupErr = s.users.FindByEmail(req.Email)
	}
	if lookupErr == nil {
		return dto.UserResponse{}, apperrors.New(apperrors.ErrConflict, "Email already in use")
	} else if !errors.Is(lookupErr, apperrors.ErrNotFound) {
		return dto.UserResponse{}, lookupErr
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return dto.UserResponse{}, apperrors.Wrap(apperrors.ErrUnexpected, "Could not process password", err)
	}

	user := models.User{
		UserID:   uuid.New(),
		TenantID: req.TenantID,
		Name:     req.Name,
		Email:    req.Email,
		Password: hashedPassword,
		Role:     RoleUser,
		Branch:   req.Branch,
		Semester: req.Semester,
	}

	var createErr error
	if repo, ok := s.users.(tenantUserRepository); ok {
		createErr = repo.CreateInTenant(&user, req.TenantID)
	} else {
		createErr = s.users.Create(&user)
	}
	if createErr != nil {
		if repositories.IsUniqueViolation(createErr) {
			return dto.UserResponse{}, apperrors.New(apperrors.ErrConflict, "Email already in use")
		}
		return dto.UserResponse{}, createErr
	}
	return ToUserResponse(user), nil
}

func (s *UserService) Login(req dto.LoginRequest) (dto.LoginResponse, error) {
	tenantID := normalizeTenantID(req.TenantID)
	if !tenantIDRegex.MatchString(tenantID) {
		return dto.LoginResponse{}, apperrors.New(apperrors.ErrValidation, "Invalid tenant_id")
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || req.Password == "" {
		return dto.LoginResponse{}, apperrors.New(apperrors.ErrValidation, "Email and password are required")
	}

	var user models.User
	var err error
	if repo, ok := s.users.(tenantUserRepository); ok {
		user, err = repo.FindByEmailInTenant(email, tenantID)
	} else {
		user, err = s.users.FindByEmail(email)
	}
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return dto.LoginResponse{}, apperrors.New(apperrors.ErrUnauthorized, "Invalid credentials")
		}
		return dto.LoginResponse{}, err
	}

	if err := utils.CheckPassword(user.Password, req.Password); err != nil {
		return dto.LoginResponse{}, apperrors.New(apperrors.ErrUnauthorized, "Invalid credentials")
	}

	var token string
	if generator, ok := s.jwt.(tenantTokenGenerator); ok {
		token, err = generator.GenerateForTenant(user.UserID.String(), user.Role, tenantID)
	} else {
		token, err = s.jwt.Generate(user.UserID.String(), user.Role)
	}
	if err != nil {
		return dto.LoginResponse{}, apperrors.Wrap(apperrors.ErrUnexpected, "Could not generate token", err)
	}

	return dto.LoginResponse{Message: "Login successful", Token: token}, nil
}

func (s *UserService) GetAllAdmins() ([]dto.UserResponse, error) {
	admins, err := s.users.FindAdmins()
	if err != nil {
		return nil, err
	}
	out := make([]dto.UserResponse, 0, len(admins))
	for _, admin := range admins {
		out = append(out, ToUserResponse(admin))
	}
	return out, nil
}

func (s *UserService) GetAllAdminsInTenant(tenantID string) ([]dto.UserResponse, error) {
	repo, ok := s.users.(tenantUserRepository)
	if !ok {
		return s.GetAllAdmins()
	}
	admins, err := repo.FindAdminsInTenant(normalizeTenantID(tenantID))
	if err != nil {
		return nil, err
	}
	out := make([]dto.UserResponse, 0, len(admins))
	for _, admin := range admins {
		out = append(out, ToUserResponse(admin))
	}
	return out, nil
}

func normalizeTenantID(tenantID string) string {
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	if tenantID == "" {
		return "default"
	}
	return tenantID
}

func ToUserResponse(user models.User) dto.UserResponse {
	return dto.UserResponse{
		UserID:   user.UserID,
		TenantID: user.TenantID,
		Name:     user.Name,
		Email:    user.Email,
		Role:     user.Role,
		Branch:   user.Branch,
		Semester: user.Semester,
	}
}
