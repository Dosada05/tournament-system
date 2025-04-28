package services

import (
	"errors"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
	"github.com/Dosada05/tournament-system/utils"
	"log"
)

type UserService struct {
	repo repositories.UserRepository
}

func NewUserService(repo repositories.UserRepository) *UserService {
	return &UserService{repo: repo}
}

// CreateUser создает пользователя с захешированным паролем.
func (s *UserService) CreateUser(user *models.User, password string) error {
	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return err
	}
	user.PasswordHash = hashedPassword
	user.Role = "player"
	return s.repo.Create(user)
}

// AuthenticateUser проверяет email/пароль и возвращает JWT.
func (s *UserService) AuthenticateUser(credentials models.Credentials) (string, error) {
	if credentials.Email == "" || credentials.Password == "" {
		return "", errors.New("email и пароль обязательны")
	}

	user, err := s.repo.GetByEmail(credentials.Email)
	if err != nil {
		log.Printf("Ошибка при получении пользователя: %v", err)
		return "", errors.New("неверные учетные данные")
	}

	if user == nil {
		log.Printf("Пользователь с email %s не найден", credentials.Email)
		return "", errors.New("неверные учетные данные")
	}

	if !utils.CheckPasswordHash(credentials.Password, user.PasswordHash) {
		log.Printf("Неверный пароль для пользователя %s", credentials.Email)
		return "", errors.New("неверные учетные данные")
	}

	token, err := utils.GenerateJWT(user)
	if err != nil {
		log.Printf("Ошибка создания JWT: %v", err)
		return "", errors.New("ошибка создания токена")
	}

	return token, nil
}

// GetUserByID возвращает пользователя по ID.
func (s *UserService) GetUserByID(id int) (*models.User, error) {
	return s.repo.GetByID(id)
}

// UpdateUser обновляет данные пользователя.
func (s *UserService) UpdateUser(id int, user *models.User) error {
	return s.repo.Update(id, user)
}

// DeleteUser удаляет пользователя.
func (s *UserService) DeleteUser(id int) error {
	return s.repo.Delete(id)
}
