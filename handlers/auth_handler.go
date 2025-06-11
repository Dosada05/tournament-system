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
	authService  services.AuthService
	emailService *services.EmailService
	jwtSecret    []byte
}

func NewAuthHandler(authService services.AuthService, emailService *services.EmailService, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		authService:  authService,
		emailService: emailService,
		jwtSecret:    []byte(jwtSecret),
	}
}

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

	user, confirmationToken, err := h.authService.Register(r.Context(), input)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	if err := h.emailService.SendWelcomeEmail(user.Email, confirmationToken); err != nil {
		fmt.Println("Ошибка отправки email:", err)
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
		"user_id": user.ID,
		"role":    user.Role,
		"name":    user.Nickname,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
		"iat":     time.Now().Unix(),
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

func (h *AuthHandler) ConfirmEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		badRequestResponse(w, r, errors.New("confirmation token is required"))
		return
	}

	err := h.authService.ConfirmEmail(r.Context(), token)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Email успешно подтвержден!"))
}

// Запрос на сброс пароля
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email string `json:"email"`
	}
	if err := readJSON(w, r, &input); err != nil {
		badRequestResponse(w, r, err)
		return
	}
	if input.Email == "" {
		badRequestResponse(w, r, errors.New("email is required"))
		return
	}
	resetToken, err := h.authService.GeneratePasswordResetToken(r.Context(), input.Email)
	if err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}
	if err := h.emailService.SendPasswordResetEmail(input.Email, resetToken); err != nil {
		fmt.Println("Ошибка отправки email для сброса пароля:", err)
	}
	response := map[string]string{"message": "Если email зарегистрирован, ссылка для сброса отправлена"}
	if err := writeJSON(w, http.StatusOK, response, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}

// Сброс пароля по токену
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := readJSON(w, r, &input); err != nil {
		badRequestResponse(w, r, err)
		return
	}
	if input.Token == "" || input.NewPassword == "" {
		badRequestResponse(w, r, errors.New("token and new_password are required"))
		return
	}
	if err := h.authService.ResetPasswordByToken(r.Context(), input.Token, input.NewPassword); err != nil {
		mapServiceErrorToHTTP(w, r, err)
		return
	}
	response := map[string]string{"message": "Пароль успешно изменён"}
	if err := writeJSON(w, http.StatusOK, response, nil); err != nil {
		serverErrorResponse(w, r, err)
	}
}
