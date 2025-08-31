package middleware

import (
    "fmt"
    "log"
    "strings"

    "github.com/gofiber/fiber/v2"
    "github.com/golang-jwt/jwt/v4"
)

var secretKey = []byte("your-secret-key")

func AuthMiddleware() fiber.Handler {
    return func(c *fiber.Ctx) error {
        tokenHeader := c.Get("Authorization")
        if tokenHeader == "" || !strings.HasPrefix(tokenHeader, "Bearer ") {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "Unauthorized",
            })
        }

        tokenString := strings.TrimPrefix(tokenHeader, "Bearer ")

        token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
            if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid signing method")
            }
            return secretKey, nil
        })

        if err != nil || !token.Valid {
            log.Println("Token validation error:", err)
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
        }

        claims := token.Claims.(jwt.MapClaims)
        
        // Correct the key to match what is set in utils/jwt.go
        userID := claims["userID"]
        role := claims["role"]

        // Log the correct values
        fmt.Println("Authenticated user:", userID, "with role:", role)

        c.Locals("userID", userID)
        c.Locals("role", role)

        return c.Next()
    }
}

func AdminOnly(handler fiber.Handler) fiber.Handler {
    return func(c *fiber.Ctx) error {
        if role := c.Locals("role"); role != "admin" {
            return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden: Admin access required"})
        }
        return handler(c)
    }
}

func UserOnly(handler fiber.Handler) fiber.Handler {
    return func(c *fiber.Ctx) error {
        if role := c.Locals("role"); role != "user" && role != "admin" {
            return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden: User access required"})
        }
        return handler(c)
    }
}