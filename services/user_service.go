package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
	"github.com/Dosada05/tournament-system/storage"
	"github.com/Dosada05/tournament-system/utils"
)

const (
	minPasswordLength = 8
	userLogoPrefix    = "logos/users"
)

var (
	ErrUserUpdateFailed               = errors.New("failed to update user profile")
	ErrNicknameTaken                  = errors.New("nickname is already taken")
	ErrEmailTaken                     = errors.New("email is already taken")
	ErrInvalidEmailFormat             = errors.New("invalid email format")
	ErrPasswordHashingFailed          = errors.New("failed to hash password")
	ErrLogoUploadFailed               = errors.New("failed to upload logo")
	ErrInvalidLogoFormat              = errors.New("invalid logo file format or content type")
	ErrLogoUpdateDatabaseFailed       = errors.New("failed to update logo information in database")
	ErrLogoDeleteFailed               = errors.New("failed to delete previous logo")
	ErrCouldNotDetermineFileExtension = errors.New("could not determine file extension from content type")
)

type UserService interface {
	GetProfileByID(ctx context.Context, userID int) (*models.User, error)
	UpdateProfile(ctx context.Context, userID int, input UpdateProfileInput) (*models.User, error)
	ListUsersByTeamID(ctx context.Context, teamID int) ([]models.User, error)
	UpdateUserLogo(ctx context.Context, targetUserID int, currentUserID int, file io.Reader, contentType string) (*models.User, error)
}

type UpdateProfileInput struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Nickname  *string `json:"nickname"`
	Email     *string `json:"email"`
	Password  *string `json:"password"`
}

type userService struct {
	userRepo repositories.UserRepository
	uploader storage.FileUploader
}

func NewUserService(userRepo repositories.UserRepository, uploader storage.FileUploader) UserService {
	return &userService{
		userRepo: userRepo,
		uploader: uploader,
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
	s.populateLogoURL(user)
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
	if input.FirstName != nil {
		trimmedFirstName := strings.TrimSpace(*input.FirstName)
		if trimmedFirstName != user.FirstName {
			user.FirstName = trimmedFirstName
			updated = true
		}
	}
	if input.LastName != nil {
		trimmedLastName := strings.TrimSpace(*input.LastName)
		if trimmedLastName != user.LastName {
			user.LastName = trimmedLastName
			updated = true
		}
	}
	if input.Nickname != nil {
		trimmedNickname := strings.TrimSpace(*input.Nickname)
		currentNickname := ""
		if user.Nickname != nil {
			currentNickname = *user.Nickname
		}
		if trimmedNickname != currentNickname {
			if trimmedNickname == "" {
				user.Nickname = nil
			} else {
				user.Nickname = &trimmedNickname
			}
			updated = true
		}
	}
	if input.Email != nil {
		newEmail := strings.ToLower(strings.TrimSpace(*input.Email))
		if newEmail == "" {
			return nil, ErrInvalidEmailFormat
		}
		if !utils.IsValidEmail(newEmail) {
			return nil, ErrInvalidEmailFormat
		}
		if newEmail != user.Email {
			user.Email = newEmail
			updated = true
		}
	}
	if input.Password != nil {
		newPassword := *input.Password
		if len(newPassword) < minPasswordLength {
			return nil, ErrPasswordTooShort
		}
		newPasswordHash, err := hashPassword(newPassword)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrPasswordHashingFailed, err)
		}
		user.PasswordHash = newPasswordHash
		updated = true
	}
	if !updated {
		user.PasswordHash = ""
		s.populateLogoURL(user)
		return user, nil
	}
	err = s.userRepo.Update(ctx, user)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNicknameConflict) {
			return nil, ErrNicknameTaken
		}
		if errors.Is(err, repositories.ErrUserEmailConflict) {
			return nil, ErrEmailTaken
		}
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrUserUpdateFailed, err)
	}
	user.PasswordHash = ""
	s.populateLogoURL(user)
	return user, nil
}

func (s *userService) ListUsersByTeamID(ctx context.Context, teamID int) ([]models.User, error) {
	if teamID <= 0 {
		return nil, errors.New("invalid team ID")
	}
	users, err := s.userRepo.ListByTeamID(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to list users by team id from repository: %w", err)
	}
	for i := range users {
		users[i].PasswordHash = ""
		s.populateLogoURL(&users[i])
	}
	return users, nil
}

func (s *userService) UpdateUserLogo(ctx context.Context, targetUserID int, currentUserID int, file io.Reader, contentType string) (*models.User, error) {
	if targetUserID != currentUserID {
		return nil, ErrForbiddenOperation
	}
	if !strings.HasPrefix(contentType, "image/") {
		return nil, ErrInvalidLogoFormat
	}
	user, err := s.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user %d for logo update: %w", targetUserID, err)
	}
	oldLogoKey := user.LogoKey
	ext, err := getExtensionFromContentType(contentType)
	if err != nil {
		return nil, err
	}
	newKey := fmt.Sprintf("%s/%d/avatar_%d%s", userLogoPrefix, targetUserID, time.Now().UnixNano(), ext)
	_, err = s.uploader.Upload(ctx, newKey, contentType, file)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrLogoUploadFailed, err)
	}
	err = s.userRepo.UpdateLogoKey(ctx, targetUserID, newKey)
	if err != nil {
		if deleteErr := s.uploader.Delete(context.Background(), newKey); deleteErr != nil {
			fmt.Printf("CRITICAL: Failed to delete uploaded file %s after DB error: %v\n", newKey, deleteErr)
		}
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrLogoUpdateDatabaseFailed, err)
	}
	if oldLogoKey != nil && *oldLogoKey != "" && *oldLogoKey != newKey {
		go func(keyToDelete string) {
			if deleteErr := s.uploader.Delete(context.Background(), keyToDelete); deleteErr != nil {
				fmt.Printf("Failed to delete old user logo %s: %v\n", keyToDelete, deleteErr)
			}
		}(*oldLogoKey)
	}
	user.LogoKey = &newKey
	user.PasswordHash = ""
	s.populateLogoURL(user)
	return user, nil
}

func (s *userService) populateLogoURL(user *models.User) {
	if user != nil && user.LogoKey != nil && *user.LogoKey != "" {
		url := s.uploader.GetPublicURL(*user.LogoKey)
		if url != "" {
			user.LogoURL = &url
		}
	}
}

func hashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}

func getExtensionFromContentType(contentType string) (string, error) {
	switch contentType {
	case "image/jpeg":
		return ".jpg", nil
	case "image/png":
		return ".png", nil
	case "image/gif":
		return ".gif", nil
	case "image/webp":
		return ".webp", nil
	default:
		parts := strings.Split(contentType, "/")
		if len(parts) == 2 && strings.HasPrefix(parts[0], "image") && parts[1] != "" {
			ext := "." + strings.Split(parts[1], "+")[0]
			return ext, nil
		}
		return "", fmt.Errorf("%w: unsupported content type '%s'", ErrCouldNotDetermineFileExtension, contentType)
	}
}
