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
	ErrTournamentNameRequired  = errors.New("tournament name is required")
	ErrTournamentDatesRequired = errors.New("registration, start, and end dates are required")

	ErrTournamentSportNotFound     = errors.New("specified sport not found")
	ErrTournamentFormatNotFound    = errors.New("specified format not found")
	ErrTournamentOrganizerNotFound = errors.New("specified organizer user not found")

	ErrTournamentCreationFailed     = errors.New("failed to create tournament")
	ErrTournamentUpdateFailed       = errors.New("failed to update tournament")
	ErrTournamentDeleteFailed       = errors.New("failed to delete tournament")
	ErrTournamentListFailed         = errors.New("failed to list tournaments")
	ErrTournamentUpdateNotAllowed   = errors.New("tournament update not allowed in current status") // Добавлено
	ErrTournamentDeletionNotAllowed = errors.New("tournament deletion not allowed")                 // Переименовано/уточнено

	ErrTournamentInUse = repositories.ErrTournamentInUse
)

type TournamentService interface {
	CreateTournament(ctx context.Context, organizerID int, input CreateTournamentInput) (*models.Tournament, error)
	GetTournamentByID(ctx context.Context, id int) (*models.Tournament, error)
	ListTournaments(ctx context.Context, filter ListTournamentsFilter) ([]models.Tournament, error)
	UpdateTournamentDetails(ctx context.Context, id int, currentUserID int, input UpdateTournamentDetailsInput) (*models.Tournament, error)
	UpdateTournamentStatus(ctx context.Context, id int, currentUserID int, status models.TournamentStatus) (*models.Tournament, error)
	DeleteTournament(ctx context.Context, id int, currentUserID int) error
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
	// SportID, FormatID, OrganizerID не обновляются этим методом
}

// Используется и для сервиса, и для репозитория
type ListTournamentsFilter struct {
	SportID     *int                     `json:"sport_id"`
	FormatID    *int                     `json:"format_id"`
	OrganizerID *int                     `json:"organizer_id"`
	Status      *models.TournamentStatus `json:"status"`
	Limit       int                      `json:"limit"`
	Offset      int                      `json:"offset"`
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

func (s *tournamentService) CreateTournament(ctx context.Context, organizerID int, input CreateTournamentInput) (*models.Tournament, error) {
	// 1. Валидация ввода
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

	// 2. Валидация внешних сущностей
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
	// Проверяем переданный organizerID
	_, validationErr = s.userRepo.GetByID(ctx, organizerID)
	if validationErr != nil {
		if errors.Is(validationErr, repositories.ErrUserNotFound) {
			return nil, ErrTournamentOrganizerNotFound
		}
		return nil, fmt.Errorf("failed to verify organizer %d: %w", organizerID, validationErr)
	}

	// 3. Создание модели
	tournament := &models.Tournament{
		Name:            name,
		Description:     input.Description,
		SportID:         input.SportID,
		FormatID:        input.FormatID,
		OrganizerID:     organizerID, // Используем параметр метода
		RegDate:         input.RegDate,
		StartDate:       input.StartDate,
		EndDate:         input.EndDate,
		Location:        input.Location,
		MaxParticipants: input.MaxParticipants,
		Status:          models.StatusSoon, // Устанавливаем начальный статус
	}

	// 4. Вызов репозитория
	err := s.tournamentRepo.Create(ctx, tournament)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNameConflict) {
			return nil, ErrTournamentNameConflict
		}
		// Обработка FK ошибок от БД (маловероятно, т.к. проверили выше, но для полноты)
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
	// Опционально: загрузить связанные сущности (Sport, Format, Organizer)
	return tournament, nil
}

func (s *tournamentService) ListTournaments(ctx context.Context, filter ListTournamentsFilter) ([]models.Tournament, error) {
	// Передаем фильтр напрямую, т.к. структура идентична
	tournaments, err := s.tournamentRepo.List(ctx, repositories.ListTournamentsFilter(filter))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTournamentListFailed, err)
	}
	// Репозиторий должен возвращать пустой слайс, а не nil, но на всякий случай проверим
	if tournaments == nil {
		return []models.Tournament{}, nil
	}
	// Опционально: обогатить турниры связанными данными (Sport, Format...)
	return tournaments, nil
}

// Добавлен параметр currentUserID для авторизации
func (s *tournamentService) UpdateTournamentDetails(ctx context.Context, id int, currentUserID int, input UpdateTournamentDetailsInput) (*models.Tournament, error) {
	// 1. Получаем турнир
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("failed to get tournament %d for update: %w", id, err)
	}

	// 2. Авторизация
	if tournament.OrganizerID != currentUserID {
		return nil, ErrForbiddenOperation
	}

	// 3. Проверка статуса (можно ли обновлять?)
	if tournament.Status != models.StatusSoon && tournament.Status != models.StatusRegistration {
		// Разрешаем обновление только до начала активной фазы
		return nil, fmt.Errorf("%w: cannot update tournament in status '%s'", ErrTournamentUpdateNotAllowed, tournament.Status)
	}

	updated := false
	// 4. Применение изменений
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
	// Сравнение указателей Description
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
	// Сравнение указателей Location
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

	// 5. Перепроверка дат, если они менялись
	if updated { // Проверяем только если были изменения
		if err := validateTournamentDates(tournament.RegDate, tournament.StartDate, tournament.EndDate); err != nil {
			return nil, err // Используем ошибки из validateTournamentDates
		}
	}

	// 6. Если не было изменений
	if !updated {
		return tournament, nil
	}

	// 7. Вызов репозитория
	err = s.tournamentRepo.Update(ctx, tournament)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNameConflict) {
			return nil, ErrTournamentNameConflict
		}
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return nil, ErrTournamentNotFound
		} // Удален между Get и Update?
		// Обработка FK ошибок (если бы SportID/FormatID/OrganizerID обновлялись)
		return nil, fmt.Errorf("%w: %w", ErrTournamentUpdateFailed, err)
	}

	return tournament, nil
}

