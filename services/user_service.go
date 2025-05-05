package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	_ "golang.org/x/crypto/bcrypt"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
	"github.com/Dosada05/tournament-system/utils"
)

// --- Константы уровня пакета ---
const (
	minPasswordLength = 8 // Минимальная длина пароля
)

// --- Ошибки уровня пакета ---
var (
	ErrUserUpdateFailed      = errors.New("failed to update user profile")
	ErrNicknameTaken         = errors.New("nickname is already taken")
	ErrEmailTaken            = errors.New("email is already taken")
	ErrInvalidEmailFormat    = errors.New("invalid email format")
	ErrPasswordHashingFailed = errors.New("failed to hash password")
)

// --- Интерфейс Сервиса ---
type UserService interface {
	GetProfileByID(ctx context.Context, userID int) (*models.User, error)
	UpdateProfile(ctx context.Context, userID int, input UpdateProfileInput) (*models.User, error)
	ListUsersByTeamID(ctx context.Context, teamID int) ([]models.User, error)
}

// --- Структура Ввода (DTO) ---
type UpdateProfileInput struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Nickname  *string `json:"nickname"`
	Email     *string `json:"email"`
	Password  *string `json:"password"`
}

type userService struct {
	userRepo repositories.UserRepository
}

func NewUserService(userRepo repositories.UserRepository) UserService {
	return &userService{
		userRepo: userRepo,
	}
}

func (s *userService) GetProfileByID(ctx context.Context, userID int) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by id %d: %w", userID, err)
	}

	user.PasswordHash = "" // Очищаем хеш перед возвратом
	return user, nil
}

func (s *userService) UpdateProfile(ctx context.Context, userID int, input UpdateProfileInput) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user for update %d: %w", userID, err)
	}

	updated := false // Флаг, указывающий, были ли изменения

	// Обновление FirstName
	if input.FirstName != nil {
		trimmedFirstName := strings.TrimSpace(*input.FirstName)
		// Доп. проверка: запретить пустое имя после TrimSpace?
		// if trimmedFirstName == "" { return nil, errors.New("first name cannot be empty") }
		if trimmedFirstName != user.FirstName {
			user.FirstName = trimmedFirstName
			updated = true
		}
	}

	// Обновление LastName
	if input.LastName != nil {
		trimmedLastName := strings.TrimSpace(*input.LastName)
		// Доп. проверка: запретить пустую фамилию после TrimSpace?
		// if trimmedLastName == "" { return nil, errors.New("last name cannot be empty") }
		if trimmedLastName != user.LastName {
			user.LastName = trimmedLastName
			updated = true
		}
	}

	// Обновление Nickname
	if input.Nickname != nil {
		trimmedNickname := strings.TrimSpace(*input.Nickname)
		currentNickname := ""
		if user.Nickname != nil {
			currentNickname = *user.Nickname
		}
		if trimmedNickname != currentNickname {
			if trimmedNickname == "" {
				user.Nickname = nil // Сброс никнейма
			} else {
				// Проверка длины/формата никнейма, если нужно
				user.Nickname = &trimmedNickname
			}
			updated = true
		}
	}

	// Обновление Email
	if input.Email != nil {
		newEmail := strings.ToLower(strings.TrimSpace(*input.Email)) // Приводим к нижнему регистру
		if newEmail == "" {
			return nil, ErrInvalidEmailFormat // Пустой email недопустим
		}
		if !utils.IsValidEmail(newEmail) { // Валидация формата
			return nil, ErrInvalidEmailFormat
		}
		if newEmail != user.Email {
			user.Email = newEmail
			updated = true
		}
	}

	// Обновление Password
	if input.Password != nil {
		newPassword := *input.Password            // Пароль не триммим
		if len(newPassword) < minPasswordLength { // Проверка длины
			return nil, ErrPasswordTooShort
		}

		newPasswordHash, err := utils.HashPassword(newPassword) // Хешируем
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrPasswordHashingFailed, err)
		}
		// Обновляем хеш, если пароль был предоставлен и прошел валидацию
		user.PasswordHash = newPasswordHash
		updated = true
	}

	// Если не было никаких обновлений, просто возвращаем профиль (без хеша)
	if !updated {
		user.PasswordHash = ""
		return user, nil
	}

	// Вызываем репозиторий для сохранения изменений
	err = s.userRepo.Update(ctx, user)
	if err != nil {
		// Обрабатываем ошибки конфликтов и другие ошибки БД
		if errors.Is(err, repositories.ErrUserNicknameConflict) {
			return nil, ErrNicknameTaken
		}
		if errors.Is(err, repositories.ErrUserEmailConflict) {
			return nil, ErrEmailTaken
		}
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrUserNotFound // Пользователь удален?
		}
		return nil, fmt.Errorf("%w: %w", ErrUserUpdateFailed, err)
	}

	user.PasswordHash = "" // Очищаем хеш перед возвратом клиенту
	return user, nil
}

func (s *userService) ListUsersByTeamID(ctx context.Context, teamID int) ([]models.User, error) {
	if teamID <= 0 {
		return nil, errors.New("invalid team ID")
	}

	users, err := s.userRepo.ListByTeamID(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to list users by team id from repository: %w", err)
	}

	for i := range users {
		users[i].PasswordHash = "" // Убираем хеши
	}

	return users, nil
}
