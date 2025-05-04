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
	ListUsersByTeamID(ctx context.Context, teamID int) ([]models.User, error)
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

func (s *userService) ListUsersByTeamID(ctx context.Context, teamID int) ([]models.User, error) {
	if teamID <= 0 {
		return nil, errors.New("invalid team ID") // Базовая валидация ID
	}

	users, err := s.userRepo.ListByTeamID(ctx, teamID)
	if err != nil {
		// В ListByTeamID репозитория обычно нет специфичных ошибок типа NotFound или Conflict,
		// поэтому просто оборачиваем ошибку репозитория.
		return nil, fmt.Errorf("failed to list users by team id from repository: %w", err)
	}

	// Сервис не должен возвращать хеши паролей
	for i := range users {
		users[i].PasswordHash = ""
		// Здесь НЕ нужно очищать users[i].Team, так как репозиторий ListByTeamID
		// (согласно коду из предыдущего шага) не делает JOIN и не заполняет user.Team.
		// Если бы он делал JOIN, то очистка была бы нужна здесь.
	}

	// Возвращаем пустой слайс, если команда пуста (не ошибку)
	return users, nil
}