// Добавлен параметр currentUserID для авторизации
func (s *tournamentService) UpdateTournamentStatus(ctx context.Context, id int, currentUserID int, newStatus models.TournamentStatus) (*models.Tournament, error) {
	// 1. Валидация значения статуса
	if !isValidTournamentStatus(newStatus) {
		return nil, fmt.Errorf("%w: '%s'", ErrTournamentInvalidStatus, newStatus)
	}

	// 2. Получаем турнир
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("failed to get tournament %d for status update: %w", id, err)
	}

	// 3. Авторизация
	if tournament.OrganizerID != currentUserID {
		return nil, ErrForbiddenOperation
	}

	// 4. Валидация перехода
	if !isValidStatusTransition(tournament.Status, newStatus) {
		return nil, fmt.Errorf("%w: from '%s' to '%s'", ErrTournamentInvalidStatusTransition, tournament.Status, newStatus)
	}

	// 5. Вызов репозитория
	err = s.tournamentRepo.UpdateStatus(ctx, id, newStatus)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return nil, ErrTournamentNotFound
		} // Удален?
		// UpdateStatus обычно не вызывает конфликтов или FK ошибок
		return nil, fmt.Errorf("%w: failed to update status in repository: %w", ErrTournamentUpdateFailed, err)
	}

	// 6. Обновляем статус в объекте и возвращаем
	tournament.Status = newStatus
	return tournament, nil
}

// Добавлен параметр currentUserID для авторизации
func (s *tournamentService) DeleteTournament(ctx context.Context, id int, currentUserID int) error {
	// 1. Получаем турнир для проверок
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return ErrTournamentNotFound
		} // Уже удален
		return fmt.Errorf("failed to get tournament %d for deletion check: %w", id, err)
	}

	// 2. Авторизация
	if tournament.OrganizerID != currentUserID {
		return ErrForbiddenOperation
	}

	if tournament.Status != models.StatusSoon && tournament.Status != models.StatusRegistration && tournament.Status != models.StatusCanceled {
		return fmt.Errorf("%w: cannot delete tournament with status '%s'", ErrTournamentDeletionNotAllowed, tournament.Status)
	}

	// 4. Вызов репозитория
	err = s.tournamentRepo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return ErrTournamentNotFound
		} // Уже удален
		// Обрабатываем ошибку FK constraint violation (ErrTournamentInUse)
		if errors.Is(err, repositories.ErrTournamentInUse) {
			// Можно вернуть более специфичную ошибку, если нужно знать причину (участники или матчи)
			return fmt.Errorf("%w: tournament might have participants or matches", ErrTournamentDeletionNotAllowed)
		}
		return fmt.Errorf("%w: %w", ErrTournamentDeleteFailed, err)
	}

	return nil
}

func isValidTournamentStatus(status models.TournamentStatus) bool {
	switch status {
	case models.StatusSoon, models.StatusRegistration, models.StatusActive, models.StatusCompleted, models.StatusCanceled:
		return true
	default:
		return false
	}
}

// isValidStatusTransition определяет допустимые переходы между статусами
func isValidStatusTransition(current, next models.TournamentStatus) bool {
	if current == next {
		return true // Разрешаем "обновление" на тот же статус
	}

	// Карта разрешенных переходов
	allowedTransitions := map[models.TournamentStatus][]models.TournamentStatus{
		models.StatusSoon:         {models.StatusRegistration, models.StatusCanceled},
		models.StatusRegistration: {models.StatusActive, models.StatusCanceled}, // Можно добавить Soon?
		models.StatusActive:       {models.StatusCompleted, models.StatusCanceled},
		models.StatusCompleted:    {}, // Нельзя выйти из Completed
		models.StatusCanceled:     {}, // Нельзя выйти из Canceled
	}

	allowed, ok := allowedTransitions[current]
	if !ok {
		return false // Неизвестный текущий статус
	}
	for _, nextAllowed := range allowed {
		if next == nextAllowed {
			return true
		}
	}
	return false // Переход не найден в списке разрешенных
}

func validateTournamentDates(reg, start, end time.Time) error {
	if reg.IsZero() || start.IsZero() || end.IsZero() {
		return ErrTournamentDatesRequired
	}
	if reg.After(start) { // reg > start
		return fmt.Errorf("%w: registration date (%s) cannot be after start date (%s)", ErrTournamentInvalidRegDate, reg.Format(time.DateOnly), start.Format(time.DateOnly))
	}
	if !start.Before(end) { // start >= end
		return fmt.Errorf("%w: start date (%s) must be before end date (%s)", ErrTournamentInvalidDateRange, start.Format(time.DateOnly), end.Format(time.DateOnly))
	}
	return nil
}

// derefString безопасно разыменовывает *string, возвращая "" если указатель nil
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
