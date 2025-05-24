package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
	"github.com/Dosada05/tournament-system/storage"
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

// ErrTournamentIncorrectFormatType означает, что тип участника турнира не соответствует ожидаемому для операции.
var ErrTournamentIncorrectFormatType = errors.New("tournament format participant type is incorrect for this operation")

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
	formatRepo      repositories.FormatRepository // Добавлена зависимость для загрузки формата
	fileUploader    storage.FileUploader
}

func NewParticipantService(
	participantRepo repositories.ParticipantRepository,
	tournamentRepo repositories.TournamentRepository,
	userRepo repositories.UserRepository,
	teamRepo repositories.TeamRepository,
	formatRepo repositories.FormatRepository, // Добавлен параметр
	fileUploader storage.FileUploader,
) ParticipantService {
	return &participantService{
		participantRepo: participantRepo,
		tournamentRepo:  tournamentRepo,
		userRepo:        userRepo,
		teamRepo:        teamRepo,
		formatRepo:      formatRepo, // Инициализация
		fileUploader:    fileUploader,
	}
}

func (s *participantService) populateParticipantDetails(p *models.Participant) {
	if p == nil || s.fileUploader == nil {
		return
	}
	if p.User != nil && p.User.LogoKey != nil && *p.User.LogoKey != "" {
		url := s.fileUploader.GetPublicURL(*p.User.LogoKey)
		if url != "" {
			p.User.LogoURL = &url
		}
	}
	if p.Team != nil && p.Team.LogoKey != nil && *p.Team.LogoKey != "" {
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
		return nil, ErrForbiddenOperation // Используем общую ошибку
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrUserNotFound, "failed to get user %d for solo registration", userID)
	}

	tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament %d for solo registration", tournamentID)
	}

	if tournament.OrganizerID == currentUserID { // currentUserID здесь это userID регистрирующегося
		return nil, ErrOrganizerCannotParticipate
	}

	// Загружаем формат турнира, чтобы проверить ParticipantType
	if tournament.Format == nil && tournament.FormatID > 0 {
		format, formatErr := s.formatRepo.GetByID(ctx, tournament.FormatID)
		if formatErr != nil {
			return nil, handleRepositoryError(formatErr, ErrFormatNotFound, "failed to load format %d for tournament %d", tournament.FormatID, tournamentID)
		}
		tournament.Format = format
	}
	if tournament.Format == nil {
		return nil, fmt.Errorf("tournament format information is missing for tournament ID %d", tournamentID)
	}

	// Проверяем, что это действительно solo-турнир
	if tournament.Format.ParticipantType != models.FormatParticipantSolo {
		return nil, fmt.Errorf("%w: expected solo tournament, got %s", ErrTournamentIncorrectFormatType, tournament.Format.ParticipantType)
	}

	// Теперь применяем правило: если турнир solo, игрок не должен состоять в команде
	if user.TeamID != nil {
		// Загрузим команду, чтобы убедиться, что TeamID действительно валидный и пользователь в команде
		// Это также поможет избежать ситуации, когда TeamID остался "висеть" после удаления команды.
		_, teamErr := s.teamRepo.GetByID(ctx, *user.TeamID)
		if teamErr == nil { // Если команда существует, значит пользователь действительно в команде
			return nil, ErrUserCannotRegisterSolo // Ошибка из services/errors.go
		}
		// Если команда не найдена (repos.ErrTeamNotFound), это может означать, что TeamID у пользователя "устарел".
		// В этом случае можно разрешить регистрацию, но лучше бы иметь механизм очистки таких TeamID.
		// Пока что, если команда не найдена, считаем, что пользователь не в активной команде.
		if !errors.Is(teamErr, repositories.ErrTeamNotFound) {
			// Если ошибка не "не найдено", то это другая проблема с БД
			return nil, fmt.Errorf("failed to verify user's team %d: %w", *user.TeamID, teamErr)
		}
		// Если команда не найдена, то пользователь как бы "свободен", даже если TeamID есть.
		// Это поведение можно обсудить. Для строгости можно было бы запретить, если TeamID != nil.
		// Но текущая ошибка ErrUserCannotRegisterSolo подразумевает, что он именно "не может", если в команде.
	}

	if tournament.Status != models.StatusRegistration {
		return nil, ErrRegistrationNotOpen // Используем общую ошибку
	}

	approvedParticipantsCount, err := s.countApprovedParticipants(ctx, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("failed to count approved participants for tournament %d: %w", tournamentID, err)
	}
	if approvedParticipantsCount >= tournament.MaxParticipants {
		return nil, ErrTournamentFull // Используем общую ошибку
	}

	// Проверка, не зарегистрирован ли уже пользователь на этот турнир
	_, err = s.participantRepo.FindByUserAndTournament(ctx, userID, tournamentID)
	if err == nil { // Если ошибки нет, значит участник найден -> конфликт
		return nil, ErrRegistrationConflict // Используем общую ошибку
	}
	// Убедимся, что ошибка была именно "не найдено", а не другая проблема с БД
	if !errors.Is(err, repositories.ErrParticipantNotFound) {
		return nil, fmt.Errorf("failed to check existing registration for user %d in tournament %d: %w", userID, tournamentID, err)
	}

	participant := &models.Participant{
		UserID:       &userID,
		TournamentID: tournamentID,
		Status:       models.StatusApplicationSubmitted,
	}

	if err := s.participantRepo.Create(ctx, participant); err != nil {
		return nil, handleParticipantCreateError(err) // Используем наш хелпер
	}

	// Получаем созданную заявку с деталями (User/Team)
	createdParticipant, err := s.participantRepo.GetWithDetails(ctx, participant.ID)
	if err != nil {
		fmt.Printf("Warning: failed to get participant details after creation for participant ID %d: %v\n", participant.ID, err)
		s.populateParticipantDetails(participant) // Попытка заполнить лого, если User/Team были в исходном participant
		return participant, nil
	}
	s.populateParticipantDetails(createdParticipant)
	return createdParticipant, nil
}

