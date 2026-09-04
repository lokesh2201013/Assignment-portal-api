// Package service contains business logic layer implementations.
package service

import (
	"context"
	"crypto/rand"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/lokesh2201013/email-service/internal/config"
	"github.com/lokesh2201013/email-service/internal/domain"
	"github.com/lokesh2201013/email-service/internal/dto"
	"github.com/lokesh2201013/email-service/internal/errors/apperror"
	"github.com/lokesh2201013/email-service/internal/repository"
)

// AuthService handles authentication business logic.
type AuthService struct {
	userRepo repository.UserRepository
	config   *config.Config
	logger   *slog.Logger
}

// NewAuthService creates a new auth service.
func NewAuthService(userRepo repository.UserRepository, cfg *config.Config, logger *slog.Logger) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		config:   cfg,
		logger:   logger,
	}
}

// Register creates a new user with hashed password.
func (s *AuthService) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.LoginResponse, error) {
	// Validate input
	if err := s.validateUsername(req.Username); err != nil {
		return nil, err
	}
	if err := s.validatePassword(req.Password); err != nil {
		return nil, err
	}

	// Check if user already exists
	_, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err == nil {
		// User exists
		return nil, apperror.Conflict("username already registered")
	}
	if !apperror.IsType(err, apperror.NotFoundError) {
		// Database error, not a not-found
		return nil, err
	}

	// Hash password
	hashedPassword, err := s.hashPassword(req.Password)
	if err != nil {
		s.logger.Error("failed to hash password", "error", err)
		return nil, apperror.Internal("failed to process registration", err)
	}

	// Create user
	user := &domain.User{
		Username: req.Username,
		Password: hashedPassword,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	s.logger.Info("user registered successfully", "username", req.Username)

	// Generate token
	return s.generateToken(user)
}

// Login authenticates a user and returns a JWT token.
func (s *AuthService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error) {
	// Get user by username
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		if apperror.IsType(err, apperror.NotFoundError) {
			return nil, apperror.Unauthorized("invalid credentials")
		}
		return nil, err
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		s.logger.Warn("failed login attempt", "username", req.Username)
		return nil, apperror.Unauthorized("invalid credentials")
	}

	s.logger.Info("user logged in", "username", req.Username)

	// Generate token
	return s.generateToken(user)
}

// VerifyToken validates a JWT token and returns the username.
func (s *AuthService) VerifyToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, apperror.Unauthorized("invalid token signing method")
		}
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		return "", apperror.Unauthorized("invalid or expired token")
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.Subject == "" {
		return "", apperror.Unauthorized("invalid token claims")
	}

	return claims.Subject, nil
}

// generateToken creates a JWT token for a user.
func (s *AuthService) generateToken(user *domain.User) (*dto.LoginResponse, error) {
	expiresAt := time.Now().Add(s.config.JWTExpiry)

	claims := &jwt.RegisteredClaims{
		Subject:   user.Username,
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		s.logger.Error("failed to sign token", "error", err)
		return nil, apperror.Internal("failed to generate token", err)
	}

	return &dto.LoginResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt,
	}, nil
}

// hashPassword hashes a password using bcrypt.
func (s *AuthService) hashPassword(password string) (string, error) {
	// Generate cost from password length for variable difficulty
	cost := bcrypt.DefaultCost
	if len(password) > 72 {
		// bcrypt has 72-byte limit, hash long passwords first
		hash := make([]byte, 32)
		if _, err := rand.Read(hash); err != nil {
			return "", err
		}
		cost = bcrypt.MaxCost
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}

	return string(hashedPassword), nil
}

// validateUsername checks username constraints.
func (s *AuthService) validateUsername(username string) error {
	if len(username) < 3 {
		return apperror.Validation("username must be at least 3 characters")
	}
	if len(username) > 50 {
		return apperror.Validation("username must not exceed 50 characters")
	}
	return nil
}

// validatePassword checks password constraints.
func (s *AuthService) validatePassword(password string) error {
	if len(password) < 8 {
		return apperror.Validation("password must be at least 8 characters")
	}
	if len(password) > 128 {
		return apperror.Validation("password must not exceed 128 characters")
	}
	return nil
}
