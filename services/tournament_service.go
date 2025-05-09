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
	tournamentLogoPrefix = "logos/tournaments"
)

var (
	ErrTournamentNameRequired  = errors.New("tournament name is required")
	ErrTournamentDatesRequired = errors.New("registration, start, and end dates are required")

	ErrTournamentSportNotFound     = errors.New("specified sport not found")
	ErrTournamentFormatNotFound    = errors.New("specified format not found")
	ErrTournamentOrganizerNotFound = errors.New("specified organizer user not found")

	ErrTournamentCreationFailed     = errors.New("failed to create tournament")
	ErrTournamentUpdateFailed       = errors.New("failed to update tournament")
	ErrTournamentDeleteFailed       = errors.New("failed to delete tournament")
	ErrTournamentListFailed         = errors.New("failed to list tournaments")
	ErrTournamentUpdateNotAllowed   = errors.New("tournament update not allowed in current status")
	ErrTournamentDeletionNotAllowed = errors.New("tournament deletion not allowed")

	ErrTournamentInUse = repositories.ErrTournamentInUse
)

type TournamentService interface {
	CreateTournament(ctx context.Context, organizerID int, input CreateTournamentInput) (*models.Tournament, error)
	GetTournamentByID(ctx context.Context, id int) (*models.Tournament, error)
	ListTournaments(ctx context.Context, filter ListTournamentsFilter) ([]models.Tournament, error)
	UpdateTournamentDetails(ctx context.Context, id int, currentUserID int, input UpdateTournamentDetailsInput) (*models.Tournament, error)
	UpdateTournamentStatus(ctx context.Context, id int, currentUserID int, status models.TournamentStatus) (*models.Tournament, error)
	DeleteTournament(ctx context.Context, id int, currentUserID int) error
	UploadTournamentLogo(ctx context.Context, tournamentID int, currentUserID int, file io.Reader, contentType string) (*models.Tournament, error)
}

type CreateTournamentInput struct {
	Name            string    `json:"name" validate:"required"`
	Description     *string   `json:"description"`
	SportID         int       `json:"sport_id" validate:"required,gt=0"`
	FormatID        int       `json:"format_id" validate:"required,gt=0"`
	RegDate         time.Time `json:"reg_date" validate:"required"`
	StartDate       time.Time `json:"start_date" validate:"required"`
	EndDate         time.Time `json:"end_date" validate:"required"`
	Location        *string   `json:"location"`
	MaxParticipants int       `json:"max_participants" validate:"required,gt=0"`
}

type UpdateTournamentDetailsInput struct {
	Name            *string    `json:"name"`
	Description     *string    `json:"description"`
	RegDate         *time.Time `json:"reg_date"`
	StartDate       *time.Time `json:"start_date"`
	EndDate         *time.Time `json:"end_date"`
	Location        *string    `json:"location"`
	MaxParticipants *int       `json:"max_participants" validate:"omitempty,gt=0"`
}

type ListTournamentsFilter struct {
	SportID     *int
	FormatID    *int
	OrganizerID *int
	Status      *models.TournamentStatus
	Limit       int
	Offset      int
}

type tournamentService struct {
	tournamentRepo repositories.TournamentRepository
	sportRepo      repositories.SportRepository
	formatRepo     repositories.FormatRepository
	userRepo       repositories.UserRepository
	uploader       storage.FileUploader
}

func NewTournamentService(
	tournamentRepo repositories.TournamentRepository,
	sportRepo repositories.SportRepository,
	formatRepo repositories.FormatRepository,
	userRepo repositories.UserRepository,
	uploader storage.FileUploader,
) TournamentService {
	return &tournamentService{
		tournamentRepo: tournamentRepo,
		sportRepo:      sportRepo,
		formatRepo:     formatRepo,
		userRepo:       userRepo,
		uploader:       uploader,
	}
}

func (s *tournamentService) populateTournamentLogoURL(tournament *models.Tournament) {
	if tournament != nil && tournament.LogoKey != nil && *tournament.LogoKey != "" && s.uploader != nil {
		url := s.uploader.GetPublicURL(*tournament.LogoKey)
		if url != "" {
			tournament.LogoURL = &url
		}
	}
}