// ... (остальные методы RegisterTeamParticipant, CancelRegistration, ListTournamentApplications, UpdateApplicationStatus, countApprovedParticipants, handleRepositoryError, handleParticipantCreateError без изменений) ...

func (s *participantService) RegisterTeamParticipant(ctx context.Context, teamID, tournamentID, currentUserID int) (*models.Participant, error) {
	team, err := s.teamRepo.GetByID(ctx, teamID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTeamNotFound, "failed to get team %d for team registration", teamID)
	}
	if team.CaptainID != currentUserID {
		return nil, ErrUserMustBeCaptain
	}

	tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament %d for team registration", tournamentID)
	}

	if tournament.OrganizerID == currentUserID {
		return nil, ErrOrganizerCannotParticipate
	}

	// Загружаем формат турнира, чтобы проверить ParticipantType
	if tournament.Format == nil && tournament.FormatID > 0 {
		format, formatErr := s.formatRepo.GetByID(ctx, tournament.FormatID)
		if formatErr != nil {
			return nil, handleRepositoryError(formatErr, ErrFormatNotFound, "failed to load format %d for tournament %d", tournament.FormatID, tournamentID)
		}
		tournament.Format = format
	}
	if tournament.Format == nil {
		return nil, fmt.Errorf("tournament format information is missing for tournament ID %d", tournamentID)
	}

	// Проверяем, что это действительно team-турнир
	if tournament.Format.ParticipantType != models.FormatParticipantTeam {
		return nil, fmt.Errorf("%w: expected team tournament, got %s", ErrTournamentIncorrectFormatType, tournament.Format.ParticipantType)
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
		fmt.Printf("Warning: failed to get participant details after creation for participant ID %d: %v\n", participant.ID, err)
		s.populateParticipantDetails(participant)
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
	_, err := s.tournamentRepo.GetByID(ctx, tournamentID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament %d for listing applications", tournamentID)
	}

	participants, err := s.participantRepo.ListByTournament(ctx, tournamentID, statusFilter, true)
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

	if tournament.Status == models.StatusActive || tournament.Status == models.StatusCompleted || tournament.Status == models.StatusCanceled {
		return nil, fmt.Errorf("%w: current tournament status is '%s'", ErrApplicationUpdateNotAllowed, tournament.Status)
	}

	if participant.Status != models.StatusApplicationSubmitted {
		return nil, fmt.Errorf("%w: can only update applications with status '%s', current is '%s'",
			ErrApplicationUpdateNotAllowed, models.StatusApplicationSubmitted, participant.Status)
	}

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

	updatedParticipant, err := s.participantRepo.GetWithDetails(ctx, participantID)
	if err != nil {
		fmt.Printf("Warning: failed to get participant details after status update for participant ID %d: %v\n", participantID, err)
		participant.Status = newStatus
		s.populateParticipantDetails(participant)
		return participant, nil
	}
	s.populateParticipantDetails(updatedParticipant)
	return updatedParticipant, nil
}

func (s *participantService) countApprovedParticipants(ctx context.Context, tournamentID int) (int, error) {
	statusParticipant := models.StatusParticipant
	approvedParticipants, err := s.participantRepo.ListByTournament(ctx, tournamentID, &statusParticipant, false)
	if err != nil {
		return 0, err
	}
	return len(approvedParticipants), nil
}

func handleRepositoryError(err error, serviceSpecificNotFoundError error, format string, args ...interface{}) error {
	knownNotFoundErrors := []error{
		repositories.ErrUserNotFound,
		repositories.ErrTeamNotFound,
		repositories.ErrSportNotFound,
		repositories.ErrFormatNotFound,
		repositories.ErrTournamentNotFound,
		repositories.ErrParticipantNotFound,
		repositories.ErrInviteNotFound,
		repositories.ErrSoloMatchNotFound,
		repositories.ErrTeamMatchNotFound,
	}

	for _, knownErr := range knownNotFoundErrors {
		if errors.Is(err, knownErr) {
			if serviceSpecificNotFoundError != nil {
				if errors.Is(serviceSpecificNotFoundError, knownErr) {
					return serviceSpecificNotFoundError
				}
				return serviceSpecificNotFoundError
			}
			return err
		}
	}

	allArgs := append(make([]interface{}, 0, len(args)+1), args...)
	allArgs = append(allArgs, err)
	return fmt.Errorf(format+": %w", allArgs...)
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
