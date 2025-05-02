package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config хранит все конфигурационные параметры приложения.
type Config struct {
	DatabaseURL  string
	JWTSecretKey string
	ServerPort   int
}

// Load загружает конфигурацию из переменных окружения.
// Опционально подгружает .env файл (полезно для локальной разработки).
func Load() (*Config, error) {
	// Загружаем .env файл, если он есть. Ошибку не считаем фатальной.
	_ = godotenv.Load() // Можно добавить логирование, если файл не найден, но не падать

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Можно установить значение по умолчанию или вернуть ошибку
		return nil, fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	jwtKey := os.Getenv("JWT_SECRET_KEY")
	if jwtKey == "" {
		// Можно установить значение по умолчанию (НЕБЕЗОПАСНО для JWT!) или вернуть ошибку
		return nil, fmt.Errorf("JWT_SECRET_KEY environment variable is not set")
	}

	portStr := os.Getenv("SERVER_PORT")
	if portStr == "" {
		portStr = "8080" // Порт по умолчанию
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SERVER_PORT environment variable: %w", err)
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("SERVER_PORT must be between 1 and 65535, got %d", port)
	}

	cfg := &Config{
		DatabaseURL:  dbURL,
		JWTSecretKey: jwtKey,
		ServerPort:   port,
	}

	return cfg, nil
}
