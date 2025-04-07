package services

import (
	"errors"
	"github.com/Dosada05/tournament-system/config"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
	"github.com/Dosada05/tournament-system/utils"
	"github.com/dgrijalva/jwt-go"
	"time"
)

func CreateUser(user *models.User) error {
	return repositories.CreateUser(user)
}

func AuthenticateUser(credentials models.Credentials) (string, error) {
	user, err := repositories.GetUserByEmail(credentials.Email)
	if err != nil || !utils.CheckPasswordHash(credentials.Password, user.PasswordHash) {
		return "", errors.New("invalid credentials")
	}

	token, err := generateJWT(user)
	if err != nil {
		return "", err
	}

	return token, nil
}

func generateJWT(user *models.User) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":   user.ID,
		"role": user.Role,
		"exp":  time.Now().Add(time.Hour * 72).Unix(),
	})

	tokenString, err := token.SignedString([]byte(config.JWTSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
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
