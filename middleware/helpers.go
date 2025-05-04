package middleware

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/Dosada05/tournament-system/models"
	"github.com/golang-jwt/jwt/v4"
)

// Оставляем определение contextKey здесь, если оно не в middleware.go
// type contextKey string
// const userContextKey contextKey = "user"

// Определяем константы для имен JWT claims
const (
	jwtClaimUserID = "user_id" // Используем user_id, как в логе Authenticate
	jwtClaimRole   = "role"
)

func GetUserIDFromContext(ctx context.Context) (int, error) {
	claims, ok := ctx.Value(userContextKey).(jwt.MapClaims)
	if !ok {
		// Эта ошибка теперь будет возникать реже, так как ключи совпадают
		return 0, errors.New("user claims not found in context or invalid type")
	}

	// Ищем claim по имени jwtClaimUserID ("user_id")
	userIDClaim, ok := claims[jwtClaimUserID]
	if !ok {
		return 0, fmt.Errorf("missing '%s' claim in token", jwtClaimUserID)
	}

	userIDFloat, ok := userIDClaim.(float64)
	if !ok {
		userIDStr, okStr := userIDClaim.(string)
		if okStr {
			userIDInt, err := strconv.Atoi(userIDStr)
			if err == nil {
				if userIDInt <= 0 {
					return 0, fmt.Errorf("invalid user ID value in '%s' claim: %d", jwtClaimUserID, userIDInt)
				}
				return userIDInt, nil
			}
		}
		return 0, fmt.Errorf("invalid type for '%s' claim: expected float64 or string, got %T", jwtClaimUserID, userIDClaim)
	}

	if userIDFloat != float64(int(userIDFloat)) {
		return 0, fmt.Errorf("'%s' claim is not an integer: %f", jwtClaimUserID, userIDFloat)
	}

	userID := int(userIDFloat)
	if userID <= 0 {
		return 0, fmt.Errorf("invalid user ID value in '%s' claim: %d", jwtClaimUserID, userID)
	}

	return userID, nil
}

func GetUserRoleFromContext(ctx context.Context) (models.UserRole, error) { // Используем models.Role
	claims, ok := ctx.Value(userContextKey).(jwt.MapClaims)
	if !ok {
		return "", errors.New("user claims not found in context or invalid type")
	}

	// Ищем claim по имени jwtClaimRole ("role")
	roleClaim, ok := claims[jwtClaimRole]
	if !ok {
		return "", fmt.Errorf("missing '%s' claim in token", jwtClaimRole)
	}

	roleStr, ok := roleClaim.(string)
	if !ok {
		return "", fmt.Errorf("invalid type for '%s' claim: expected string, got %T", jwtClaimRole, roleClaim)
	}

	role := models.UserRole(roleStr) // Используем models.Role

	switch role {
	case models.RoleAdmin, models.RoleOrganizer, models.RolePlayer:
		return role, nil
	default:
		return "", fmt.Errorf("invalid role value in claim: %q", roleStr)
	}
}
