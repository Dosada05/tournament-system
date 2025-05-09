package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
	"github.com/Dosada05/tournament-system/storage"
)

const (
	teamLogoPrefix = "logos/teams"
)

var (
	ErrTeamCreationFailed       = errors.New("failed to create team")
	ErrTeamUpdateFailed         = errors.New("failed to update team")
	ErrTeamDeleteFailed         = errors.New("failed to delete team")
	ErrTeamCannotDeleteNotEmpty = errors.New("team cannot be deleted because it is not empty")
	ErrUserNotInThisTeam        = errors.New("user is not a member of this team")
	ErrMemberAddFailed          = errors.New("failed to add member to team")
	ErrMemberRemoveFailed       = errors.New("failed to remove member from team")
	ErrInvalidSportID           = errors.New("invalid sport ID provided")
)

type TeamService interface {
	CreateTeam(ctx context.Context, input CreateTeamInput) (*models.Team, error)
	GetTeamByID(ctx context.Context, teamID int) (*models.Team, error)
	UpdateTeamDetails(ctx context.Context, teamID int, input UpdateTeamInput, currentUserID int) (*models.Team, error)
	RemoveMember(ctx context.Context, teamID int, userIDToRemove int, currentUserID int) error
	DeleteTeam(ctx context.Context, teamID int, currentUserID int) error
	UploadLogo(ctx context.Context, teamID int, currentUserID int, file io.Reader, contentType string) (*models.Team, error)
}

type CreateTeamInput struct {
	Name      string `json:"name"`
	SportID   int    `json:"sport_id"`
	CreatorID int
}

type UpdateTeamInput struct {
	Name    *string
	SportID *int
}

type teamService struct {
	teamRepo  repositories.TeamRepository
	userRepo  repositories.UserRepository
	sportRepo repositories.SportRepository
	uploader  storage.FileUploader
}

func NewTeamService(
	teamRepo repositories.TeamRepository,
	userRepo repositories.UserRepository,
	sportRepo repositories.SportRepository,
	uploader storage.FileUploader,
) TeamService {
	return &teamService{
		teamRepo:  teamRepo,
		userRepo:  userRepo,
		sportRepo: sportRepo,
		uploader:  uploader,
	}
}

func (s *teamService) CreateTeam(ctx context.Context, input CreateTeamInput) (*models.Team, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrTeamNameRequired
	}

	if _, err := s.sportRepo.GetByID(ctx, input.SportID); err != nil {
		if errors.Is(err, repositories.ErrSportNotFound) {
			return nil, ErrInvalidSportID
		}
		return nil, fmt.Errorf("failed to verify sport %d: %w", input.SportID, err)
	}

	creator, err := s.userRepo.GetByID(ctx, input.CreatorID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get creator user %d: %w", input.CreatorID, err)
	}

	if creator.TeamID != nil {
		return nil, ErrUserAlreadyInTeam
	}

	team := &models.Team{
		Name:      name,
		SportID:   input.SportID,
		CaptainID: input.CreatorID,
	}

	err = s.teamRepo.Create(ctx, team)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrTeamNameConflict):
			return nil, ErrTeamNameConflict
		case errors.Is(err, repositories.ErrTeamCaptainInvalid):
			return nil, ErrUserNotFound
		case errors.Is(err, repositories.ErrTeamSportInvalid):
			return nil, ErrInvalidSportID
		default:
			return nil, fmt.Errorf("%w: %w", ErrTeamCreationFailed, err)
		}
	}

	creator.TeamID = &team.ID
	err = s.userRepo.Update(ctx, creator)
	if err != nil {
		_ = s.teamRepo.Delete(ctx, team.ID)
		return nil, fmt.Errorf("failed to assign creator %d to new team %d: %w", creator.ID, team.ID, err)
	}

	return team, nil
}

func (s *teamService) GetTeamByID(ctx context.Context, teamID int) (*models.Team, error) {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrTeamNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("failed to get team by id %d: %w", teamID, err)
	}
	s.populateTeamLogoURL(team)
	return team, nil
}

func (s *teamService) UpdateTeamDetails(ctx context.Context, teamID int, input UpdateTeamInput, currentUserID int) (*models.Team, error) {
	team, err := s.GetTeamByID(ctx, teamID)
	if err != nil {
		return nil, err
	}

	if team.CaptainID != currentUserID {
		return nil, ErrCaptainActionForbidden
	}

	updated := false
	if input.Name != nil {
		trimmedName := strings.TrimSpace(*input.Name)
		if trimmedName == "" {
			return nil, ErrTeamNameRequired
		}
		if trimmedName != team.Name {
			team.Name = trimmedName
			updated = true
		}
	}
	if input.SportID != nil && *input.SportID != team.SportID {
		if _, err := s.sportRepo.GetByID(ctx, *input.SportID); err != nil {
			if errors.Is(err, repositories.ErrSportNotFound) {
				return nil, ErrInvalidSportID
			}
			return nil, fmt.Errorf("failed to verify sport %d for update: %w", *input.SportID, err)
		}
		team.SportID = *input.SportID
		updated = true
	}

	if !updated {
		s.populateTeamLogoURL(team)
		return team, nil
	}

	err = s.teamRepo.Update(ctx, team)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrTeamNameConflict):
			return nil, ErrTeamNameConflict
		case errors.Is(err, repositories.ErrTeamCaptainInvalid):
			return nil, ErrUserNotFound
		case errors.Is(err, repositories.ErrTeamSportInvalid):
			return nil, ErrInvalidSportID
		case errors.Is(err, repositories.ErrTeamNotFound):
			return nil, ErrTeamNotFound
		default:
			return nil, fmt.Errorf("%w: %w", ErrTeamUpdateFailed, err)
		}
	}
	s.populateTeamLogoURL(team)
	return team, nil
}

