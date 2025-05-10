package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
	"github.com/Dosada05/tournament-system/storage" // Для заполнения LogoURL у User/Team
)

var (
	ErrParticipantStatusUpdateFailed = errors.New("failed to update participant status")
	ErrParticipantCreationFailed     = errors.New("failed to create participant registration")
	ErrParticipantDeleteFailed       = errors.New("failed to delete participant registration")
	ErrParticipantListFailed         = errors.New("failed to list participants")
	ErrCancellationNotAllowed        = errors.New("registration cannot be cancelled in the current tournament state or by this user")
	ErrApplicationUpdateNotAllowed   = errors.New("application status cannot be updated in the current tournament state")
	ErrNotTournamentOrganizer        = errors.New("only the tournament organizer can manage applications")
	ErrInvalidParticipantStatus      = errors.New("invalid participant status provided for update")
)

type ParticipantService interface {
	RegisterSoloParticipant(ctx context.Context, userID, tournamentID, currentUserID int) (*models.Participant, error)
	RegisterTeamParticipant(ctx context.Context, teamID, tournamentID, currentUserID int) (*models.Participant, error)
	CancelRegistration(ctx context.Context, participantID, currentUserID int) error
	ListTournamentApplications(ctx context.Context, tournamentID int, currentUserID int, statusFilter *models.ParticipantStatus) ([]*models.Participant, error)
	UpdateApplicationStatus(ctx context.Context, participantID int, newStatus models.ParticipantStatus, currentUserID int) (*models.Participant, error)
}

type participantService struct {
	participantRepo repositories.ParticipantRepository
	tournamentRepo  repositories.TournamentRepository
	userRepo        repositories.UserRepository
	teamRepo        repositories.TeamRepository
	fileUploader    storage.FileUploader // Для заполнения URL логотипов User/Team
}

func NewParticipantService(
	participantRepo repositories.ParticipantRepository,
	tournamentRepo repositories.TournamentRepository,
	userRepo repositories.UserRepository,
	teamRepo repositories.TeamRepository,
	fileUploader storage.FileUploader,
) ParticipantService {
	return &participantService{
		participantRepo: participantRepo,
		tournamentRepo:  tournamentRepo,
		userRepo:        userRepo,
		teamRepo:        teamRepo,
		fileUploader:    fileUploader,
	}
}

func (s *participantService) populateParticipantDetails(p *models.Participant) {
	if p == nil {
		return
	}
	if p.User != nil && p.User.LogoKey != nil && *p.User.LogoKey != "" && s.fileUploader != nil {
		url := s.fileUploader.GetPublicURL(*p.User.LogoKey)
		if url != "" {
			p.User.LogoURL = &url
		}
	}
	if p.Team != nil && p.Team.LogoKey != nil && *p.Team.LogoKey != "" && s.fileUploader != nil {
		url := s.fileUploader.GetPublicURL(*p.Team.LogoKey)
		if url != "" {
			p.Team.LogoURL = &url
		}
	}
}

func (s *participantService) populateParticipantListDetails(participants []*models.Participant) {
	for _, p := range participants {
		s.populateParticipantDetails(p)
	}
}

func (s *participantService) RegisterSoloParticipant(ctx context.Context, userID, tournamentID, currentUserID int) (*models.Participant, error) {
	if userID != currentUserID {
		return nil, ErrForbiddenOperation
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrUserNotFound, "failed to get user %d", userID)
	}
	if user.TeamID != nil {
		return nil, ErrUserCannotRegisterSolo
	}

	tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament %d", tournamentID)
	}
	if tournament.Status != models.StatusRegistration {
		return nil, ErrRegistrationNotOpen
	}

	approvedParticipantsCount, err := s.countApprovedParticipants(ctx, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("failed to count approved participants for tournament %d: %w", tournamentID, err)
	}
	if approvedParticipantsCount >= tournament.MaxParticipants {
		return nil, ErrTournamentFull
	}

	_, err = s.participantRepo.FindByUserAndTournament(ctx, userID, tournamentID)
	if err == nil {
		return nil, ErrRegistrationConflict
	}
	if !errors.Is(err, repositories.ErrParticipantNotFound) {
		return nil, fmt.Errorf("failed to check existing registration for user %d in tournament %d: %w", userID, tournamentID, err)
	}

	participant := &models.Participant{
		UserID:       &userID,
		TournamentID: tournamentID,
		Status:       models.StatusApplicationSubmitted,
	}

	if err := s.participantRepo.Create(ctx, participant); err != nil {
		return nil, handleParticipantCreateError(err)
	}

	createdParticipant, err := s.participantRepo.GetWithDetails(ctx, participant.ID)
	if err != nil {
		return participant, nil // Возвращаем созданного, но без деталей, если GetWithDetails упал
	}
	s.populateParticipantDetails(createdParticipant)
	return createdParticipant, nil
}

