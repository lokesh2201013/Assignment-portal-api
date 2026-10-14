package controllers

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/apperrors"
)

func respondError(c *fiber.Ctx, logger *slog.Logger, err error) error {
	status := fiber.StatusInternalServerError
	switch {
	case errors.Is(err, apperrors.ErrValidation):
		status = fiber.StatusBadRequest
	case errors.Is(err, apperrors.ErrNotFound):
		status = fiber.StatusNotFound
	case errors.Is(err, apperrors.ErrConflict):
		status = fiber.StatusConflict
	case errors.Is(err, apperrors.ErrUnauthorized):
		status = fiber.StatusUnauthorized
	case errors.Is(err, apperrors.ErrForbidden):
		status = fiber.StatusForbidden
	case errors.Is(err, apperrors.ErrDatabase), errors.Is(err, apperrors.ErrExternal), errors.Is(err, apperrors.ErrUnexpected):
		status = fiber.StatusInternalServerError
	}

	if status >= fiber.StatusInternalServerError {
		logger.Error("request failed", slog.Any("error", err), slog.String("path", c.Path()), slog.String("method", c.Method()))
	} else {
		logger.Info("request rejected", slog.String("reason", apperrors.PublicMessage(err)), slog.String("path", c.Path()), slog.String("method", c.Method()))
	}

	message := apperrors.PublicMessage(err)
	if status >= fiber.StatusInternalServerError {
		message = "Internal server error"
	}
	return c.Status(status).JSON(fiber.Map{"error": message})
}
