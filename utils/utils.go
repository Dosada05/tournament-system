package utils

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/Dosada05/tournament-system/models"
	"github.com/dgrijalva/jwt-go"
	"golang.org/x/crypto/bcrypt"
)

const BcryptCost = 14

const userKey = "user"

var jwtSecret = []byte(getEnvOrDefault("JWT_SECRET", "TSSSSS"))

func GetJWTSecret() []byte {
	return jwtSecret
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func GenerateJWT(user *models.User) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"id":   user.ID,
		"role": user.Role,
		"exp":  now.Add(time.Hour * 24).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(jwtSecret)
}

// GetUserIDFromContext достаёт user_id из JWT claims в контексте запроса
func GetUserIDFromContext(ctx context.Context) (int, error) {
	claims, ok := ctx.Value(userKey).(jwt.MapClaims)
	if !ok {
		return 0, errors.New("user claims not found in context")
	}
	idRaw, ok := claims["user_id"]
	if !ok {
		return 0, errors.New("user_id not found in token")
	}

	idFloat, ok := idRaw.(float64)
	if !ok {
		return 0, errors.New("user_id has invalid type")
	}
	return int(idFloat), nil
}
