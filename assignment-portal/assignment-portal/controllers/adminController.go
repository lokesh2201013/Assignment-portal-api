package controllers

import (
	"net/http"
	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/assignment-portal/database"
	"github.com/lokesh2201013/assignment-portal/models"
)

func GetAdminAssignments(c *fiber.Ctx) error {
	var assignments []models.Assignment
	if err := database.DB.Find(&assignments).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error fetching assignments"})
	}
	return c.JSON(assignments)
}

func AcceptAssignment(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := database.DB.Model(&models.Assignment{}).Where("id = ?", id).Update("status", "accepted").Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error updating assignment status"})
	}
	return c.JSON(fiber.Map{"message": "Assignment accepted"})
}

func RejectAssignment(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := database.DB.Model(&models.Assignment{}).Where("id = ?", id).Update("status", "rejected").Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error updating assignment status"})
	}
	return c.JSON(fiber.Map{"message": "Assignment rejected"})
}

func UploadAssignment(c *fiber.Ctx) error {
	var assignment models.Assignment
	if err := c.BodyParser(&assignment); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := database.DB.Create(&assignment).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error uploading assignment"})
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{"message": "Assignment uploaded successfully"})
}

func GetUserAssignments(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)
	var assignments []models.Assignment

	if err := database.DB.Where("user_id = ?", userID).Find(&assignments).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error fetching user assignments"})
	}

	return c.JSON(assignments)
}