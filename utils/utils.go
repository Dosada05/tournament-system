package utils

import (
	"os"
)

const BcryptCost = 14

const userKey = "user"

var jwtSecret = []byte(getEnvOrDefault("JWT_SECRET_KEY", "YERNUR"))

func GetJWTSecret() []byte {
	return jwtSecret
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func IsValidEmail(email string) bool {
	// Реализация проверки формата email (например, с помощью regexp)
	// Пример:
	// const emailRegex = `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	// re := regexp.MustCompile(emailRegex)
	// return re.MatchString(email)
	return true // Заглушка
}
