package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/jwtauth"

	"github.com/Dosada05/tournament-system/utils"
)

const (
	bearerPrefix = "Bearer "
	userKey      = "user"
)

var tokenAuth *jwtauth.JWTAuth

func init() {
	tokenAuth = jwtauth.New("HS256", utils.GetJWTSecret(), nil)
}

// Authenticate проверяет JWT-токен и добавляет информацию о пользователе в контекст
func Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := extractToken(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if token == "" {
			http.Error(w, "No token provided", http.StatusUnauthorized)
			return
		}

		// Проверка токена
		parsedToken, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
			if t.Method.Alg() != "HS256" {
				return nil, jwt.ErrSignatureInvalid
			}
			return utils.GetJWTSecret(), nil
		})

		if err != nil || !parsedToken.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Добавляем claims в контекст
		claims, ok := parsedToken.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "Invalid token claims", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Authorize проверяет роль пользователя
func Authorize(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userClaims, ok := r.Context().Value(userKey).(jwt.MapClaims)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			userRole, ok := userClaims["role"].(string)
			if !ok {
				http.Error(w, "Invalid token claims", http.StatusUnauthorized)
				return
			}

			// Проверка наличия необходимой роли
			authorized := false
			for _, role := range roles {
				if role == userRole {
					authorized = true
					break
				}
			}

			if !authorized {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractToken извлекает токен из заголовка Authorization
func extractToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", nil
	}

	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return "", jwt.ErrSignatureInvalid
	}

	return strings.TrimPrefix(authHeader, bearerPrefix), nil
}
