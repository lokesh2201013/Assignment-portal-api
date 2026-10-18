package controllers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/apperrors"
)

func respondError(c *fiber.Ctx, logger *slog.Logger, err error) error {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, apperrors.ErrValidation):
		status = http.StatusBadRequest
	case errors.Is(err, apperrors.ErrUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, apperrors.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, apperrors.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, apperrors.ErrConflict):
		status = http.StatusConflict
	}
	if logger != nil {
		logger.Error("request failed", slog.Int("status", status), slog.Any("error", err))
	}
	return c.Status(status).JSON(fiber.Map{"error": apperrors.PublicMessage(err)})
}