func (s *tournamentService) populateTournamentListLogoURLs(tournaments []models.Tournament) {
	for i := range tournaments {
		s.populateTournamentLogoURL(&tournaments[i])
	}
}

func (s *tournamentService) CreateTournament(ctx context.Context, organizerID int, input CreateTournamentInput) (*models.Tournament, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrTournamentNameRequired
	}
	if err := validateTournamentDates(input.RegDate, input.StartDate, input.EndDate); err != nil {
		return nil, err
	}
	if input.MaxParticipants <= 0 {
		return nil, ErrTournamentInvalidCapacity
	}

	var validationErr error
	_, validationErr = s.sportRepo.GetByID(ctx, input.SportID)
	if validationErr != nil {
		if errors.Is(validationErr, repositories.ErrSportNotFound) {
			return nil, ErrTournamentSportNotFound
		}
		return nil, fmt.Errorf("failed to verify sport %d: %w", input.SportID, validationErr)
	}
	_, validationErr = s.formatRepo.GetByID(ctx, input.FormatID)
	if validationErr != nil {
		if errors.Is(validationErr, repositories.ErrFormatNotFound) {
			return nil, ErrTournamentFormatNotFound
		}
		return nil, fmt.Errorf("failed to verify format %d: %w", input.FormatID, validationErr)
	}
	_, validationErr = s.userRepo.GetByID(ctx, organizerID)
	if validationErr != nil {
		if errors.Is(validationErr, repositories.ErrUserNotFound) {
			return nil, ErrTournamentOrganizerNotFound
		}
		return nil, fmt.Errorf("failed to verify organizer %d: %w", organizerID, validationErr)
	}

	tournament := &models.Tournament{
		Name:            name,
		Description:     input.Description,
		SportID:         input.SportID,
		FormatID:        input.FormatID,
		OrganizerID:     organizerID,
		RegDate:         input.RegDate,
		StartDate:       input.StartDate,
		EndDate:         input.EndDate,
		Location:        input.Location,
		MaxParticipants: input.MaxParticipants,
		Status:          models.StatusSoon,
	}

	err := s.tournamentRepo.Create(ctx, tournament)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNameConflict) {
			return nil, ErrTournamentNameConflict
		}
		if errors.Is(err, repositories.ErrTournamentInvalidSport) {
			return nil, ErrTournamentSportNotFound
		}
		if errors.Is(err, repositories.ErrTournamentInvalidFormat) {
			return nil, ErrTournamentFormatNotFound
		}
		if errors.Is(err, repositories.ErrTournamentInvalidOrg) {
			return nil, ErrTournamentOrganizerNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrTournamentCreationFailed, err)
	}
	s.populateTournamentLogoURL(tournament)
	return tournament, nil
}

func (s *tournamentService) GetTournamentByID(ctx context.Context, id int) (*models.Tournament, error) {
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("failed to get tournament by id %d: %w", id, err)
	}
	s.populateTournamentLogoURL(tournament)
	return tournament, nil
}

func (s *tournamentService) ListTournaments(ctx context.Context, filter ListTournamentsFilter) ([]models.Tournament, error) {
	repoFilter := repositories.ListTournamentsFilter{
		SportID:     filter.SportID,
		FormatID:    filter.FormatID,
		OrganizerID: filter.OrganizerID,
		Status:      filter.Status,
		Limit:       filter.Limit,
		Offset:      filter.Offset,
	}
	tournaments, err := s.tournamentRepo.List(ctx, repoFilter)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTournamentListFailed, err)
	}
	if tournaments == nil {
		return []models.Tournament{}, nil
	}
	s.populateTournamentListLogoURLs(tournaments)
	return tournaments, nil
}

