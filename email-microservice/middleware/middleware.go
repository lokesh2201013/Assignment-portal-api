package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte("secret")

func GenerateToken(username string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "username": username,
        "exp":      time.Now().Add(time.Hour * 24).Unix(),
    })

    return token.SignedString(jwtSecret)
}

func AuthRequired(c *fiber.Ctx) error {
    tokenString := c.Get("Authorization")
    
    if tokenString == "" {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Missing authorization header"})
    }

    if len(tokenString) <= 7 || tokenString[:7] != "Bearer " {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid authorization header format"})
    }

    tokenString = tokenString[7:]

    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        return jwtSecret, nil
    })

    if err != nil || !token.Valid {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or expired token"})
    }

    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token claims"})
    }

    username := claims["username"].(string)
    c.Locals("admin", username)  // Storeing the admin username in context for later use for db querying

    return c.Next()
}
