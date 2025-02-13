package controllers

import (
	"net/http"
	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/assignment-portal/database"
	"github.com/lokesh2201013/assignment-portal/models"
	"time"
)

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

func AssignTostudents(c *fiber.Ctx) error{
	var assignment models.Assignment
	
	if err:= c.BodyParser(&assignment);err!=nil{
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	var students []models.User

	if err:= database.DB.Where("role=? AND branch=? AND semester=? ","user",assignment.Branch,assignment.Semester).Find(&students).Error; err!=nil{
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error fetching students"})
	}

	if len(students) == 0 {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "No students found for the given criteria"})
	}

	 var assignments []models.Assignment

	 for _, student := range students {
		assignments = append(assignments, models.Assignment{
			UserID:    student.ID,
			AdminID:   assignment.AdminID,
			Task:      assignment.Task,
			Status:    "pending",
			Branch:    assignment.Branch,
			Semester:  assignment.Semester,
			CreatedAt: time.Now().Format(time.RFC3339),
			UpdatedAt: time.Now().Format(time.RFC3339),
		})
	}

	if err := database.DB.Create(&assignments).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error assigning tasks"})
	}

	return c.JSON(fiber.Map{"message": "Assignment assigned successfully", "count": len(assignments)})
}
