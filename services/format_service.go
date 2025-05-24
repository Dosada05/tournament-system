package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

var (
	ErrFormatNameRequired        = errors.New("format name is required")
	ErrFormatNameConflict        = errors.New("format name already exists")
	ErrFormatInUse               = errors.New("format cannot be deleted as it is currently in use")
	ErrFormatCreationFailed      = errors.New("failed to create format")
	ErrFormatUpdateFailed        = errors.New("failed to update format")
	ErrFormatDeleteFailed        = errors.New("failed to delete format")
	ErrInvalidBracketType        = errors.New("invalid bracket type specified")
	ErrInvalidRoundRobinSettings = errors.New("invalid settings for RoundRobin format")
)

type FormatService interface {
	CreateFormat(ctx context.Context, input CreateFormatInput) (*models.Format, error)
	GetFormatByID(ctx context.Context, id int) (*models.Format, error)
	GetAllFormats(ctx context.Context) ([]models.Format, error)
	UpdateFormat(ctx context.Context, id int, input UpdateFormatInput) (*models.Format, error)
	DeleteFormat(ctx context.Context, id int) error
}

type CreateFormatInput struct {
	Name            string                       `json:"name" validate:"required"`
	BracketType     string                       `json:"bracket_type" validate:"required"` // e.g., "SingleElimination", "RoundRobin"
	ParticipantType models.FormatParticipantType `json:"participant_type" validate:"required,oneof=solo team"`
	SettingsJSON    json.RawMessage              `json:"settings_json,omitempty"` // json.RawMessage для гибкости, может быть nil
}

// Обновленная структура для обновления (все поля - указатели)
type UpdateFormatInput struct {
	Name            *string                       `json:"name,omitempty"`
	BracketType     *string                       `json:"bracket_type,omitempty"`
	ParticipantType *models.FormatParticipantType `json:"participant_type,omitempty" validate:"omitempty,oneof=solo team"`
	SettingsJSON    *json.RawMessage              `json:"settings_json,omitempty"` // Используем *json.RawMessage для определения, было ли поле передано
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

	if input.BracketType != "SingleElimination" && input.BracketType != "RoundRobin" {
		return nil, fmt.Errorf("%w: %s. Supported types are 'SingleElimination', 'RoundRobin'", ErrInvalidBracketType, input.BracketType)
	}

	var settingsStrPointer *string
	if len(input.SettingsJSON) > 0 && string(input.SettingsJSON) != "null" {
		parsedSettings := make(map[string]interface{})
		if err := json.Unmarshal(input.SettingsJSON, &parsedSettings); err != nil {
			return nil, fmt.Errorf("invalid settings_json format: %w", err)
		}

		if input.BracketType == "RoundRobin" {
			var rrSettings models.RoundRobinSettings
			if err := json.Unmarshal(input.SettingsJSON, &rrSettings); err != nil {
				return nil, fmt.Errorf("%w: could not parse RoundRobin settings: %v", ErrInvalidRoundRobinSettings, err)
			}
			if rrSettings.NumberOfRounds < 1 || rrSettings.NumberOfRounds > 2 { // Example validation
				return nil, fmt.Errorf("%w: NumberOfRounds must be 1 or 2, got %d", ErrInvalidRoundRobinSettings, rrSettings.NumberOfRounds)
			}
			// Re-marshal validated/defaulted settings if necessary, or just store the original valid JSON
			validJsonBytes, _ := json.Marshal(rrSettings) // Assuming rrSettings might have defaults applied
			s := string(validJsonBytes)
			settingsStrPointer = &s
		} else {
			// For other bracket types, just store the provided JSON if it's valid
			sJSON := string(input.SettingsJSON)
			settingsStrPointer = &sJSON
		}
	}

	format := &models.Format{
		Name:            name,
		BracketType:     input.BracketType,
		ParticipantType: input.ParticipantType,
		SettingsJSON:    settingsStrPointer,
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
	formatToUpdate, err := s.formatRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrFormatNotFound) {
			return nil, ErrFormatNotFound
		}
		return nil, fmt.Errorf("failed to get format %d for update: %w", id, err)
	}

	updated := false
	if input.Name != nil {
		trimmedName := strings.TrimSpace(*input.Name)
		if trimmedName == "" {
			return nil, ErrFormatNameRequired
		}
		if trimmedName != formatToUpdate.Name {
			formatToUpdate.Name = trimmedName
			updated = true
		}
	}

	currentBracketType := formatToUpdate.BracketType
	if input.BracketType != nil {
		if *input.BracketType != "SingleElimination" && *input.BracketType != "RoundRobin" {
			return nil, fmt.Errorf("%w: %s. Supported types are 'SingleElimination', 'RoundRobin'", ErrInvalidBracketType, *input.BracketType)
		}
		if *input.BracketType != formatToUpdate.BracketType {
			formatToUpdate.BracketType = *input.BracketType
			currentBracketType = *input.BracketType // Update for settings validation
			updated = true
		}
	}

	if input.ParticipantType != nil && *input.ParticipantType != formatToUpdate.ParticipantType {
		formatToUpdate.ParticipantType = *input.ParticipantType
		updated = true
	}

	if input.SettingsJSON != nil {
		if len(*input.SettingsJSON) == 0 || string(*input.SettingsJSON) == "null" {
			if formatToUpdate.SettingsJSON != nil {
				formatToUpdate.SettingsJSON = nil
				updated = true
			}
		} else {
			parsedSettings := make(map[string]interface{})
			if errJson := json.Unmarshal(*input.SettingsJSON, &parsedSettings); errJson != nil {
				return nil, fmt.Errorf("invalid settings_json format for update: %w", errJson)
			}

			newSettingsStr := string(*input.SettingsJSON)

			if currentBracketType == "RoundRobin" {
				var rrSettings models.RoundRobinSettings
				if errJson := json.Unmarshal(*input.SettingsJSON, &rrSettings); errJson != nil {
					return nil, fmt.Errorf("%w: could not parse RoundRobin settings for update: %v", ErrInvalidRoundRobinSettings, errJson)
				}
				if rrSettings.NumberOfRounds < 1 || rrSettings.NumberOfRounds > 2 {
					return nil, fmt.Errorf("%w: NumberOfRounds must be 1 or 2, got %d", ErrInvalidRoundRobinSettings, rrSettings.NumberOfRounds)
				}
				validJsonBytes, _ := json.Marshal(rrSettings)
				newSettingsStr = string(validJsonBytes)
			}

			if formatToUpdate.SettingsJSON == nil || *formatToUpdate.SettingsJSON != newSettingsStr {
				formatToUpdate.SettingsJSON = &newSettingsStr
				updated = true
			}
		}
	}

	if !updated {
		return formatToUpdate, nil
	}

	err = s.formatRepo.Update(ctx, formatToUpdate)
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
