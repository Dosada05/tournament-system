package services

import (
	"errors"
	"fmt"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

// TeamService инкапсулирует бизнес-логику для команд.
type TeamService struct {
	teamRepo  repositories.TeamRepository
	sportRepo repositories.SportRepository
}

// NewTeamService создаёт TeamService с внедрением зависимостей.
func NewTeamService(teamRepo repositories.TeamRepository, sportRepo repositories.SportRepository) *TeamService {
	return &TeamService{
		teamRepo:  teamRepo,
		sportRepo: sportRepo,
	}
}

// CreateTeam выполняет валидацию и создаёт команду.
func (s *TeamService) CreateTeam(team *models.Team) error {
	if len(team.Name) < 3 {
		return errors.New("название команды должно содержать не менее 3 символов")
	}
	if len(team.Name) > 30 {
		return errors.New("название команды не должно превышать 30 символов")
	}

	exists, err := s.teamRepo.ExistsByName(team.Name)
	if err != nil {
		return fmt.Errorf("ошибка при проверке названия команды: %w", err)
	}
	if exists {
		return errors.New("команда с таким названием уже существует")
	}

	sport, err := s.sportRepo.GetByID(team.SportID)
	if err != nil {
		return fmt.Errorf("ошибка при проверке спорта: %w", err)
	}
	if sport == nil {
		return errors.New("указанный вид спорта не существует")
	}

	return s.teamRepo.Create(team)
}

// GetTeamByID возвращает команду по ID.
func (s *TeamService) GetTeamByID(id int) (*models.Team, error) {
	return s.teamRepo.GetByID(id)
}

// GetAllTeams возвращает все команды.
func (s *TeamService) GetAllTeams() ([]models.Team, error) {
	return s.teamRepo.GetAll()
}

// UpdateTeam выполняет валидацию и обновляет команду.
func (s *TeamService) UpdateTeam(id int, team *models.Team) error {
	if len(team.Name) < 3 {
		return errors.New("название команды должно содержать не менее 3 символов")
	}
	if len(team.Name) > 30 {
		return errors.New("название команды не должно превышать 30 символов")
	}

	exists, err := s.teamRepo.ExistsByName(team.Name)
	if err != nil {
		return fmt.Errorf("ошибка при проверке названия команды: %w", err)
	}
	if exists {
		return errors.New("команда с таким названием уже существует")
	}

	sport, err := s.sportRepo.GetByID(team.SportID)
	if err != nil {
		return fmt.Errorf("ошибка при проверке спорта: %w", err)
	}
	if sport == nil {
		return errors.New("указанный вид спорта не существует")
	}

	return s.teamRepo.Update(id, team)
}

// DeleteTeam удаляет команду.
func (s *TeamService) DeleteTeam(id int) error {
	return s.teamRepo.Delete(id)
}
