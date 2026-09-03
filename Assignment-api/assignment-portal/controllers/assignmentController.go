package controllers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/lokesh2201013/apperrors"
	"github.com/lokesh2201013/dto"
)

func (h *AssignmentController) UploadAssignment(c *fiber.Ctx) error {
	assignmentID := c.FormValue("assignment_id")
	userID := c.FormValue("user_id")

	parsedAssignmentID, err := uuid.Parse(assignmentID)
	if err != nil {
		return respondError(c, h.logger, apperrors.New(apperrors.ErrValidation, "Invalid assignment_id"))
	}
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return respondError(c, h.logger, apperrors.New(apperrors.ErrValidation, "Invalid user_id"))
	}

	image, imageErr := c.FormFile("image")
	file, fileErr := c.FormFile("file")
	if imageErr != nil || fileErr != nil {
		return respondError(c, h.logger, apperrors.New(apperrors.ErrValidation, "Image file and file upload are required"))
	}

	imagePath, filePath, err := h.service.SaveSubmissionFiles(image, file, h.imageDir, h.fileDir)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	submission, err := h.service.SubmitAssignment(dto.AssignmentSubmissionUpload{
		AssignmentID: parsedAssignmentID,
		UserID:       parsedUserID,
		DueDate:      c.FormValue("due_date"),
		Comments:     c.FormValue("comments"),
		FilePath:     filePath,
		ImagePath:    imagePath,
	})
	if err != nil {
		return respondError(c, h.logger, err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":    "Assignment submitted successfully",
		"submission": submission,
	})
}

func (h *AssignmentController) AssignToStudents(c *fiber.Ctx) error {
	var req dto.AssignmentCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	adminIDStr, ok := c.Locals("userID").(string)
	if !ok {
		return respondError(c, h.logger, apperrors.New(apperrors.ErrUnauthorized, "Admin ID not found in context"))
	}
	parsedAdminID, err := uuid.Parse(adminIDStr)
	if err != nil {
		return respondError(c, h.logger, apperrors.New(apperrors.ErrUnauthorized, "Invalid Admin ID format"))
	}

	res, err := h.service.AssignToStudents(c.UserContext(), parsedAdminID, req)
	if err != nil {
		return respondError(c, h.logger, err)
	}
	return c.JSON(res)
}
