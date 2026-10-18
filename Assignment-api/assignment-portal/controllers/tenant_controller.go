package controllers

import (
	"context"
	"io"
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/lokesh2201013/apperrors"
	"github.com/lokesh2201013/dto"
	"github.com/lokesh2201013/models"
	"github.com/lokesh2201013/services"
)

type TenantService interface {
	CreateTenant(ctx context.Context, req services.TenantCreateRequest) (services.TenantCreateResponse, error)
	Invite(ctx context.Context, tenantID string, actorID uuid.UUID, req services.TenantInvitationRequest) (services.InvitationResponse, error)
	AcceptInvitation(ctx context.Context, req services.InvitationAcceptRequest) (services.InvitationAcceptResponse, error)
	CreateCourse(ctx context.Context, tenantID string, actorID uuid.UUID, req dto.CourseCreateRequest) (models.Course, error)
	ListCourses(ctx context.Context, tenantID string) ([]models.Course, error)
	BulkInvite(ctx context.Context, tenantID string, actorID uuid.UUID, req dto.BulkInvitationRequest) (dto.RosterImportResult, error)
	ImportRoster(ctx context.Context, tenantID string, actorID uuid.UUID, reader io.Reader) (dto.RosterImportResult, error)
	ListMembers(ctx context.Context, tenantID string, actorID uuid.UUID, status string, page int, limit int) ([]models.TenantMember, dto.PageMetadata, error)
	SetMemberStatus(ctx context.Context, tenantID string, actorID uuid.UUID, userID uuid.UUID, status string) error
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

func (h *TenantController) CreateCourse(c *fiber.Ctx) error {
	var req dto.CourseCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	actorID, err := parseActorID(c)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	tenantID, _ := c.Locals("tenantID").(string)
	course, err := h.service.CreateCourse(c.UserContext(), tenantID, actorID, req)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.Status(fiber.StatusCreated).JSON(course)
}

func (h *TenantController) ListCourses(c *fiber.Ctx) error {
	tenantID, _ := c.Locals("tenantID").(string)
	courses, err := h.service.ListCourses(c.UserContext(), tenantID)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(fiber.Map{"data": courses})
}

func (h *TenantController) BulkInvite(c *fiber.Ctx) error {
	var req dto.BulkInvitationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	actorID, err := parseActorID(c)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	tenantID, _ := c.Locals("tenantID").(string)
	result, err := h.service.BulkInvite(c.UserContext(), tenantID, actorID, req)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *TenantController) ImportRoster(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return respondError(c, h.logger, apperrors.New(apperrors.ErrValidation, "CSV file is required"))
	}
	handle, err := file.Open()
	if err != nil {
		return respondError(c, h.logger, apperrors.New(apperrors.ErrValidation, "Could not read roster file"))
	}
	defer handle.Close()
	actorID, err := parseActorID(c)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	tenantID, _ := c.Locals("tenantID").(string)
	result, err := h.service.ImportRoster(c.UserContext(), tenantID, actorID, handle)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *TenantController) ListMembers(c *fiber.Ctx) error {
	actorID, err := parseActorID(c)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	tenantID, _ := c.Locals("tenantID").(string)
	members, page, err := h.service.ListMembers(c.UserContext(), tenantID, actorID, c.Query("status"), c.QueryInt("page", 1), c.QueryInt("limit", 25))
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(dto.PaginatedResponse[models.TenantMember]{Data: members, Page: page})
}

func (h *TenantController) SetMemberStatus(c *fiber.Ctx) error {
	userID, err := uuid.Parse(c.Params("user_id"))
	if err != nil {
		return respondError(c, h.logger, apperrors.New(apperrors.ErrValidation, "Invalid user_id"))
	}
	var req dto.MembershipStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	actorID, err := parseActorID(c)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	tenantID, _ := c.Locals("tenantID").(string)
	if err := h.service.SetMemberStatus(c.UserContext(), tenantID, actorID, userID, req.Status); err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(fiber.Map{"message": "Membership updated", "user_id": userID, "status": req.Status})
}

func parseActorID(c *fiber.Ctx) (uuid.UUID, error) {
	value, _ := c.Locals("userID").(string)
	actorID, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, apperrors.New(apperrors.ErrUnauthorized, "Invalid user identity")
	}
	return actorID, nil
}
