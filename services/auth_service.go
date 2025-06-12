package services

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrAuthInvalidCredentials = errors.New("invalid email or password")
	ErrAuthEmailTaken         = errors.New("email is already taken")
)

type AuthService interface {
	Register(ctx context.Context, input RegisterInput) (*models.User, string, error)
	Login(ctx context.Context, input LoginInput) (*models.User, error)
	ConfirmEmail(ctx context.Context, token string) error
	GeneratePasswordResetToken(ctx context.Context, email string) (string, error)
	ResetPasswordByToken(ctx context.Context, token string, newPassword string) error
}

type RegisterInput struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

type LoginInput struct {
	Email    string
	Password string
}

type authService struct {
	userRepo repositories.UserRepository
}

func NewAuthService(userRepo repositories.UserRepository) AuthService {
	return &authService{
		userRepo: userRepo,
	}
}

func (s *authService) Register(ctx context.Context, input RegisterInput) (*models.User, string, error) {

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("ошибка хеширования пароля: %w", err)
	}

	confirmationToken := generateRandomToken(32)

	user := &models.User{
		FirstName:              input.FirstName,
		LastName:               input.LastName,
		Email:                  input.Email,
		PasswordHash:           string(hashedPassword),
		Role:                   models.RolePlayer,
		EmailConfirmed:         false,
		EmailConfirmationToken: confirmationToken,
	}

	err = s.userRepo.Create(ctx, user)
	if err != nil {
		if errors.Is(err, ErrAuthEmailTaken) {
			return nil, "", ErrAuthEmailTaken
		}
		return nil, "", fmt.Errorf("ошибка создания пользователя: %w", err)
	}
	return user, confirmationToken, nil
}

func (s *authService) Login(ctx context.Context, input LoginInput) (*models.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, input.Email)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrAuthInvalidCredentials
		}
		return nil, fmt.Errorf("failed to find user by email: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return nil, ErrAuthInvalidCredentials
		}
		return nil, fmt.Errorf("failed to compare password hash: %w", err)
	}

	user.PasswordHash = ""

	return user, nil
}

func (s *authService) ConfirmEmail(ctx context.Context, token string) error {
	user, err := s.userRepo.GetByConfirmationToken(ctx, token)
	if err != nil {
		return fmt.Errorf("invalid or expired confirmation token: %w", err)
	}
	if user.EmailConfirmed {
		return fmt.Errorf("email already confirmed")
	}
	if err := s.userRepo.ConfirmEmail(ctx, user.ID); err != nil {
		return fmt.Errorf("failed to confirm email: %w", err)
	}
	return nil
}

func (s *authService) GeneratePasswordResetToken(ctx context.Context, email string) (string, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		// Не раскрываем, зарегистрирован ли email
		return "", nil
	}
	resetToken := generateRandomToken(32)
	// Сохраняем токен и время его создания в БД
	err = s.userRepo.SetPasswordResetToken(ctx, user.ID, resetToken, time.Now().Add(1*time.Hour))
	if err != nil {
		return "", err
	}
	return resetToken, nil
}

func (s *authService) ResetPasswordByToken(ctx context.Context, token string, newPassword string) error {
	user, err := s.userRepo.GetByPasswordResetToken(ctx, token)
	if err != nil {
		log.Printf("Error getting user by password reset token: %v", err)
		return errors.New("invalid or expired token")
	}
	if user.PasswordResetExpiresAt == nil || user.PasswordResetExpiresAt.Before(time.Now()) {
		return errors.New("token expired")
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("ошибка хеширования пароля: %w", err)
	}
	user.PasswordHash = string(hashedPassword)
	user.PasswordResetToken = nil
	user.PasswordResetExpiresAt = nil
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("ошибка обновления пользователя: %w", err)
	}
	return nil
}

func generateRandomToken(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)
	if err != nil {
		for i := range b {
			b[i] = charset[int(time.Now().UnixNano())%len(charset)]
		}
		return string(b)
	}
	for i, rb := range randomBytes {
		b[i] = charset[int(rb)%len(charset)]
	}
	return string(b)
}
