package handlers

import (
	"errors"
	"fmt"

	"net/http"
	"time" // Необходимо для генерации токена

	_ "github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/services"
	"github.com/golang-jwt/jwt/v4" // Используем более новую библиотеку jwt/v4
)

type AuthHandler struct {
	authService services.AuthService
	jwtSecret   []byte // Добавляем секретный ключ как зависимость
}

// Обновляем конструктор, чтобы он принимал секретный ключ
func NewAuthHandler(auth services.AuthService, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		authService: auth,
		jwtSecret:   []byte(jwtSecret),
	}
}

// Register обрабатывает запрос на регистрацию нового пользователя.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	// Структура ввода соответствует RegisterInput из вашего сервиса
	var input services.RegisterInput

	err := readJSON(w, r, &input)
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	// Проверка входных данных (например, на пустоту) должна быть здесь или в сервисе
	if input.Email == "" || input.Password == "" || input.FirstName == "" {
		badRequestResponse(w, r, errors.New("first name, email, and password are required"))
		return
	}
	// TODO: Добавить более строгую валидацию (длина пароля, формат email)

	// Вызываем метод Register из AuthService
	user, err := h.authService.Register(r.Context(), input)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err) // Маппинг ошибок AuthService (включая ErrAuthEmailTaken)
		return
	}

	// Успешная регистрация. Возвращаем пользователя (сервис уже очистил PasswordHash)
	response := jsonResponse{
		"user": user,
	}

	err = writeJSON(w, http.StatusCreated, response, nil)
	if err != nil {
		serverErrorResponse(w, r, err)
	}
}

// Login обрабатывает запрос на вход пользователя и генерирует JWT.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// Структура ввода соответствует LoginInput из вашего сервиса
	var input services.LoginInput

	err := readJSON(w, r, &input)
	if err != nil {
		badRequestResponse(w, r, err)
		return
	}

	// Проверка на пустые поля
	if input.Email == "" || input.Password == "" {
		badRequestResponse(w, r, errors.New("email and password are required"))
		return
	}

	// Вызываем Login из AuthService, который вернет пользователя
	user, err := h.authService.Login(r.Context(), input)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err) // Обрабатываем ErrAuthInvalidCredentials
		return
	}

	// --- Генерация JWT токена ---
	// Создаем клеймы (claims)
	claims := jwt.MapClaims{
		"sub":  user.ID,                               // Subject (идентификатор пользователя)
		"role": user.Role,                             // Роль пользователя
		"name": user.Nickname,                         // Имя пользователя (или другое)
		"exp":  time.Now().Add(time.Hour * 72).Unix(), // Срок действия токена (например, 72 часа)
		"iat":  time.Now().Unix(),                     // Время создания токена
	}

	// Создаем токен с указанием алгоритма подписи и клеймов
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Подписываем токен секретным ключом
	tokenString, err := token.SignedString(h.jwtSecret)
	if err != nil {
		serverErrorResponse(w, r, fmt.Errorf("failed to sign token: %w", err))
		return
	}
	// --- Конец генерации JWT ---

	// Возвращаем токен
	response := jsonResponse{
		"token": tokenString,
	}

	err = writeJSON(w, http.StatusOK, response, nil)
	if err != nil {
		serverErrorResponse(w, r, err)
	}
}