func (s *tournamentService) UpdateTournamentDetails(ctx context.Context, id int, currentUserID int, input UpdateTournamentDetailsInput) (*models.Tournament, error) {
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("failed to get tournament %d for update: %w", id, err)
	}

	if tournament.OrganizerID != currentUserID {
		return nil, ErrForbiddenOperation
	}

	if tournament.Status != models.StatusSoon && tournament.Status != models.StatusRegistration {
		return nil, fmt.Errorf("%w: cannot update tournament in status '%s'", ErrTournamentUpdateNotAllowed, tournament.Status)
	}

	updated := false
	if input.Name != nil {
		trimmedName := strings.TrimSpace(*input.Name)
		if trimmedName == "" {
			return nil, ErrTournamentNameRequired
		}
		if trimmedName != tournament.Name {
			tournament.Name = trimmedName
			updated = true
		}
	}
	if input.Description != nil && derefString(input.Description) != derefString(tournament.Description) {
		tournament.Description = input.Description
		updated = true
	}
	if input.RegDate != nil && !input.RegDate.IsZero() && !input.RegDate.Equal(tournament.RegDate) {
		tournament.RegDate = *input.RegDate
		updated = true
	}
	if input.StartDate != nil && !input.StartDate.IsZero() && !input.StartDate.Equal(tournament.StartDate) {
		tournament.StartDate = *input.StartDate
		updated = true
	}
	if input.EndDate != nil && !input.EndDate.IsZero() && !input.EndDate.Equal(tournament.EndDate) {
		tournament.EndDate = *input.EndDate
		updated = true
	}
	if input.Location != nil && derefString(input.Location) != derefString(tournament.Location) {
		tournament.Location = input.Location
		updated = true
	}
	if input.MaxParticipants != nil && *input.MaxParticipants != tournament.MaxParticipants {
		if *input.MaxParticipants <= 0 {
			return nil, ErrTournamentInvalidCapacity
		}
		tournament.MaxParticipants = *input.MaxParticipants
		updated = true
	}

	if updated {
		if err := validateTournamentDates(tournament.RegDate, tournament.StartDate, tournament.EndDate); err != nil {
			return nil, err
		}
	}

	if !updated {
		s.populateTournamentLogoURL(tournament)
		return tournament, nil
	}

	err = s.tournamentRepo.Update(ctx, tournament)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNameConflict) {
			return nil, ErrTournamentNameConflict
		}
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrTournamentUpdateFailed, err)
	}
	s.populateTournamentLogoURL(tournament)
	return tournament, nil
}

func (s *tournamentService) UpdateTournamentStatus(ctx context.Context, id int, currentUserID int, newStatus models.TournamentStatus) (*models.Tournament, error) {
	if !isValidTournamentStatus(newStatus) {
		return nil, fmt.Errorf("%w: '%s'", ErrTournamentInvalidStatus, newStatus)
	}

	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("failed to get tournament %d for status update: %w", id, err)
	}

	if tournament.OrganizerID != currentUserID {
		return nil, ErrForbiddenOperation
	}

	if !isValidStatusTransition(tournament.Status, newStatus) {
		return nil, fmt.Errorf("%w: from '%s' to '%s'", ErrTournamentInvalidStatusTransition, tournament.Status, newStatus)
	}

	err = s.tournamentRepo.UpdateStatus(ctx, id, newStatus)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("%w: failed to update status in repository: %w", ErrTournamentUpdateFailed, err)
	}

	tournament.Status = newStatus
	s.populateTournamentLogoURL(tournament)
	return tournament, nil
}

func (s *tournamentService) DeleteTournament(ctx context.Context, id int, currentUserID int) error {
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return ErrTournamentNotFound
		}
		return fmt.Errorf("failed to get tournament %d for deletion check: %w", id, err)
	}

	if tournament.OrganizerID != currentUserID {
		return ErrForbiddenOperation
	}

	if tournament.Status != models.StatusSoon && tournament.Status != models.StatusRegistration && tournament.Status != models.StatusCanceled {
		return fmt.Errorf("%w: cannot delete tournament with status '%s'", ErrTournamentDeletionNotAllowed, tournament.Status)
	}

	oldLogoKey := tournament.LogoKey

	err = s.tournamentRepo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return ErrTournamentNotFound
		}
		if errors.Is(err, repositories.ErrTournamentInUse) {
			return fmt.Errorf("%w: tournament might have participants or matches", ErrTournamentDeletionNotAllowed)
		}
		return fmt.Errorf("%w: %w", ErrTournamentDeleteFailed, err)
	}

	if oldLogoKey != nil && *oldLogoKey != "" && s.uploader != nil {
		go func(keyToDelete string) {
			deleteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if deleteErr := s.uploader.Delete(deleteCtx, keyToDelete); deleteErr != nil {
				fmt.Printf("Warning: Failed to delete tournament logo %s during tournament deletion: %v\n", keyToDelete, deleteErr)
			}
		}(*oldLogoKey)
	}

	return nil
}

