package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

var (
	ErrTournamentNameRequired      = errors.New("tournament name is required")
	ErrTournamentSportNotFound     = errors.New("specified sport not found")
	ErrTournamentFormatNotFound    = errors.New("specified format not found")
	ErrTournamentOrganizerNotFound = errors.New("specified organizer user not found")
	ErrTournamentCannotBeDeleted   = errors.New("tournament cannot be deleted in its current state or due to dependencies")
	ErrTournamentCreationFailed    = errors.New("failed to create tournament")
	ErrTournamentUpdateFailed      = errors.New("failed to update tournament")
	ErrTournamentDeleteFailed      = errors.New("failed to delete tournament")
	ErrTournamentListFailed        = errors.New("failed to list tournaments")
)

type TournamentService interface {
	CreateTournament(ctx context.Context, input CreateTournamentInput) (*models.Tournament, error)
	GetTournamentByID(ctx context.Context, id int) (*models.Tournament, error)
	ListTournaments(ctx context.Context, filter ListTournamentsFilter) ([]models.Tournament, error)
	UpdateTournamentDetails(ctx context.Context, id int, input UpdateTournamentDetailsInput) (*models.Tournament, error)
	UpdateTournamentStatus(ctx context.Context, id int, status models.TournamentStatus) (*models.Tournament, error)
	DeleteTournament(ctx context.Context, id int) error
}

type CreateTournamentInput struct {
	Name            string
	Description     *string
	SportID         int
	FormatID        int
	OrganizerID     int
	RegDate         time.Time
	StartDate       time.Time
	EndDate         time.Time
	Location        *string
	MaxParticipants int
}

type UpdateTournamentDetailsInput struct {
	Name            *string
	Description     *string
	RegDate         *time.Time
	StartDate       *time.Time
	EndDate         *time.Time
	Location        *string
	MaxParticipants *int
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
}

func NewTournamentService(
	tournamentRepo repositories.TournamentRepository,
	sportRepo repositories.SportRepository,
	formatRepo repositories.FormatRepository,
	userRepo repositories.UserRepository,
) TournamentService {
	return &tournamentService{
		tournamentRepo: tournamentRepo,
		sportRepo:      sportRepo,
		formatRepo:     formatRepo,
		userRepo:       userRepo,
	}
}

func (s *tournamentService) CreateTournament(ctx context.Context, input CreateTournamentInput) (*models.Tournament, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrTournamentNameRequired
	}
	if input.RegDate.IsZero() || input.StartDate.IsZero() || input.EndDate.IsZero() {
		return nil, errors.New("registration, start, and end dates are required")
	}
	if !input.RegDate.Before(input.StartDate) {
		return nil, ErrTournamentInvalidRegDate
	}
	if !input.StartDate.Before(input.EndDate) {
		return nil, ErrTournamentInvalidDateRange
	}
	if input.MaxParticipants <= 0 {
		return nil, ErrTournamentInvalidCapacity
	}

	if _, err := s.sportRepo.GetByID(ctx, input.SportID); err != nil {
		if errors.Is(err, repositories.ErrSportNotFound) {
			return nil, ErrTournamentSportNotFound
		}
		return nil, fmt.Errorf("failed to verify sport %d: %w", input.SportID, err)
	}

	if _, err := s.formatRepo.GetByID(ctx, input.FormatID); err != nil {
		if errors.Is(err, repositories.ErrFormatNotFound) {
			return nil, ErrTournamentFormatNotFound
		}
		return nil, fmt.Errorf("failed to verify format %d: %w", input.FormatID, err)
	}

	_, err := s.userRepo.GetByID(ctx, input.OrganizerID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrTournamentOrganizerNotFound
		}
		return nil, fmt.Errorf("failed to verify organizer %d: %w", input.OrganizerID, err)
	}

	tournament := &models.Tournament{
		Name:            name,
		Description:     input.Description,
		SportID:         input.SportID,
		FormatID:        input.FormatID,
		OrganizerID:     input.OrganizerID,
		RegDate:         input.RegDate,
		StartDate:       input.StartDate,
		EndDate:         input.EndDate,
		Location:        input.Location,
		MaxParticipants: input.MaxParticipants,
		Status:          models.StatusSoon,
	}

	err = s.tournamentRepo.Create(ctx, tournament)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNameConflict) {
			return nil, ErrTournamentNameConflict
		}
		return nil, fmt.Errorf("%w: %w", ErrTournamentCreationFailed, err)
	}

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

	return tournaments, nil
}

func (s *tournamentService) UpdateTournamentDetails(ctx context.Context, id int, input UpdateTournamentDetailsInput) (*models.Tournament, error) {
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("failed to get tournament %d for update: %w", id, err)
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
	if input.Description != nil && (tournament.Description == nil || *input.Description != *tournament.Description) {
		tournament.Description = input.Description
		updated = true
	}
	if input.RegDate != nil && !input.RegDate.Equal(tournament.RegDate) {
		tournament.RegDate = *input.RegDate
		updated = true
	}
	if input.StartDate != nil && !input.StartDate.Equal(tournament.StartDate) {
		tournament.StartDate = *input.StartDate
		updated = true
	}
	if input.EndDate != nil && !input.EndDate.Equal(tournament.EndDate) {
		tournament.EndDate = *input.EndDate
		updated = true
	}
	if input.Location != nil && (tournament.Location == nil || *input.Location != *tournament.Location) {
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

	if !tournament.RegDate.IsZero() && !tournament.StartDate.IsZero() && !tournament.RegDate.Before(tournament.StartDate) {
		return nil, ErrTournamentInvalidRegDate
	}
	if !tournament.StartDate.IsZero() && !tournament.EndDate.IsZero() && !tournament.StartDate.Before(tournament.EndDate) {
		return nil, ErrTournamentInvalidDateRange
	}

	if !updated {
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

	return tournament, nil
}

func (s *tournamentService) UpdateTournamentStatus(ctx context.Context, id int, newStatus models.TournamentStatus) (*models.Tournament, error) {
	switch newStatus {
	case models.StatusSoon, models.StatusRegistration, models.StatusActive, models.StatusCompleted, models.StatusCanceled:
	default:
		return nil, ErrTournamentInvalidStatus
	}

	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("failed to get tournament %d for status update: %w", id, err)
	}

	if !isValidStatusTransition(tournament.Status, newStatus) {
		return nil, fmt.Errorf("%w: from %s to %s", ErrTournamentInvalidStatusTransition, tournament.Status, newStatus)
	}

	err = s.tournamentRepo.UpdateStatus(ctx, id, newStatus)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("%w: %w", ErrTournamentUpdateFailed, err)
	}

	tournament.Status = newStatus
	return tournament, nil
}

func (s *tournamentService) DeleteTournament(ctx context.Context, id int) error {
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return ErrTournamentNotFound
		}
		return fmt.Errorf("failed to get tournament %d for deletion check: %w", id, err)
	}

	if tournament.Status != models.StatusSoon && tournament.Status != models.StatusCanceled {
		return fmt.Errorf("%w: cannot delete tournament with status %s", ErrTournamentCannotBeDeleted, tournament.Status)
	}

	err = s.tournamentRepo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return ErrTournamentNotFound
		}
		if errors.Is(err, repositories.ErrTournamentInUse) {
			return ErrTournamentCannotBeDeleted
		}
		return fmt.Errorf("%w: %w", ErrTournamentDeleteFailed, err)
	}

	return nil
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
