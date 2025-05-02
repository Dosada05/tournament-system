package services

import (
	"context"
	"errors"
	"fmt" // Используем для оборачивания ошибок

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrAuthInvalidCredentials = errors.New("invalid email or password")
	ErrAuthEmailTaken         = errors.New("email is already taken")
	// Другие специфичные для сервиса ошибки могут быть добавлены здесь
)

// AuthService определяет интерфейс для аутентификации пользователей.
type AuthService interface {
	Register(ctx context.Context, input RegisterInput) (*models.User, error)
	Login(ctx context.Context, input LoginInput) (*models.User, error)
}

// RegisterInput определяет данные, необходимые для регистрации.
type RegisterInput struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

// LoginInput определяет данные, необходимые для входа.
type LoginInput struct {
	Email    string
	Password string
}

// authService реализует AuthService.
type authService struct {
	userRepo repositories.UserRepository
	// passwordHasher // Можно вынести хешер в отдельный интерфейс/структуру для большей гибкости
}

// NewAuthService создает новый экземпляр AuthService.
func NewAuthService(userRepo repositories.UserRepository) AuthService {
	return &authService{
		userRepo: userRepo,
	}
}

// Register регистрирует нового пользователя.
func (s *authService) Register(ctx context.Context, input RegisterInput) (*models.User, error) {
	// Валидация входных данных (проверка на пустоту, формат email и т.д.)
	// обычно выполняется в слое выше (handler) или здесь, если правила сложные.
	// Для простоты пока опустим.

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		// Логирование ошибки здесь может быть полезно
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		Email:        input.Email,
		PasswordHash: string(hashedPassword),
		Role:         models.RolePlayer,
	}

	err = s.userRepo.Create(ctx, user)
	if err != nil {
		if errors.Is(err, repositories.ErrUserEmailConflict) {
			return nil, ErrAuthEmailTaken // Преобразуем ошибку репозитория в ошибку сервиса
		}
		// Логирование других ошибок репозитория
		return nil, fmt.Errorf("failed to register user: %w", err)
	}

	// Важно: Не возвращаем хеш пароля наружу после регистрации.
	// Создаем копию или обнуляем поле перед возвратом.
	user.PasswordHash = "" // Очищаем для безопасности

	return user, nil
}

// Login аутентифицирует пользователя по email и паролю.
func (s *authService) Login(ctx context.Context, input LoginInput) (*models.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, input.Email)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrAuthInvalidCredentials // Не раскрываем, что именно не так (email или пароль)
		}
		// Логирование других ошибок репозитория
		return nil, fmt.Errorf("failed to find user by email: %w", err)
	}

	// Сравниваем предоставленный пароль с хешем из базы данных.
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password))
	if err != nil {
		// Если ошибка - это bcrypt.ErrMismatchedHashAndPassword, значит пароль неверный.
		// Любая другая ошибка указывает на проблему с хешем или самим процессом сравнения.
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return nil, ErrAuthInvalidCredentials
		}
		// Логирование неожиданной ошибки сравнения
		return nil, fmt.Errorf("failed to compare password hash: %w", err)
	}

	// Успешный вход. Снова очищаем хеш пароля перед возвратом.
	user.PasswordHash = ""

	return user, nil
}