func (s *participantService) RegisterTeamParticipant(ctx context.Context, teamID, tournamentID, currentUserID int) (*models.Participant, error) {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTeamNotFound, "failed to get team %d", teamID)
	}
	if team.CaptainID != currentUserID {
		return nil, ErrUserMustBeCaptain
	}

	tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament %d", tournamentID)
	}
	if tournament.Status != models.StatusRegistration {
		return nil, ErrRegistrationNotOpen
	}

	approvedParticipantsCount, err := s.countApprovedParticipants(ctx, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("failed to count approved participants for tournament %d: %w", tournamentID, err)
	}
	if approvedParticipantsCount >= tournament.MaxParticipants {
		return nil, ErrTournamentFull
	}

	_, err = s.participantRepo.FindByTeamAndTournament(ctx, teamID, tournamentID)
	if err == nil {
		return nil, ErrRegistrationConflict
	}
	if !errors.Is(err, repositories.ErrParticipantNotFound) {
		return nil, fmt.Errorf("failed to check existing registration for team %d in tournament %d: %w", teamID, tournamentID, err)
	}

	participant := &models.Participant{
		TeamID:       &teamID,
		TournamentID: tournamentID,
		Status:       models.StatusApplicationSubmitted,
	}

	if err := s.participantRepo.Create(ctx, participant); err != nil {
		return nil, handleParticipantCreateError(err)
	}

	createdParticipant, err := s.participantRepo.GetWithDetails(ctx, participant.ID)
	if err != nil {
		return participant, nil
	}
	s.populateParticipantDetails(createdParticipant)
	return createdParticipant, nil
}

func (s *participantService) CancelRegistration(ctx context.Context, participantID, currentUserID int) error {
	participant, err := s.participantRepo.FindByID(ctx, participantID)
	if err != nil {
		return handleRepositoryError(err, ErrParticipantNotFound, "failed to get participant %d for cancellation", participantID)
	}

	tournament, err := s.tournamentRepo.GetByID(ctx, participant.TournamentID)
	if err != nil && !errors.Is(err, repositories.ErrTournamentNotFound) {
		return fmt.Errorf("failed to get tournament %d for cancellation check: %w", participant.TournamentID, err)
	}

	if tournament != nil && (tournament.Status == models.StatusActive || tournament.Status == models.StatusCompleted) {
		return ErrCancellationNotAllowed
	}
	// Разрешаем отмену, если статус турнира Soon, Registration или Canceled, или если турнир не найден (мог быть удален)

	canCancel := false
	if participant.UserID != nil && *participant.UserID == currentUserID {
		canCancel = true
	} else if participant.TeamID != nil {
		team, teamErr := s.teamRepo.GetByID(ctx, *participant.TeamID)
		if teamErr != nil && !errors.Is(teamErr, repositories.ErrTeamNotFound) {
			return fmt.Errorf("failed to get team %d for cancellation check: %w", *participant.TeamID, teamErr)
		}
		if team != nil && team.CaptainID == currentUserID {
			canCancel = true
		}
		// Если команда не найдена, но TeamID у участника есть, и это капитан, то отменять нельзя, т.к. команда не его
	}

	if !canCancel {
		return ErrForbiddenOperation
	}

	if err := s.participantRepo.Delete(ctx, participantID); err != nil {
		return handleRepositoryError(err, ErrParticipantNotFound, "failed to delete participant registration %d", participantID)
	}
	return nil
}

func (s *participantService) ListTournamentApplications(ctx context.Context, tournamentID int, currentUserID int, statusFilter *models.ParticipantStatus) ([]*models.Participant, error) {
	tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament %d for listing applications", tournamentID)
	}
	if tournament.OrganizerID != currentUserID {
		// Обычный пользователь может видеть список подтвержденных участников, если статус не ApplicationSubmitted
		if statusFilter != nil && *statusFilter == models.StatusApplicationSubmitted {
			return nil, ErrForbiddenOperation // Только организатор видит заявки
		}
		if statusFilter == nil { // Если фильтр не указан, не админ не должен видеть все подряд
			// По умолчанию показываем только подтвержденных участников, если не организатор
			confirmedStatus := models.StatusParticipant
			statusFilter = &confirmedStatus
		}
	}
	// Организатор может видеть все статусы или фильтровать

	participants, err := s.participantRepo.ListByTournament(ctx, tournamentID, statusFilter, true) // true - для загрузки User/Team
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrParticipantListFailed, err)
	}
	if participants == nil {
		return []*models.Participant{}, nil
	}
	s.populateParticipantListDetails(participants)
	return participants, nil
}

