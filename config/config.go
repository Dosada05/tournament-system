package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL  string
	JWTSecretKey string
	ServerPort   int
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	jwtKey := os.Getenv("JWT_SECRET_KEY")
	if jwtKey == "" {
		return nil, fmt.Errorf("JWT_SECRET_KEY environment variable is not set")
	}

	portStr := os.Getenv("SERVER_PORT")
	if portStr == "" {
		portStr = "8080"
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
