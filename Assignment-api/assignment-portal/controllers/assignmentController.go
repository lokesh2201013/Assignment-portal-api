package controllers

import (
	"fmt"
	"net/http"
	"time"
     "os"
	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/database"
	"github.com/lokesh2201013/models"
	//"strconv"
	"github.com/google/uuid"
)

func UploadAssignment(c *fiber.Ctx) error {
	var submission models.SubmitAssignment

	
	assignmentID := c.FormValue("assignment_id")
	userID := c.FormValue("user_id")
	dueDateStr := c.FormValue("due_date")
	comments := c.FormValue("comments")

	parsedAssignmentID, err := uuid.Parse(assignmentID)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid assignment_id"})
	}
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user_id"})
	}

	image, imageErr := c.FormFile("image")
	if imageErr != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Image file is required"})
	}
	
	file, fileErr := c.FormFile("file")
	if fileErr != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "File upload is required"})
	}

	// Save image
	imageDir := "./uploads/images"
	if err := os.MkdirAll(imageDir, os.ModePerm); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create image directory"})
	}
	imagePath := fmt.Sprintf("%s/%d_%s", imageDir, time.Now().UnixNano(), image.Filename)
	if saveErr := c.SaveFile(image, imagePath); saveErr != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save image"})
	}

	
	fileDir := "./uploads/files"
	if err := os.MkdirAll(fileDir, os.ModePerm); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create file directory"})
	}
	filePath := fmt.Sprintf("%s/%d_%s", fileDir, time.Now().UnixNano(), file.Filename)
	if saveErr := c.SaveFile(file, filePath); saveErr != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save file"})
	}

	// Parse due date to check lateness
	dueDate, err := time.Parse(time.RFC3339, dueDateStr)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid due_date format, must be RFC3339"})
	}

	// Populate submission struct
	submission.AssignmentID = parsedAssignmentID
	submission.UserID = parsedUserID
	submission.File = filePath
	submission.Image = imagePath
	submission.Comments = comments
	submission.LateSubmission = time.Now().After(dueDate)
	submission.CreatedAt = time.Now().Format(time.RFC3339)

	
	if dbErr := database.DB.Create(&submission).Error; dbErr != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error saving submission"})
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"message": "Assignment submitted successfully",
		"submission": submission,
	})
}

func AssignToStudents(c *fiber.Ctx) error {
	var assignment models.Assignment
	if err := c.BodyParser(&assignment); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Safely retrieve the user ID (as a string) from the middleware context
	adminIDStr, ok := c.Locals("userID").(string)
	if !ok {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Admin ID not found in context"})
	}

	// Convert the string to a UUID.
	parsedAdminID, err := uuid.Parse(adminIDStr)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Invalid Admin ID format"})
	}

	// Assign the UUID
	assignment.AdminID = parsedAdminID
	assignment.AssignmentID = uuid.New()
	//assignment.Status = "pending"
	assignment.CreatedAt = time.Now().Format(time.RFC3339)
	assignment.UpdatedAt = time.Now().Format(time.RFC3339)

	// Save single assignment
	if err := database.DB.Create(&assignment).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error creating assignment"})
	}

	// Fetch students for branch+semester
	var students []models.User
	if err := database.DB.Where("role=? AND branch=? AND semester=?",
		"user", assignment.Branch, assignment.Semester).Find(&students).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error fetching students"})
	}

	if len(students) == 0 {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "No students found for the given criteria"})
	}
	var Submissions []models.SubmitAssignment
	for _, student := range students {
		submission := models.SubmitAssignment{
			SubmissionID:   uuid.New(),
			AssignmentID:   assignment.AssignmentID,
			UserID:        student.UserID,
			Status:       "pending",
			CreatedAt:    time.Now().Format(time.RFC3339),
		}
		Submissions = append(Submissions, submission)
	}

	// Assuming assignTask is a function you have elsewhere in your code
	// Mock assignTask as it's not provided
	if err := database.DB.Create(&Submissions).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error creating Submissions"})
	}
	
	mail_status, err := assignTask([]models.Assignment{assignment})
	if err != nil {
		fmt.Println("Error sending emails:", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": 	 "Assignment created successfully",
		"assignment": 	assignment,
		"student_cnt": len(students),
		"mail_status": mail_status,
	})
}

