package controllers

import (
    "github.com/gofiber/fiber/v2"
    "github.com/lokesh2201013/email-service/database"
    "github.com/lokesh2201013/email-service/models"
    "github.com/lokesh2201013/email-service/middleware"
    "golang.org/x/crypto/bcrypt"
    "fmt"
)

func Register(c *fiber.Ctx) error {
    var user models.User
    if err := c.BodyParser(&user); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
    }

    hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
    user.Password = string(hashedPassword)

   
    if err := database.DB.Create(&user).Error; err != nil {
        return c.Status(500).JSON(fiber.Map{"error": "Could not register user"})
    }

    user.SandboxTime = user.CreatedAt.AddDate(0, 0, 7)


    if err := database.DB.Save(&user).Error; err != nil {
        return c.Status(500).JSON(fiber.Map{"error": "Could not update user with SandboxTime"})
    }

    return c.JSON(fiber.Map{
        "message":       "User registered successfully and Analytics created",
        "Created At":    user.CreatedAt,
        "SandboxMessage": fmt.Sprintf("Newly registered user will be added to the sandbox and will be in it till %s", user.SandboxTime),
    })
}

func Login(c *fiber.Ctx) error {
    var user models.User
    var storedUser models.User

    if err := c.BodyParser(&user); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
    }

    if err := database.DB.Where("username = ?", user.Username).First(&storedUser).Error; err != nil {
        return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
    }

    if err := bcrypt.CompareHashAndPassword([]byte(storedUser.Password), []byte(user.Password)); err != nil {
        return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
    }

    token, _ := middleware.GenerateToken(user.Username)

    return c.JSON(fiber.Map{"token": token})
}
