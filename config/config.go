package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL       string
	JWTSecretKey      string
	ServerPort        int
	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2BucketName      string
	R2PublicBaseURL   string
	PublicURL         string

	SMTPHost string
	SMTPPort int
	SMTPUser string
	SMTPPass string
	SMTPFrom string
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

	r2AccountID := os.Getenv("R2_ACCOUNT_ID")
	if r2AccountID == "" {
		return nil, fmt.Errorf("R2_ACCOUNT_ID environment variable is not set")
	}
	r2AccessKeyID := os.Getenv("R2_ACCESS_KEY_ID")
	if r2AccessKeyID == "" {
		return nil, fmt.Errorf("R2_ACCESS_KEY_ID environment variable is not set")
	}
	r2SecretAccessKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	if r2SecretAccessKey == "" {
		return nil, fmt.Errorf("R2_SECRET_ACCESS_KEY environment variable is not set")
	}
	r2BucketName := os.Getenv("R2_BUCKET_NAME")
	if r2BucketName == "" {
		return nil, fmt.Errorf("R2_BUCKET_NAME environment variable is not set")
	}
	r2PublicBaseURL := os.Getenv("R2_PUBLIC_BASE_URL")
	if r2PublicBaseURL == "" {
		return nil, fmt.Errorf("R2_PUBLIC_BASE_URL environment variable is not set")
	}

	publicURL := os.Getenv("PUBLIC_URL")
	if publicURL == "" {
		return nil, fmt.Errorf("PUBLIC_URL environment variable is not set")
	}

	// SMTP config
	smtpHost := os.Getenv("SMTP_HOST")
	if smtpHost == "" {
		return nil, fmt.Errorf("SMTP_HOST environment variable is not set")
	}
	smtpPortStr := os.Getenv("SMTP_PORT")
	if smtpPortStr == "" {
		smtpPortStr = "587"
	}
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP_PORT environment variable: %w", err)
	}
	if smtpPort <= 0 || smtpPort > 65535 {
		return nil, fmt.Errorf("SMTP_PORT must be between 1 and 65535, got %d", smtpPort)
	}
	smtpUser := os.Getenv("SMTP_USER")
	if smtpUser == "" {
		return nil, fmt.Errorf("SMTP_USER environment variable is not set")
	}
	smtpPass := os.Getenv("SMTP_PASS")
	if smtpPass == "" {
		return nil, fmt.Errorf("SMTP_PASS environment variable is not set")
	}
	smtpFrom := os.Getenv("SMTP_FROM")
	if smtpFrom == "" {
		return nil, fmt.Errorf("SMTP_FROM environment variable is not set")
	}

	cfg := &Config{
		DatabaseURL:       dbURL,
		JWTSecretKey:      jwtKey,
		ServerPort:        port,
		R2AccountID:       r2AccountID,
		R2AccessKeyID:     r2AccessKeyID,
		R2SecretAccessKey: r2SecretAccessKey,
		R2BucketName:      r2BucketName,
		R2PublicBaseURL:   r2PublicBaseURL,
		PublicURL:         publicURL,

		SMTPHost: smtpHost,
		SMTPPort: smtpPort,
		SMTPUser: smtpUser,
		SMTPPass: smtpPass,
		SMTPFrom: smtpFrom,
	}

	return cfg, nil
}
