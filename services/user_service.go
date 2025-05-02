package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

var (
	ErrUserUpdateFailed = errors.New("failed to update user profile")
	ErrNicknameTaken    = errors.New("nickname is already taken")
)

type UserService interface {
	GetProfileByID(ctx context.Context, userID int) (*models.User, error)
	UpdateProfile(ctx context.Context, userID int, input UpdateProfileInput) (*models.User, error)
}

type UpdateProfileInput struct {
	FirstName *string
	LastName  *string
	Nickname  *string
}

type userService struct {
	userRepo repositories.UserRepository
}

func NewUserService(userRepo repositories.UserRepository) UserService {
	return &userService{
		userRepo: userRepo,
	}
}

func (s *userService) GetProfileByID(ctx context.Context, userID int) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by id %d: %w", userID, err)
	}

	user.PasswordHash = ""
	return user, nil
}

func (s *userService) UpdateProfile(ctx context.Context, userID int, input UpdateProfileInput) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user for update %d: %w", userID, err)
	}

	updated := false
	if input.FirstName != nil && *input.FirstName != user.FirstName {
		user.FirstName = *input.FirstName
		updated = true
	}
	if input.LastName != nil && *input.LastName != user.LastName {
		user.LastName = *input.LastName
		updated = true
	}
	if input.Nickname != nil {
		currentNickname := ""
		if user.Nickname != nil {
			currentNickname = *user.Nickname
		}
		if *input.Nickname != currentNickname {
			if *input.Nickname == "" {
				user.Nickname = nil
			} else {
				newNickname := *input.Nickname
				user.Nickname = &newNickname
			}
			updated = true
		}
	}

	if !updated {
		user.PasswordHash = ""
		return user, nil
	}

	err = s.userRepo.Update(ctx, user)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNicknameConflict) { // <--- ТЕПЕРЬ ЭТО КОРРЕКТНО
			return nil, ErrNicknameTaken
		}
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrUserUpdateFailed, err)
	}

	user.PasswordHash = ""
	return user, nil
}
