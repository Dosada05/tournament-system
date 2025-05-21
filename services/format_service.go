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
	// Добавьте валидацию для BracketType, ParticipantType, если необходимо

	var settingsStrPointer *string
	if len(input.SettingsJSON) > 0 && string(input.SettingsJSON) != "null" { // Проверяем, что JSON не пустой и не "null"
		// Проверка валидности JSON
		var js map[string]interface{}
		if err := json.Unmarshal(input.SettingsJSON, &js); err != nil {
			return nil, fmt.Errorf("invalid settings_json format: %w", err)
		}
		s := string(input.SettingsJSON)
		settingsStrPointer = &s
	}

	format := &models.Format{
		Name:            name,
		BracketType:     input.BracketType,
		ParticipantType: input.ParticipantType,
		SettingsJSON:    settingsStrPointer,
	}

	err := s.formatRepo.Create(ctx, format) // Репозиторий должен поддерживать новые поля
	if err != nil {
		if errors.Is(err, repositories.ErrFormatNameConflict) {
			return nil, ErrFormatNameConflict
		}
		return nil, fmt.Errorf("%w: %w", ErrFormatCreationFailed, err)
	}
	return format, nil
}

func (s *formatService) GetFormatByID(ctx context.Context, id int) (*models.Format, error) {
	// Этот метод остается без изменений, т.к. репозиторий уже возвращает все поля
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
	// Этот метод также остается без изменений
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
	if input.BracketType != nil && *input.BracketType != formatToUpdate.BracketType {
		// Добавить валидацию BracketType, если нужно
		formatToUpdate.BracketType = *input.BracketType
		updated = true
	}
	if input.ParticipantType != nil && *input.ParticipantType != formatToUpdate.ParticipantType {
		// Валидация ParticipantType уже есть в CHECK constraint БД, но можно и здесь
		formatToUpdate.ParticipantType = *input.ParticipantType
		updated = true
	}

	if input.SettingsJSON != nil { // Поле было передано
		if len(*input.SettingsJSON) == 0 || string(*input.SettingsJSON) == "null" {
			// Пользователь хочет очистить settings
			if formatToUpdate.SettingsJSON != nil { // Если было значение, а теперь нет
				formatToUpdate.SettingsJSON = nil
				updated = true
			}
		} else {
			// Проверка валидности JSON
			var js map[string]interface{}
			if errJson := json.Unmarshal(*input.SettingsJSON, &js); errJson != nil {
				return nil, fmt.Errorf("invalid settings_json format for update: %w", errJson)
			}
			newSettingsStr := string(*input.SettingsJSON)
			if formatToUpdate.SettingsJSON == nil || *formatToUpdate.SettingsJSON != newSettingsStr {
				formatToUpdate.SettingsJSON = &newSettingsStr
				updated = true
			}
		}
	}

	if !updated {
		return formatToUpdate, nil // Нет изменений
	}

	err = s.formatRepo.Update(ctx, formatToUpdate) // Репозиторий должен поддерживать обновление всех полей
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
	// Этот метод остается без изменений, логика проверки использования формата в репозитории
	err := s.formatRepo.Delete(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, repositories.ErrFormatNotFound):
			return ErrFormatNotFound
		case errors.Is(err, repositories.ErrFormatInUse): // Репозиторий должен возвращать эту ошибку
			return ErrFormatInUse
		default:
			return fmt.Errorf("%w (id: %d): %w", ErrFormatDeleteFailed, id, err)
		}
	}
	return nil
}
