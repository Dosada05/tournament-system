package middleware

import (
	"context"
	"github.com/Dosada05/tournament-system/config"
	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/jwtauth"
	"net/http"
)

var tokenAuth *jwtauth.JWTAuth

func init() {
	tokenAuth = jwtauth.New("HS256", []byte(config.JWTSecret), nil)
}

func Authenticate(next http.Handler) http.Handler {
	return jwtauth.Verifier(tokenAuth)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, claims, err := jwtauth.FromContext(r.Context())
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "user", claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	}))
}

func Authorize(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userClaims := r.Context().Value("user").(jwt.MapClaims)
			userRole := userClaims["role"].(string)

			for _, role := range roles {
				if role == userRole {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, "Forbidden", http.StatusForbidden)
		})
	}
}