func (s *teamService) RemoveMember(ctx context.Context, teamID int, userIDToRemove int, currentUserID int) error {
	team, err := s.GetTeamByID(ctx, teamID)
	if err != nil {
		return err
	}

	userToRemove, err := s.userRepo.GetByID(ctx, userIDToRemove)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to get user %d to remove: %w", userIDToRemove, err)
	}

	if userToRemove.TeamID == nil || *userToRemove.TeamID != team.ID {
		return ErrUserNotInThisTeam
	}

	isCaptainAction := team.CaptainID == currentUserID
	isSelfLeave := userIDToRemove == currentUserID

	if !isCaptainAction && !isSelfLeave {
		return ErrSelfLeaveForbidden
	}

	if userToRemove.ID == team.CaptainID {
		return ErrCannotRemoveCaptain
	}

	userToRemove.TeamID = nil
	err = s.userRepo.Update(ctx, userToRemove)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrMemberRemoveFailed, err)
	}

	return nil
}

func (s *teamService) DeleteTeam(ctx context.Context, teamID int, currentUserID int) error {
	team, err := s.GetTeamByID(ctx, teamID)
	if err != nil {
		return err
	}

	if team.CaptainID != currentUserID {
		return ErrCaptainActionForbidden
	}

	members, err := s.userRepo.ListByTeamID(ctx, teamID)
	if err != nil {
		return fmt.Errorf("failed to check team members for deletion: %w", err)
	}

	if len(members) > 1 {
		return fmt.Errorf("%w: team has %d members", ErrTeamCannotDeleteNotEmpty, len(members))
	}
	if len(members) == 1 && members[0].ID != team.CaptainID {
		return fmt.Errorf("%w: the only remaining member is not the captain", ErrTeamCannotDeleteNotEmpty)
	}

	err = s.teamRepo.Delete(ctx, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrTeamNotFound) {
			return ErrTeamNotFound
		}
		return fmt.Errorf("%w: %w", ErrTeamDeleteFailed, err)
	}

	if len(members) == 1 && members[0].ID == team.CaptainID {
		captain := members[0]
		captain.TeamID = nil
		errUpdate := s.userRepo.Update(ctx, &captain)
		if errUpdate != nil {
			fmt.Printf("Warning: failed to remove team ID from captain %d after team %d deletion: %v\n", captain.ID, teamID, errUpdate)
		}
	}
	if team.LogoKey != nil && *team.LogoKey != "" {
		go func(keyToDelete string) {
			deleteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if deleteErr := s.uploader.Delete(deleteCtx, keyToDelete); deleteErr != nil {
				fmt.Printf("Warning: Failed to delete team logo %s during team deletion: %v\n", keyToDelete, deleteErr)
			}
		}(*team.LogoKey)
	}

	return nil
}

func (s *teamService) UploadLogo(ctx context.Context, teamID int, currentUserID int, file io.Reader, contentType string) (*models.Team, error) {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrTeamNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("failed to get team %d for logo upload: %w", teamID, err)
	}

	if team.CaptainID != currentUserID {
		return nil, ErrCaptainActionForbidden
	}

	if !strings.HasPrefix(contentType, "image/") {
		return nil, ErrInvalidLogoFormat
	}

	oldLogoKey := team.LogoKey

	ext, err := GetExtensionFromContentType(contentType)
	if err != nil {
		return nil, err
	}

	newKey := fmt.Sprintf("%s/%d/logo_%d%s", teamLogoPrefix, teamID, time.Now().UnixNano(), ext)

	_, err = s.uploader.Upload(ctx, newKey, contentType, file)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrLogoUploadFailed, newKey, err)
	}

	err = s.teamRepo.UpdateLogoKey(ctx, teamID, &newKey)
	if err != nil {
		if deleteErr := s.uploader.Delete(context.Background(), newKey); deleteErr != nil {
			fmt.Printf("CRITICAL: Failed to delete uploaded team logo %s after DB update error: %v. DB error: %v\n", newKey, deleteErr, err)
		}
		if errors.Is(err, repositories.ErrTeamNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrLogoUpdateDatabaseFailed, err)
	}

	if oldLogoKey != nil && *oldLogoKey != "" && *oldLogoKey != newKey {
		go func(keyToDelete string) {
			deleteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if deleteErr := s.uploader.Delete(deleteCtx, keyToDelete); deleteErr != nil {
				fmt.Printf("Warning: Failed to delete old team logo %s: %v\n", keyToDelete, deleteErr)
			}
		}(*oldLogoKey)
	}

	team.LogoKey = &newKey
	s.populateTeamLogoURL(team)
	return team, nil
}

func (s *teamService) populateTeamLogoURL(team *models.Team) {
	if team != nil && team.LogoKey != nil && *team.LogoKey != "" {
		url := s.uploader.GetPublicURL(*team.LogoKey)
		if url != "" {
			team.LogoURL = &url
		}
	}
}

func GetExtensionFromContentType(contentType string) (string, error) {
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