func (s *participantService) UpdateApplicationStatus(ctx context.Context, participantID int, newStatus models.ParticipantStatus, currentUserID int) (*models.Participant, error) {
	if newStatus != models.StatusParticipant && newStatus != models.StatusApplicationRejected {
		return nil, fmt.Errorf("%w: can only change to '%s' or '%s'", ErrInvalidParticipantStatus, models.StatusParticipant, models.StatusApplicationRejected)
	}

	participant, err := s.participantRepo.FindByID(ctx, participantID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrParticipantNotFound, "failed to get participant %d for status update", participantID)
	}

	tournament, err := s.tournamentRepo.GetByID(ctx, participant.TournamentID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament %d for status update", participant.TournamentID)
	}

	if tournament.OrganizerID != currentUserID {
		return nil, ErrNotTournamentOrganizer
	}

	if tournament.Status != models.StatusRegistration && tournament.Status != models.StatusSoon {
		// Разрешаем менять статус заявок, пока турнир не активен
		return nil, fmt.Errorf("%w: current tournament status is '%s'", ErrApplicationUpdateNotAllowed, tournament.Status)
	}

	if participant.Status != models.StatusApplicationSubmitted {
		return nil, fmt.Errorf("%w: can only update applications with status '%s', current is '%s'",
			ErrApplicationUpdateNotAllowed, models.StatusApplicationSubmitted, participant.Status)
	}

	// Проверка на максимальное количество участников при одобрении
	if newStatus == models.StatusParticipant {
		approvedParticipantsCount, err := s.countApprovedParticipants(ctx, tournament.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to count approved participants for tournament %d: %w", tournament.ID, err)
		}
		if approvedParticipantsCount >= tournament.MaxParticipants {
			return nil, ErrTournamentFull
		}
	}

	if err := s.participantRepo.UpdateStatus(ctx, participantID, newStatus); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrParticipantStatusUpdateFailed, err)
	}

	participant.Status = newStatus // Обновляем статус в объекте для возврата

	updatedParticipant, err := s.participantRepo.GetWithDetails(ctx, participantID)
	if err != nil {
		// Если не удалось получить детали, возвращаем то, что есть, но с обновленным статусом
		s.populateParticipantDetails(participant) // Попытка заполнить старые детали, если они были
		return participant, nil
	}
	s.populateParticipantDetails(updatedParticipant)
	return updatedParticipant, nil
}

// Вспомогательная функция для подсчета одобренных участников
func (s *participantService) countApprovedParticipants(ctx context.Context, tournamentID int) (int, error) {
	statusParticipant := models.StatusParticipant
	approvedParticipants, err := s.participantRepo.ListByTournament(ctx, tournamentID, &statusParticipant, false)
	if err != nil {
		return 0, err
	}
	return len(approvedParticipants), nil
}

// Вспомогательные функции для обработки ошибок
func handleRepositoryError(err error, notFoundError error, format string, args ...interface{}) error {
	if errors.Is(err, repositories.ErrParticipantNotFound) || (notFoundError != nil && errors.Is(err, notFoundError)) {
		return notFoundError
	}
	return fmt.Errorf(format+": %w", append(args, err)...)
}

func handleParticipantCreateError(err error) error {
	switch {
	case errors.Is(err, repositories.ErrParticipantConflict):
		return ErrRegistrationConflict
	case errors.Is(err, repositories.ErrParticipantUserInvalid):
		return ErrUserNotFound
	case errors.Is(err, repositories.ErrParticipantTeamInvalid):
		return ErrTeamNotFound
	case errors.Is(err, repositories.ErrParticipantTournamentInvalid):
		return ErrTournamentNotFound
	case errors.Is(err, repositories.ErrParticipantTypeViolation):
		return fmt.Errorf("internal error: participant type violation on create: %w", err)
	default:
		return fmt.Errorf("%w: %w", ErrParticipantCreationFailed, err)
	}
}
