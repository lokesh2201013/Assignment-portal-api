package controllers

import (
	"net/http"
	"github.com/gofiber/fiber/v2"
	"github.com/lokesh2201013/assignment-portal/database"
	"github.com/lokesh2201013/assignment-portal/models"
	"strconv"
	"strings"
	"fmt"
	pb "github.com/lokesh2201013/assignment-portal/proto"
)

func GetAdminAssignments(c *fiber.Ctx) error {
	var assignments []models.Assignment

	branch := c.Query("branch")
	semester := c.Query("semester")

	query := database.DB.Model(&models.Assignment{})
	if branch != "" {
		query = query.Where("branch = ?", branch)
	}
	if semester != "" {
		query = query.Where("semester = ?", semester)
	}

	if err := query.Find(&assignments).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error fetching assignments"})
	}
	return c.JSON(assignments)
}

func AcceptAssignment(c *fiber.Ctx) error {
// Get comma-separated student IDs from form
	idList := c.Query("id") // e.g., "101,102,103"
   // reason:=c.Query("reason")

	if idList == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "No student IDs provided"})
	}

	// Split and parse to integers
	var UserInfo []models.User
	idStrs := strings.Split(idList, ",")
	studentIDs := make([]int, 0, len(idStrs))
	for _, s := range idStrs {
		id, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid student ID: " + s})
		}
		studentIDs = append(studentIDs, id)
	}
    if err:=database.DB.Where("user_id IN ?",studentIDs).Find(&UserInfo).Error;err!=nil{
		return  c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error updating email status"})
	}
	// Update all assignments for those student IDs
	if err := database.DB.Model(&models.Assignment{}).
		Where("user_id ?", studentIDs).
		Update("status", "rejected").Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error updating assignment status"})
	}

	var emails []string
	for _,user:=range UserInfo{
		emails=append(emails,user.Email)
	}
    
	req := &pb.AssignmentEmailRequest{
		Subject:    fmt.Sprintf("You have a task assigned by Admin"),
		Body:       fmt.Sprintf("Your assignment has been rejected a new task due to \nReason: "),
		Recipients: emails,
	}

	sendAssignmentNotification(req)

	return c.JSON(fiber.Map{"message": "Assignments Rejected"})
}


func RejectAssignment(c *fiber.Ctx) error {
// Get comma-separated student IDs from form
	idList := c.Query("id") // e.g., "101,102,103"
    reason:=c.Query("reason")

	if idList == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "No student IDs provided"})
	}

	// Split and parse to integers
	var UserInfo []models.User
	idStrs := strings.Split(idList, ",")
	studentIDs := make([]int, 0, len(idStrs))
	for _, s := range idStrs {
		id, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid student ID: " + s})
		}
		studentIDs = append(studentIDs, id)
	}
    if err:=database.DB.Where("user_id IN ?",studentIDs).Find(&UserInfo).Error;err!=nil{
		return  c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error updating email status"})
	}
	// Update all assignments for those student IDs
	if err := database.DB.Model(&models.Assignment{}).
		Where("user_id ?", studentIDs).
		Update("status", "rejected").Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error updating assignment status"})
	}

	var emails []string
	for _,user:=range UserInfo{
		emails=append(emails,user.Email)
	}
    
	req := &pb.AssignmentEmailRequest{
		Subject:    fmt.Sprintf("You have a task assigned by Admin"),
		Body:       fmt.Sprintf("Your assignment has been rejected a new task due to \nReason: %s",reason),
		Recipients: emails,
	}

	sendAssignmentNotification(req)

	return c.JSON(fiber.Map{"message": "Assignments Rejected"})
}

func GetUserAssignments(c *fiber.Ctx) error {
	requserID:=c.Query("user_id")
      
	if requserID==""{
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "No userID sent"})
	}
    
      userID,err:=strconv.Atoi(requserID)

	  if err!=nil{
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Invalid userID"})
	  }
    

	var assignments []models.Assignment

	if err := database.DB.Where("user_id = ?", userID).Find(&assignments).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Error fetching user assignments"})
	}
	return c.JSON(assignments)
}