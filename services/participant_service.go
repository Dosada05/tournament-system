package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

var (
	ErrCancellationNotAllowed        = errors.New("registration cannot be cancelled in the current tournament state")
	ErrParticipantStatusUpdateFailed = errors.New("failed to update participant status")
	ErrParticipantCreationFailed     = errors.New("failed to create participant registration")
	ErrParticipantDeleteFailed       = errors.New("failed to delete participant registration")
	ErrParticipantListFailed         = errors.New("failed to list participants")
)

type ParticipantService interface {
	RegisterSoloParticipant(ctx context.Context, userID, tournamentID, currentUserID int) (*models.Participant, error)
	RegisterTeamParticipant(ctx context.Context, teamID, tournamentID, currentUserID int) (*models.Participant, error)
	CancelRegistration(ctx context.Context, participantID, currentUserID int) error
	ListParticipants(ctx context.Context, tournamentID int, statusFilter *models.ParticipantStatus) ([]*models.Participant, error)
	// Возможно, понадобятся методы для изменения статуса участника администратором
	// UpdateParticipantStatus(ctx context.Context, participantID int, newStatus models.ParticipantStatus, currentUserID int) error
}

type participantService struct {
	participantRepo repositories.ParticipantRepository
	tournamentRepo  repositories.TournamentRepository
	userRepo        repositories.UserRepository
	teamRepo        repositories.TeamRepository
}

func NewParticipantService(
	participantRepo repositories.ParticipantRepository,
	tournamentRepo repositories.TournamentRepository,
	userRepo repositories.UserRepository,
	teamRepo repositories.TeamRepository,
) ParticipantService {
	return &participantService{
		participantRepo: participantRepo,
		tournamentRepo:  tournamentRepo,
		userRepo:        userRepo,
		teamRepo:        teamRepo,
	}
}

func (s *participantService) RegisterSoloParticipant(ctx context.Context, userID, tournamentID, currentUserID int) (*models.Participant, error) {
	if userID != currentUserID {
		return nil, ErrForbiddenOperation // Пользователь может регистрировать только себя
	}

	tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("failed to get tournament %d: %w", tournamentID, err)
	}

	if tournament.Status != models.StatusRegistration {
		return nil, ErrRegistrationNotOpen
	}

	// Проверка вместимости (упрощенная, без учета статуса участников)
	// Для точной проверки нужен подсчет подтвержденных участников
	participants, err := s.participantRepo.ListByTournament(ctx, tournamentID, nil) // Получаем всех
	if err != nil {
		return nil, fmt.Errorf("failed to check tournament capacity for tournament %d: %w", tournamentID, err)
	}
	if len(participants) >= tournament.MaxParticipants {
		return nil, ErrTournamentFull
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user %d: %w", userID, err)
	}

	if user.TeamID != nil {
		return nil, ErrUserCannotRegisterSolo
	}

	// Проверка на существующую регистрацию этого пользователя
	_, err = s.participantRepo.FindByUserAndTournament(ctx, userID, tournamentID)
	if err == nil {
		return nil, ErrRegistrationConflict // Уже зарегистрирован
	}
	if !errors.Is(err, repositories.ErrParticipantNotFound) {
		// Неожиданная ошибка при проверке
		return nil, fmt.Errorf("failed to check existing registration for user %d in tournament %d: %w", userID, tournamentID, err)
	}

	participant := &models.Participant{
		UserID:       &userID,
		TeamID:       nil,
		TournamentID: tournamentID,
		Status:       models.StatusApplicationSubmitted,
	}

	err = s.participantRepo.Create(ctx, participant)
	if err != nil {
		if errors.Is(err, repositories.ErrParticipantConflict) {
			return nil, ErrRegistrationConflict // На случай гонки состояний
		}
		// Преобразование других ошибок репозитория
		if errors.Is(err, repositories.ErrParticipantUserInvalid) {
			return nil, ErrUserNotFound
		}
		if errors.Is(err, repositories.ErrParticipantTournamentInvalid) {
			return nil, ErrTournamentNotFound
		}
		if errors.Is(err, repositories.ErrParticipantTypeViolation) {
			return nil, fmt.Errorf("internal error: participant type violation on create: %w", err)
		}

		return nil, fmt.Errorf("%w: %w", ErrParticipantCreationFailed, err)
	}

	return participant, nil
}

