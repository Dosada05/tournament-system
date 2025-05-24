package middleware

import (
	"context"
	"errors" // Добавлено для ошибки из helpers
	"log"
	"net/http"
	"strings"
	_ "time"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/utils"
	"github.com/golang-jwt/jwt/v4"
)

const (
	bearerPrefix = "Bearer "
)

type contextKey string

const userContextKey contextKey = "user"

func Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString, err := extractToken(r)
		if err != nil {
			log.Printf("Error extracting token: %v", err)
			http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}

		if tokenString == "" {
			log.Println("No token provided")
			http.Error(w, "Unauthorized: No token provided", http.StatusUnauthorized)
			return
		}

		parsedToken, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				log.Printf("Unexpected signing method: %v", t.Header["alg"])
				return nil, jwt.ErrSignatureInvalid
			}
			return utils.GetJWTSecret(), nil
		})

		if err != nil {
			log.Printf("Token parsing/validation error: %v", err)
			// Определяем, истек ли токен
			if errors.Is(err, jwt.ErrTokenExpired) {
				http.Error(w, "Unauthorized: Token expired", http.StatusUnauthorized)
			} else {
				http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
			}
			return
		}

		if !parsedToken.Valid {
			log.Println("Token is invalid (parsedToken.Valid is false)")
			http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
			return
		}

		claims, ok := parsedToken.Claims.(jwt.MapClaims)
		if !ok {
			log.Println("Invalid token claims type")
			http.Error(w, "Unauthorized: Invalid token claims", http.StatusUnauthorized)
			return
		}

		_, idOk := claims[jwtClaimUserID] // Используем константу для имени claim ID
		_, roleOk := claims[jwtClaimRole] // Используем константу для имени claim Role
		if !idOk || !roleOk {
			log.Printf("Missing required claims ('%s' or '%s') in token", jwtClaimUserID, jwtClaimRole)
			http.Error(w, "Unauthorized: Missing required token claims", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, claims)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func Authorize(roles ...models.UserRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Используем GetUserRoleFromContext для получения роли
			userRole, err := GetUserRoleFromContext(r.Context())
			if err != nil {
				log.Printf("Authorization failed: %v", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized) // Или Forbidden, если аутентификация прошла, но роль не извлечь
				return
			}

			authorized := false
			for _, allowedRole := range roles {
				if allowedRole == userRole {
					authorized = true
					break
				}
			}

			if !authorized {
				log.Printf("Authorization failed: User role '%s' not in allowed roles %v", userRole, roles)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			log.Printf("Authorization successful: User role '%s' is allowed.", userRole)
			next.ServeHTTP(w, r)
		})
	}
}

func extractToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", nil
	}

	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return "", errors.New("invalid authorization header format") // Возвращаем ошибку
	}

	return strings.TrimPrefix(authHeader, bearerPrefix), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
