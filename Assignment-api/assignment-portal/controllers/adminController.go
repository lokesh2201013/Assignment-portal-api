package controllers

import (
	"fmt"
	"net/http"
	//"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/lokesh2201013/database"
	"github.com/lokesh2201013/models"
	pb "github.com/lokesh2201013/proto"
)

func GetAdminAssignments(c *fiber.Ctx) error {
	var assignments []models.Assignment

	branch := c.Query("branch")
	semester := c.Query("semester")
	subjectCode := c.Query("subject_code")

	query := database.DB.Model(&models.Assignment{})
	if branch != "" {
		query = query.Where("branch = ?", branch)
	}
	if semester != "" {
		query = query.Where("semester = ?", semester)
	}

	if subjectCode != "" {
		query = query.Where("subject_code = ?", subjectCode)
	}

	if err := query.Find(&assignments).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error fetching assignments"})
	}
	return c.JSON(assignments)
}


func AcceptAssignment(c *fiber.Ctx) error {
	idList := c.Query("id")
	if idList == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "No student IDs provided"})
	}

	idStrs := strings.Split(idList, ",")
	studentIDs := make([]string, 0, len(idStrs))
	for _, s := range idStrs {
		studentIDs = append(studentIDs, strings.TrimSpace(s))
	}
	
	// First, find the submitted assignments that match the user IDs.
	var submittedAssignments []models.SubmitAssignment
	if err := database.DB.Where("user_id IN ?", studentIDs).Find(&submittedAssignments).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error getting submitted assignments"})
	}

	if len(submittedAssignments) == 0 {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "No submitted assignments found for these users"})
	}

	
	assignmentIDs := make([]string, 0)
	for _, submission := range submittedAssignments {
		assignmentIDs = append(assignmentIDs, submission.AssignmentID.String())
	}
	
	// Now, update the main Assignment records' status.
	if err := database.DB.Model(&models.SubmitAssignment{}).
		Where("user_id IN ?", studentIDs).
		Update("status", "accepted").Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error updating assignment status"})
	}
	
	// Fetch user info for email notification.
	var UserInfo []models.User
	if err := database.DB.Where("user_id IN ?", studentIDs).Find(&UserInfo).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error getting user info for emails"})
	}

	var emails []string
	for _, user := range UserInfo {
		emails = append(emails, user.Email)
	}

	req := &pb.AssignmentEmailRequest{
		Subject:    fmt.Sprintf("Assignment Accepted"),
		Body:       fmt.Sprintf("Your assignment has been Accepted"),
		Recipients: emails,
	}

	// This function is assumed to be defined elsewhere.
	SendAssignmentNotification(req)

	return c.JSON(fiber.Map{"message": "Assignments Accepted"})
}



func RejectAssignment(c *fiber.Ctx) error {
// Get comma-separated student IDs from form
	idList := c.Query("id")
	reason := c.Query("reason")
	if idList == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "No student IDs provided"})
	}

	idStrs := strings.Split(idList, ",")
	studentIDs := make([]string, 0, len(idStrs))
	for _, s := range idStrs {
		studentIDs = append(studentIDs, strings.TrimSpace(s))
	}
	
	// First, find the submitted assignments that match the user IDs.
	var submittedAssignments []models.SubmitAssignment
	if err := database.DB.Where("user_id IN ?", studentIDs).Find(&submittedAssignments).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error getting submitted assignments"})
	}

	if len(submittedAssignments) == 0 {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "No submitted assignments found for these users"})
	}

	
	assignmentIDs := make([]string, 0)
	for _, submission := range submittedAssignments {
		assignmentIDs = append(assignmentIDs, submission.AssignmentID.String())
	}
	
	// Now, update the main Assignment records' status.
	if err := database.DB.Model(&models.SubmitAssignment{}).
		Where("user_id IN ?", studentIDs).
		Update("status", "rejected").Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error updating assignment status"})
	}
	
	// Fetch user info for email notification.
	var UserInfo []models.User
	if err := database.DB.Where("user_id IN ?", studentIDs).Find(&UserInfo).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error getting user info for emails"})
	}

	var emails []string
	for _, user := range UserInfo {
		emails = append(emails, user.Email)
	}

	req := &pb.AssignmentEmailRequest{
		Subject:    fmt.Sprintf("Assignment Rejected"),
		Body:       fmt.Sprintf("Your assignment has been Rejected due to %s", reason),
		Recipients: emails,
	}

	// This function is assumed to be defined elsewhere.
	SendAssignmentNotification(req)

	return c.JSON(fiber.Map{"message": "Assignments Rejected"})
}

func GetUserAssignments(c *fiber.Ctx) error {
	requserID:=c.Params("user_id")
      
	if requserID==""{
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "No userID sent"})
	}
    
      userID,err:=uuid.Parse(requserID)

	  if err!=nil{
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Invalid userID"})
	  }
    

	var assignments []models.SubmitAssignment

	if err := database.DB.Where("user_id = ?", userID).Find(&assignments).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error fetching user assignments"})
	}
	return c.JSON(assignments)
}

func GetSubmittedAssignments(c *fiber.Ctx) error {
	assignmentId := c.Query("assignment_id")
	if assignmentId == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "No assignment ID provided"})
	}

	var submissions []models.SubmitAssignment
	if err := database.DB.Where("status = ?", "pending").Find(&submissions).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error fetching submissions"})
	}

	return c.JSON(submissions)
}
