package controllers

import (
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/lokesh2201013/database"
	"github.com/lokesh2201013/models"
	"github.com/lokesh2201013/utils"
)

// Global regex for email validation
var emailRegex = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)

func Register(c *fiber.Ctx) error {
	var user models.User
	if err := c.BodyParser(&user); err != nil {
		log.Println("Error parsing request body:", err)
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Log received email and role for debugging
	log.Printf("Attempting to register user with email: %s and role: %s", user.Email, user.Role)

	// Email validation
	if !emailRegex.MatchString(user.Email) {
		log.Println("Validation failed: Invalid email format")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid email format"})
	}

	var existingUser models.User
	if err := database.DB.Where("email=?", user.Email).First(&existingUser).Error; err == nil {
		log.Println("Validation failed: Email already in use")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Email already in use"})
	}

	if user.Role == "user" {
		log.Println("User role is 'user', performing specific validations...")
		
		// Branch validation
		if user.Branch == "" || len(user.Branch) > 5 {
			log.Printf("Validation failed: Invalid branch. Received '%s'", user.Branch)
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid branch. Must be 1-5 characters long."})
		}
		
		user.Branch = strings.ToUpper(user.Branch)
		log.Printf("Branch converted to uppercase: %s", user.Branch)

		// Semester validation
		if user.Semester < 1 || user.Semester > 20 {
			log.Printf("Validation failed: Invalid semester. Received %d", user.Semester)
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid semester. Must be between 1 and 20."})
		}
	}

	hashpassword, err := utils.HashPassword(user.Password)
	if err != nil {
		log.Println("Password hashing error:", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Internal server error"})
	}
	user.Password = hashpassword
	user.UserID = uuid.New()
	log.Printf("Hashed password and generated UserID: %s", user.UserID)

	if err := database.DB.Create(&user).Error; err != nil {
		log.Println("Database error on user creation:", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Internal server error"})
	}

	log.Println("User registered successfully.")
	return c.Status(http.StatusCreated).JSON(user)
}

func Login(c *fiber.Ctx) error {
	var loginData struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&loginData); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid input"})
	}

	var user models.User
	if err := database.DB.Where("email=?", loginData.Email).First(&user).Error; err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if err := utils.CheckPassword(user.Password, loginData.Password); err != nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid credentials"})
	}
    userId:=user.UserID.String()
	token, err := utils.GenerateJWT(userId, user.Role)
	if err != nil {
		log.Println("Token generation error:", err)
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Internal server error"})
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"message": "Login successful",
		"token":   token,
	})
}

func GetAllAdmins(c *fiber.Ctx) error {
	var admins []models.User
	if err := database.DB.Where("role = ?", "admin").Find(&admins).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Could not retrieve admins"})
	}
	return c.Status(http.StatusOK).JSON(admins)
}