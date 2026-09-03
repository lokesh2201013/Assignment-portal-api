package controllers

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/apperrors"
	"github.com/lokesh2201013/dto"
)

type fakeUserService struct {
	registerErr error
	loginErr    error
}

func (s fakeUserService) Register(req dto.RegisterRequest) (dto.UserResponse, error) {
	if s.registerErr != nil {
		return dto.UserResponse{}, s.registerErr
	}
	return dto.UserResponse{Name: req.Name, Email: req.Email, Role: "user"}, nil
}

func (s fakeUserService) Login(req dto.LoginRequest) (dto.LoginResponse, error) {
	if s.loginErr != nil {
		return dto.LoginResponse{}, s.loginErr
	}
	return dto.LoginResponse{Message: "Login successful", Token: "token"}, nil
}

func (s fakeUserService) GetAllAdmins() ([]dto.UserResponse, error) {
	return []dto.UserResponse{}, nil
}

func TestRegisterMapsValidationErrors(t *testing.T) {
	app := fiber.New()
	controller := NewUserController(fakeUserService{registerErr: apperrors.New(apperrors.ErrValidation, "Invalid email format")}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	app.Post("/signup", controller.Register)

	body, _ := json.Marshal(dto.RegisterRequest{Name: "Student"})
	req := httptest.NewRequest("POST", "/signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected %d, got %d", fiber.StatusBadRequest, resp.StatusCode)
	}
}

func TestRegisterDoesNotExposePasswordField(t *testing.T) {
	app := fiber.New()
	controller := NewUserController(fakeUserService{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	app.Post("/signup", controller.Register)

	body, _ := json.Marshal(dto.RegisterRequest{Name: "Student", Email: "student@example.com", Password: "password123"})
	req := httptest.NewRequest("POST", "/signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)

	if err != nil {
		t.Fatalf("app test: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected %d, got %d", fiber.StatusCreated, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if bytes.Contains(data, []byte("password")) {
		t.Fatalf("response exposed password field: %s", string(data))
	}
}
