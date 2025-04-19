package services

import (
	"errors"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
	"github.com/Dosada05/tournament-system/utils"
	"log"
)

func CreateUser(user *models.User, password string) error {

	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return err
	}
	user.PasswordHash = hashedPassword
	user.Role = "player"
	return repositories.CreateUser(user)
}

func AuthenticateUser(credentials models.Credentials) (string, error) {
	// Проверка наличия обязательных полей
	if credentials.Email == "" || credentials.Password == "" {
		return "", errors.New("email и пароль обязательны")
	}

	// Получаем пользователя из БД
	user, err := repositories.GetUserByEmail(credentials.Email)
	if err != nil {
		log.Printf("Ошибка при получении пользователя: %v", err)
		return "", errors.New("неверные учетные данные")
	}

	// Проверяем, что user не nil
	if user == nil {
		log.Printf("Пользователь с email %s не найден", credentials.Email)
		return "", errors.New("неверные учетные данные")
	}

	// Проверяем пароль
	if !utils.CheckPasswordHash(credentials.Password, user.PasswordHash) {
		log.Printf("Неверный пароль для пользователя %s", credentials.Email)
		return "", errors.New("неверные учетные данные")
	}

	// Генерируем токен
	token, err := utils.GenerateJWT(user)
	if err != nil {
		log.Printf("Ошибка создания JWT: %v", err)
		return "", errors.New("ошибка создания токена")
	}

	return token, nil
}

func GetUserByID(id int) (*models.User, error) {
	return repositories.GetUserByID(id)
}

func UpdateUser(id int, user *models.User) error {
	return repositories.UpdateUser(id, user)
}

func DeleteUser(id int) error {
	return repositories.DeleteUser(id)
}
