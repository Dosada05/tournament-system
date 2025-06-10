package services

import (
	"context"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

type AdminUserService interface {
	ListUsers(ctx context.Context, filter models.UserFilter) (models.UserListResponse, error)
	DeleteUser(ctx context.Context, userID int) error
}

type adminUserService struct {
	userRepo repositories.UserRepository
}

func NewAdminUserService(userRepo repositories.UserRepository) AdminUserService {
	return &adminUserService{userRepo: userRepo}
}

func (s *adminUserService) ListUsers(ctx context.Context, filter models.UserFilter) (models.UserListResponse, error) {
	users, total, err := s.userRepo.List(ctx, filter)
	if err != nil {
		return models.UserListResponse{}, err
	}

	for i := range users {
		users[i].PasswordHash = ""
	}
	return models.UserListResponse{
		Users:      users,
		TotalCount: total,
		Page:       filter.Page,
		Limit:      filter.Limit,
	}, nil
}

func (s *adminUserService) DeleteUser(ctx context.Context, userID int) error {
	return s.userRepo.Delete(ctx, userID)
}
