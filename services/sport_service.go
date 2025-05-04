package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

var (
	ErrSportNameRequired   = errors.New("sport name is required")
	ErrSportNameConflict   = errors.New("sport name already exists")
	ErrSportInUse          = errors.New("sport cannot be deleted as it is currently in use")
	ErrSportCreationFailed = errors.New("failed to create sport")
	ErrSportUpdateFailed   = errors.New("failed to update sport")
	ErrSportDeleteFailed   = errors.New("failed to delete sport")
)

type SportService interface {
	CreateSport(ctx context.Context, input CreateSportInput) (*models.Sport, error)
	GetSportByID(ctx context.Context, id int) (*models.Sport, error)
	GetAllSports(ctx context.Context) ([]models.Sport, error)
	UpdateSport(ctx context.Context, id int, input UpdateSportInput) (*models.Sport, error)
	DeleteSport(ctx context.Context, id int) error
}

type CreateSportInput struct {
	Name string
}

type UpdateSportInput struct {
	Name string
}

type sportService struct {
	sportRepo repositories.SportRepository
}

func NewSportService(sportRepo repositories.SportRepository) SportService {
	return &sportService{
		sportRepo: sportRepo,
	}
}

func (s *sportService) CreateSport(ctx context.Context, input CreateSportInput) (*models.Sport, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrSportNameRequired
	}

	sport := &models.Sport{
		Name: name,
	}

	err := s.sportRepo.Create(ctx, sport)
	if err != nil {
		if errors.Is(err, repositories.ErrSportNameConflict) {
			return nil, ErrSportNameConflict
		}
		return nil, fmt.Errorf("%w: %w", ErrSportCreationFailed, err)
	}

	return sport, nil
}

func (s *sportService) GetSportByID(ctx context.Context, id int) (*models.Sport, error) {
	sport, err := s.sportRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrSportNotFound) {
			return nil, ErrSportNotFound
		}
		return nil, fmt.Errorf("failed to get sport by id %d: %w", id, err)
	}
	return sport, nil
}

func (s *sportService) GetAllSports(ctx context.Context) ([]models.Sport, error) {
	sports, err := s.sportRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all sports: %w", err)
	}
	if sports == nil {
		return []models.Sport{}, nil
	}
	return sports, nil
}

func (s *sportService) UpdateSport(ctx context.Context, id int, input UpdateSportInput) (*models.Sport, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrSportNameRequired
	}

	sportToUpdate := &models.Sport{
		ID:   id,
		Name: name,
	}

	err := s.sportRepo.Update(ctx, sportToUpdate)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrSportNotFound):
			return nil, ErrSportNotFound
		case errors.Is(err, repositories.ErrSportNameConflict):
			return nil, ErrSportNameConflict
		default:
			return nil, fmt.Errorf("%w (id: %d): %w", ErrSportUpdateFailed, id, err)
		}
	}

	return sportToUpdate, nil
}

func (s *sportService) DeleteSport(ctx context.Context, id int) error {
	err := s.sportRepo.Delete(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrSportNotFound):
			return ErrSportNotFound
		case errors.Is(err, repositories.ErrSportInUse):
			return ErrSportInUse
		default:
			return fmt.Errorf("%w (id: %d): %w", ErrSportDeleteFailed, id, err)
		}
	}
	return nil
}