func (s *tournamentService) UploadTournamentLogo(ctx context.Context, tournamentID int, currentUserID int, file io.Reader, contentType string) (*models.Tournament, error) {
	tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("failed to get tournament %d for logo upload: %w", tournamentID, err)
	}

	if tournament.OrganizerID != currentUserID {
		return nil, ErrForbiddenOperation
	}

	if !strings.HasPrefix(contentType, "image/") {
		return nil, ErrInvalidLogoFormat
	}

	if s.uploader == nil {
		return nil, errors.New("file uploader is not configured")
	}

	oldLogoKey := tournament.LogoKey

	ext, err := GetExtensionFromContentType(contentType) // Используем общую функцию
	if err != nil {
		return nil, err
	}

	newKey := fmt.Sprintf("%s/%d/logo_%d%s", tournamentLogoPrefix, tournamentID, time.Now().UnixNano(), ext)

	_, err = s.uploader.Upload(ctx, newKey, contentType, file)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrLogoUploadFailed, newKey, err)
	}

	err = s.tournamentRepo.UpdateLogoKey(ctx, tournamentID, &newKey)
	if err != nil {
		if deleteErr := s.uploader.Delete(context.Background(), newKey); deleteErr != nil {
			fmt.Printf("CRITICAL: Failed to delete uploaded tournament logo %s after DB update error: %v. DB error: %v\n", newKey, deleteErr, err)
		}
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrLogoUpdateDatabaseFailed, err)
	}

	if oldLogoKey != nil && *oldLogoKey != "" && *oldLogoKey != newKey {
		go func(keyToDelete string) {
			deleteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if deleteErr := s.uploader.Delete(deleteCtx, keyToDelete); deleteErr != nil {
				fmt.Printf("Warning: Failed to delete old tournament logo %s: %v\n", keyToDelete, deleteErr)
			}
		}(*oldLogoKey)
	}

	tournament.LogoKey = &newKey
	s.populateTournamentLogoURL(tournament)
	return tournament, nil
}

func isValidTournamentStatus(status models.TournamentStatus) bool {
	switch status {
	case models.StatusSoon, models.StatusRegistration, models.StatusActive, models.StatusCompleted, models.StatusCanceled:
		return true
	default:
		return false
	}
}

func isValidStatusTransition(current, next models.TournamentStatus) bool {
	if current == next {
		return true
	}
	allowedTransitions := map[models.TournamentStatus][]models.TournamentStatus{
		models.StatusSoon:         {models.StatusRegistration, models.StatusCanceled},
		models.StatusRegistration: {models.StatusActive, models.StatusCanceled},
		models.StatusActive:       {models.StatusCompleted, models.StatusCanceled},
		models.StatusCompleted:    {},
		models.StatusCanceled:     {},
	}
	allowed, ok := allowedTransitions[current]
	if !ok {
		return false
	}
	for _, nextAllowed := range allowed {
		if next == nextAllowed {
			return true
		}
	}
	return false
}

func validateTournamentDates(reg, start, end time.Time) error {
	if reg.IsZero() || start.IsZero() || end.IsZero() {
		return ErrTournamentDatesRequired
	}
	if reg.After(start) {
		return fmt.Errorf("%w: registration date (%s) cannot be after start date (%s)", ErrTournamentInvalidRegDate, reg.Format(time.DateOnly), start.Format(time.DateOnly))
	}
	if !start.Before(end) {
		return fmt.Errorf("%w: start date (%s) must be before end date (%s)", ErrTournamentInvalidDateRange, start.Format(time.DateOnly), end.Format(time.DateOnly))
	}
	return nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
