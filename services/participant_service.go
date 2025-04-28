package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

// ParticipantService инкапсулирует бизнес-логику для участников турниров.
type ParticipantService struct {
	repo           repositories.ParticipantRepository
	userRepo       repositories.UserRepository
	teamRepo       repositories.TeamRepository
	tournamentRepo repositories.TournamentRepository
}

// NewParticipantService создаёт ParticipantService с внедрением зависимостей.
func NewParticipantService(
	repo repositories.ParticipantRepository,
	userRepo repositories.UserRepository,
	teamRepo repositories.TeamRepository,
	tournamentRepo repositories.TournamentRepository,
) *ParticipantService {
	return &ParticipantService{
		repo:           repo,
		userRepo:       userRepo,
		teamRepo:       teamRepo,
		tournamentRepo: tournamentRepo,
	}
}

// RegisterUserAsParticipant добавляет пользователя в турнир.
func (s *ParticipantService) RegisterUserAsParticipant(ctx context.Context, userID, tournamentID int) error {
	// Проверка существования пользователя
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("ошибка при проверке пользователя: %w", err)
	}
	if user == nil {
		return errors.New("пользователь не найден")
	}

	// Проверка существования турнира
	tournament, err := s.tournamentRepo.GetByID(tournamentID)
	if err != nil {
		return fmt.Errorf("ошибка при проверке турнира: %w", err)
	}
	if tournament == nil {
		return errors.New("турнир не найден")
	}

	// Проверка на повторную регистрацию
	existing, err := s.repo.FindByUserAndTournament(ctx, userID, tournamentID)
	if err != nil {
		return fmt.Errorf("ошибка при проверке участия: %w", err)
	}
	if existing != nil {
		return errors.New("пользователь уже зарегистрирован в этом турнире")
	}

	participant := &models.Participant{
		UserID:       &userID,
		TournamentID: tournamentID,
		Status:       models.StatusApplicationSubmitted,
	}

	return s.repo.Create(ctx, participant)
}

// RegisterTeamAsParticipant добавляет команду в турнир.
func (s *ParticipantService) RegisterTeamAsParticipant(ctx context.Context, teamID, tournamentID int) error {
	// Проверка существования команды
	team, err := s.teamRepo.GetByID(teamID)
	if err != nil {
		return fmt.Errorf("ошибка при проверке команды: %w", err)
	}
	if team == nil {
		return errors.New("команда не найдена")
	}

	// Проверка существования турнира
	tournament, err := s.tournamentRepo.GetByID(tournamentID)
	if err != nil {
		return fmt.Errorf("ошибка при проверке турнира: %w", err)
	}
	if tournament == nil {
		return errors.New("турнир не найден")
	}

	// Проверка на повторную регистрацию
	existing, err := s.repo.FindByTeamAndTournament(ctx, teamID, tournamentID)
	if err != nil {
		return fmt.Errorf("ошибка при проверке участия: %w", err)
	}
	if existing != nil {
		return errors.New("команда уже зарегистрирована в этом турнире")
	}

	participant := &models.Participant{
		TeamID:       &teamID,
		TournamentID: tournamentID,
		Status:       models.StatusApplicationSubmitted,
	}

	return s.repo.Create(ctx, participant)
}

// ChangeParticipantStatus меняет статус заявки участника.
func (s *ParticipantService) ChangeParticipantStatus(ctx context.Context, participantID int, status models.ParticipantStatus) error {
	// Проверка существования участника
	participant, err := s.repo.FindByID(ctx, participantID)
	if err != nil {
		return fmt.Errorf("ошибка при поиске участника: %w", err)
	}
	if participant == nil {
		return errors.New("участник не найден")
	}
	return s.repo.UpdateStatus(ctx, participantID, status)
}

// GetParticipantByID возвращает участника по ID.
func (s *ParticipantService) GetParticipantByID(ctx context.Context, id int) (*models.Participant, error) {
	return s.repo.FindByID(ctx, id)
}

// ListParticipantsByTournament возвращает всех участников турнира.
func (s *ParticipantService) ListParticipantsByTournament(ctx context.Context, tournamentID int) ([]*models.Participant, error) {
	return s.repo.ListByTournament(ctx, tournamentID)
}

// DeleteParticipant удаляет участника.
func (s *ParticipantService) DeleteParticipant(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}
