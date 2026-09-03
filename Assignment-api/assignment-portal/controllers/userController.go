package controllers

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/dto"
)

type UserService interface {
	Register(req dto.RegisterRequest) (dto.UserResponse, error)
	Login(req dto.LoginRequest) (dto.LoginResponse, error)
	GetAllAdmins() ([]dto.UserResponse, error)
}

type UserController struct {
	service UserService
	logger  *slog.Logger
}

func NewUserController(service UserService, logger *slog.Logger) *UserController {
	return &UserController{service: service, logger: logger}
}

func (h *UserController) Register(c *fiber.Ctx) error {
	var req dto.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}
	user, err := h.service.Register(req)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.Status(fiber.StatusCreated).JSON(user)
}

func (h *UserController) Login(c *fiber.Ctx) error {
	var req dto.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}
	res, err := h.service.Login(req)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.Status(fiber.StatusOK).JSON(res)
}

func (h *UserController) GetAllAdmins(c *fiber.Ctx) error {
	admins, err := h.service.GetAllAdmins()
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.Status(fiber.StatusOK).JSON(admins)
}
