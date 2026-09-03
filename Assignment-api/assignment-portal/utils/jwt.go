package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

type JWTManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

type Claims struct {
	UserID string
	Role   string
}

func NewJWTManager(secret, issuer string, ttl time.Duration) *JWTManager {
	return &JWTManager{secret: []byte(secret), issuer: issuer, ttl: ttl}
}

func (m *JWTManager) Generate(userID string, role string) (string, error) {
	claims := jwt.MapClaims{
		"exp":    time.Now().Add(m.ttl).Unix(),
		"iss":    m.issuer,
		"userID": userID,
		"role":   role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *JWTManager) Parse(tokenString string) (Claims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return m.secret, nil
	})
	if err != nil || !token.Valid {
		return Claims{}, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, errors.New("invalid claims")
	}

	if claims["iss"] != m.issuer {
		return Claims{}, errors.New("invalid issuer")
	}

	userID, _ := claims["userID"].(string)
	role, _ := claims["role"].(string)
	if userID == "" || role == "" {
		return Claims{}, errors.New("missing claims")
	}

	return Claims{UserID: userID, Role: role}, nil
}

var defaultJWT = NewJWTManager("your-secret-key", "assignment-portal", 24*time.Hour)

func GenerateJWT(userID string, role string) (string, error) {
	return defaultJWT.Generate(userID, role)
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
