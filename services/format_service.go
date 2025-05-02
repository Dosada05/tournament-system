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
	ErrFormatNameRequired   = errors.New("format name is required")
	ErrFormatNameConflict   = errors.New("format name already exists")
	ErrFormatInUse          = errors.New("format cannot be deleted as it is currently in use")
	ErrFormatCreationFailed = errors.New("failed to create format")
	ErrFormatUpdateFailed   = errors.New("failed to update format")
	ErrFormatDeleteFailed   = errors.New("failed to delete format")
)

type FormatService interface {
	CreateFormat(ctx context.Context, input CreateFormatInput) (*models.Format, error)
	GetFormatByID(ctx context.Context, id int) (*models.Format, error)
	GetAllFormats(ctx context.Context) ([]models.Format, error)
	UpdateFormat(ctx context.Context, id int, input UpdateFormatInput) (*models.Format, error)
	DeleteFormat(ctx context.Context, id int) error
}

type CreateFormatInput struct {
	Name string
}

type UpdateFormatInput struct {
	Name string
}

type formatService struct {
	formatRepo repositories.FormatRepository
}

func NewFormatService(formatRepo repositories.FormatRepository) FormatService {
	return &formatService{
		formatRepo: formatRepo,
	}
}

func (s *formatService) CreateFormat(ctx context.Context, input CreateFormatInput) (*models.Format, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrFormatNameRequired
	}

	format := &models.Format{
		Name: name,
	}

	err := s.formatRepo.Create(ctx, format)
	if err != nil {
		if errors.Is(err, repositories.ErrFormatNameConflict) {
			return nil, ErrFormatNameConflict
		}
		return nil, fmt.Errorf("%w: %w", ErrFormatCreationFailed, err)
	}

	return format, nil
}

func (s *formatService) GetFormatByID(ctx context.Context, id int) (*models.Format, error) {
	format, err := s.formatRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrFormatNotFound) {
			return nil, ErrFormatNotFound
		}
		return nil, fmt.Errorf("failed to get format by id %d: %w", id, err)
	}
	return format, nil
}

func (s *formatService) GetAllFormats(ctx context.Context) ([]models.Format, error) {
	formats, err := s.formatRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all formats: %w", err)
	}
	if formats == nil {
		return []models.Format{}, nil
	}
	return formats, nil
}

func (s *formatService) UpdateFormat(ctx context.Context, id int, input UpdateFormatInput) (*models.Format, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrFormatNameRequired
	}

	formatToUpdate := &models.Format{
		ID:   id,
		Name: name,
	}

	err := s.formatRepo.Update(ctx, formatToUpdate)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrFormatNotFound):
			return nil, ErrFormatNotFound
		case errors.Is(err, repositories.ErrFormatNameConflict):
			return nil, ErrFormatNameConflict
		default:
			return nil, fmt.Errorf("%w (id: %d): %w", ErrFormatUpdateFailed, id, err)
		}
	}

	return formatToUpdate, nil
}

func (s *formatService) DeleteFormat(ctx context.Context, id int) error {
	err := s.formatRepo.Delete(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrFormatNotFound):
			return ErrFormatNotFound
		case errors.Is(err, repositories.ErrFormatInUse):
			return ErrFormatInUse
		default:
			return fmt.Errorf("%w (id: %d): %w", ErrFormatDeleteFailed, id, err)
		}
	}
	return nil
}
