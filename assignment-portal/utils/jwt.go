package utils

import (
	"log"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

var secretKey = []byte("your-secret-key") // Replace with a secure secret key

func GenerateJWT(userID int, role string) (string, error) {
	claims := jwt.MapClaims{
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
		"iss":  "assignment-portal",
		"sub":  userID,
		"role": role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := token.SignedString(secretKey)
	if err != nil {
		log.Println("Error generating JWT:", err)
		return "", err
	}

	return signedToken, nil
}

func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

func CheckPassword(hashedPassword, plainPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
}
