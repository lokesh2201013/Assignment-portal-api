package controllers

import (
	"context"
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/lokesh2201013/apperrors"
	"github.com/lokesh2201013/services"
)

type TenantService interface {
	CreateTenant(ctx context.Context, req services.TenantCreateRequest) (services.TenantCreateResponse, error)
	Invite(ctx context.Context, tenantID string, actorID uuid.UUID, req services.TenantInvitationRequest) (services.InvitationResponse, error)
	AcceptInvitation(ctx context.Context, req services.InvitationAcceptRequest) (services.InvitationAcceptResponse, error)
}

type TenantController struct {
	service TenantService
	logger  *slog.Logger
}

func NewTenantController(service TenantService, logger *slog.Logger) *TenantController {
	return &TenantController{service: service, logger: logger}
}

func (h *TenantController) Create(c *fiber.Ctx) error {
	var req services.TenantCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	result, err := h.service.CreateTenant(c.UserContext(), req)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *TenantController) Invite(c *fiber.Ctx) error {
	var req services.TenantInvitationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	actorID, err := uuid.Parse(c.Locals("userID").(string))
	if err != nil {
		return respondError(c, h.logger, apperrors.New(apperrors.ErrUnauthorized, "Invalid user identity"))
	}
	tenantID, _ := c.Locals("tenantID").(string)
	result, err := h.service.Invite(c.UserContext(), tenantID, actorID, req)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *TenantController) AcceptInvitation(c *fiber.Ctx) error {
	var req services.InvitationAcceptRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	result, err := h.service.AcceptInvitation(c.UserContext(), req)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}
