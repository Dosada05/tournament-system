package handlers

import (
	"errors"
	"fmt"

	"net/http"
	"time"

	_ "github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/services"
	"github.com/golang-jwt/jwt/v4"
)

type AuthHandler struct {
	authService services.AuthService
	jwtSecret   []byte
}

func NewAuthHandler(auth services.AuthService, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		authService: auth,
		jwtSecret:   []byte(jwtSecret),
	}
}

// Register обрабатывает запрос на регистрацию нового пользователя.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {

	var input services.RegisterInput

	err := readJSON(w, r, &input)
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	if input.Email == "" || input.Password == "" || input.FirstName == "" {
		badRequestResponse(w, r, errors.New("first name, email, and password are required"))
		return
	}

	user, err := h.authService.Register(r.Context(), input)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	response := jsonResponse{
		"user": user,
	}

	err = writeJSON(w, http.StatusCreated, response, nil)
	if err != nil {
		serverErrorResponse(w, r, err)
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input services.LoginInput

	err := readJSON(w, r, &input)
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	if input.Email == "" || input.Password == "" {
		badRequestResponse(w, r, errors.New("email and password are required"))
		return
	}

	user, err := h.authService.Login(r.Context(), input)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	claims := jwt.MapClaims{
		"user_id": user.ID,                               // Subject (идентификатор пользователя)
		"role":    user.Role,                             // Роль пользователя
		"name":    user.Nickname,                         // Имя пользователя (или другое)
		"exp":     time.Now().Add(time.Hour * 72).Unix(), // Срок действия токена (например, 72 часа)
		"iat":     time.Now().Unix(),                     // Время создания токена
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(h.jwtSecret)
	if err != nil {
		serverErrorResponse(w, r, fmt.Errorf("failed to sign token: %w", err))
		return
	}

	response := jsonResponse{
		"token": tokenString,
	}

	err = writeJSON(w, http.StatusOK, response, nil)
	if err != nil {
		serverErrorResponse(w, r, err)
	}
}
