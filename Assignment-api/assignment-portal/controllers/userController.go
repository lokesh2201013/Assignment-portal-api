package controllers

import (
	"errors"
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

var (
	errLoginUserNotFound = errors.New("user not found")
	errTenantRequired    = errors.New("tenant_id is required when email belongs to multiple tenants")
)

func resolveLoginUser(users []models.User, tenantID string) (models.User, error) {
	if tenantID != "" {
		for _, user := range users {
			if loginTenantID(user) == tenantID {
				return user, nil
			}
		}
		return models.User{}, errLoginUserNotFound
	}
	switch len(users) {
	case 0:
		return models.User{}, errLoginUserNotFound
	case 1:
		return users[0], nil
	default:
		return models.User{}, errTenantRequired
	}
}

func loginTenantID(user models.User) string {
	if strings.TrimSpace(user.TenantID) == "" {
		return "default"
	}
	return user.TenantID
}

func generateLoginToken(user models.User) (string, error) {
	return utils.GenerateForTenant(user.UserID.String(), user.Role, loginTenantID(user))
}

// Global regex for email validation
var emailRegex = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)

func Register(c *fiber.Ctx) error {
	var user models.User
	if err := c.BodyParser(&user); err != nil {
		log.Println("Error parsing request body:", err)
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}
	user.TenantID = "default"

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
		TenantID string `json:"tenant_id"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&loginData); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid input"})
	}
	loginData.TenantID = strings.TrimSpace(loginData.TenantID)

	var users []models.User
	query := database.DB.Where("email = ?", loginData.Email)
	if loginData.TenantID != "" {
		if loginData.TenantID == "default" {
			query = query.Where("tenant_id = ? OR tenant_id IS NULL OR tenant_id = ''", loginData.TenantID)
		} else {
			query = query.Where("tenant_id = ?", loginData.TenantID)
		}
	}
	if err := query.Find(&users).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Could not retrieve user"})
	}
	user, err := resolveLoginUser(users, loginData.TenantID)
	if errors.Is(err, errTenantRequired) {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if err := utils.CheckPassword(user.Password, loginData.Password); err != nil {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid credentials"})
	}
	token, err := generateLoginToken(user)
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