func (s *participantService) RegisterTeamParticipant(ctx context.Context, teamID, tournamentID, currentUserID int) (*models.Participant, error) {
	tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("failed to get tournament %d: %w", tournamentID, err)
	}

	if tournament.Status != models.StatusRegistration {
		return nil, ErrRegistrationNotOpen
	}

	// Проверка вместимости
	participants, err := s.participantRepo.ListByTournament(ctx, tournamentID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check tournament capacity for tournament %d: %w", tournamentID, err)
	}
	if len(participants) >= tournament.MaxParticipants {
		return nil, ErrTournamentFull
	}

	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		if errors.Is(err, repositories.ErrTeamNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("failed to get team %d: %w", teamID, err)
	}

	if team.CaptainID != currentUserID {
		return nil, ErrUserMustBeCaptain
	}

	// Проверка на существующую регистрацию этой команды
	_, err = s.participantRepo.FindByTeamAndTournament(ctx, teamID, tournamentID)
	if err == nil {
		return nil, ErrRegistrationConflict // Команда уже зарегистрирована
	}
	if !errors.Is(err, repositories.ErrParticipantNotFound) {
		return nil, fmt.Errorf("failed to check existing registration for team %d in tournament %d: %w", teamID, tournamentID, err)
	}

	participant := &models.Participant{
		UserID:       nil,
		TeamID:       &teamID,
		TournamentID: tournamentID,
		Status:       models.StatusApplicationSubmitted, // Или StatusPending
	}

	err = s.participantRepo.Create(ctx, participant)
	if err != nil {
		if errors.Is(err, repositories.ErrParticipantConflict) {
			return nil, ErrRegistrationConflict
		}
		if errors.Is(err, repositories.ErrParticipantTeamInvalid) {
			return nil, ErrTeamNotFound
		}
		if errors.Is(err, repositories.ErrParticipantTournamentInvalid) {
			return nil, ErrTournamentNotFound
		}
		if errors.Is(err, repositories.ErrParticipantTypeViolation) {
			return nil, fmt.Errorf("internal error: participant type violation on create: %w", err)
		}

		return nil, fmt.Errorf("%w: %w", ErrParticipantCreationFailed, err)
	}

	return participant, nil
}

func (s *participantService) CancelRegistration(ctx context.Context, participantID, currentUserID int) error {
	participant, err := s.participantRepo.FindByID(ctx, participantID)
	if err != nil {
		if errors.Is(err, repositories.ErrParticipantNotFound) {
			return ErrParticipantNotFound
		}
		return fmt.Errorf("failed to get participant %d: %w", participantID, err)
	}

	tournament, err := s.tournamentRepo.GetByID(ctx, participant.TournamentID)
	if err != nil {
		// Если турнир не найден, отмена все равно возможна, но логируем предупреждение
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			fmt.Printf("Warning: Tournament %d not found when cancelling participant %d\n", participant.TournamentID, participantID)
		} else {
			return fmt.Errorf("failed to get tournament %d for cancellation check: %w", participant.TournamentID, err)
		}
	} else {
		// Проверка статуса турнира: нельзя отменить, если он идет или завершен
		if tournament.Status == models.StatusActive || tournament.Status == models.StatusCompleted {
			return ErrCancellationNotAllowed
		}
	}

	// Авторизация: пользователь отменяет свою регистрацию ИЛИ капитан отменяет регистрацию команды
	canCancel := false
	if participant.UserID != nil && *participant.UserID == currentUserID {
		canCancel = true // Пользователь отменяет свою регистрацию
	} else if participant.TeamID != nil {
		team, err := s.teamRepo.GetByID(ctx, *participant.TeamID)
		if err != nil {
			// Если команда не найдена, возможно, стоит разрешить отмену, но с предупреждением
			if errors.Is(err, repositories.ErrTeamNotFound) {
				fmt.Printf("Warning: Team %d not found when checking captain for cancelling participant %d\n", *participant.TeamID, participantID)
				// Можно разрешить отмену, если participant.TeamID не nil, но команды нет
				// canCancel = true // Зависит от бизнес-логики
			} else {
				return fmt.Errorf("failed to get team %d for cancellation check: %w", *participant.TeamID, err)
			}
		} else if team.CaptainID == currentUserID {
			canCancel = true // Капитан отменяет регистрацию команды
		}
	}

	if !canCancel {
		return ErrForbiddenOperation // Не авторизован
	}

	// Удаляем запись участника
	err = s.participantRepo.Delete(ctx, participantID)
	if err != nil {
		if errors.Is(err, repositories.ErrParticipantNotFound) {
			return ErrParticipantNotFound // Маловероятно после FindByID
		}
		return fmt.Errorf("%w: %w", ErrParticipantDeleteFailed, err)
	}

	return nil
}

func (s *participantService) ListParticipants(ctx context.Context, tournamentID int, statusFilter *models.ParticipantStatus) ([]*models.Participant, error) {
	// Проверка существования турнира (опционально, но полезно)
	if _, err := s.tournamentRepo.GetByID(ctx, tournamentID); err != nil {
		if errors.Is(err, repositories.ErrTournamentNotFound) {
			return nil, ErrTournamentNotFound
		}
		return nil, fmt.Errorf("failed to verify tournament %d: %w", tournamentID, err)
	}

	participants, err := s.participantRepo.ListByTournament(ctx, tournamentID, statusFilter)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrParticipantListFailed, err)
	}

	// Важно: Возвращаем пустой слайс, а не nil, если участников нет
	if participants == nil {
		return []*models.Participant{}, nil
	}

	return participants, nil
}

// Пример возможной реализации UpdateParticipantStatus (для админа)
// func (s *participantService) UpdateParticipantStatus(ctx context.Context, participantID int, newStatus models.ParticipantStatus, currentUserID int) error {
// 	// 1. Проверить права currentUserID (должен быть админ или организатор турнира)
// 	// 2. Получить участника participantRepo.FindByID
// 	// 3. Проверить допустимость смены статуса (isValidStatusTransition для участников?)
// 	// 4. Вызвать participantRepo.UpdateStatus
// 	// 5. Обработать ошибки
// 	return errors.New("not implemented")
// }
