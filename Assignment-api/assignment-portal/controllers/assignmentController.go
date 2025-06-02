package controllers

import (
	"fmt"
	"net/http"
	"time"
     "os"
	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/assignment-portal/database"
	"github.com/lokesh2201013/assignment-portal/models"
	"strconv"
)

func UploadAssignment(c *fiber.Ctx) error {
	var assignment models.SubmitAssignment

	// Parse text fields
	id:=c.FormValue("user_id")
	userid:=c.FormValue("user_id")
    adminid:=c.FormValue("admin_id")
	dueDateStr := c.FormValue("due_date")
	comments := c.FormValue("comments")

	// Get uploaded image
	image, imageErr := c.FormFile("image")
	if imageErr != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Image file is required"})
	}

	// Get uploaded file (optional)
	file, fileErr := c.FormFile("file")
	if fileErr != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "File upload is required"})
	}

	// Optional: Get additional comments (map this to your model if needed)
	
	assignment.Comments = comments // Add to model if not already

	// Late submission check
	
              dueDate, err := time.Parse(time.RFC3339, dueDateStr)
              if err != nil {
              return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid due_date format"})
              }
            assignment.LateSubmission = time.Now().After(dueDate)


	// Save image to local directory
	imageDir := "./uploads/images"
	if err := os.MkdirAll(imageDir, os.ModePerm); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create image directory"})
	}
	imagePath := fmt.Sprintf("%s/%d_%s", imageDir, time.Now().UnixNano(), image.Filename)
	if saveErr := c.SaveFile(image, imagePath); saveErr != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save image"})
	}
	assignment.Image = imagePath // Make sure this field exists in your model

	// Save file to local directory
	fileDir := "./uploads/files"
	if err := os.MkdirAll(fileDir, os.ModePerm); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create file directory"})
	}
	filePath := fmt.Sprintf("%s/%d_%s", fileDir, time.Now().UnixNano(), file.Filename)
	if saveErr := c.SaveFile(file, filePath); saveErr != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save file"})
	}
	assignment.File = filePath // Make sure this field exists in your model

	// Save to DB
	assignment.AssignmentDetails.UserID, _ = strconv.Atoi(userid)
	assignment.AssignmentDetails.AdminID, _ = strconv.Atoi(adminid)
	assignment.AssignmentDetails.DueDate = dueDateStr
	assignment.AssignmentDetails.Status = "pending"
	assignment.Comments = comments
	assignment.AssignmentDetails.ID,_=strconv.Atoi(id)

	if dbErr := database.DB.Create(&assignment).Error; dbErr != nil {
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
			ID:        assignment.ID,
			UserID:    student.UserID,
			Email:     student.Email,
			AdminID:   assignment.AdminID,
			Task:      assignment.Task,
			Status:    "pending",
			Branch:    assignment.Branch,
			Semester:  assignment.Semester,
			CreatedAt: time.Now().Format(time.RFC3339),
			UpdatedAt: time.Now().Format(time.RFC3339),
			DueDate:   assignment.DueDate,
		})
	}

	if err := database.DB.Create(&assignments).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error assigning tasks"})
	}

	fmt.Println("Working fine1")

    mail_status,err:=assignTask(assignments)
	if err!=nil{
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error":  err})
	}
	fmt.Println("Working fine 10")
	return c.JSON(fiber.Map{"message": "Assignment assigned successfully ",
	 "count": len(assignments),
	 "Mail Status":mail_status})
}
