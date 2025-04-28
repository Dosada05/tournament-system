package services

import (
	"errors"
	"fmt"

	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

// SportService инкапсулирует бизнес-логику для видов спорта.
type SportService struct {
	repo repositories.SportRepository
}

// NewSportService создает новый сервис видов спорта с внедренным репозиторием.
func NewSportService(repo repositories.SportRepository) *SportService {
	return &SportService{repo: repo}
}

// CreateSport добавляет новый вид спорта с валидацией.
func (s *SportService) CreateSport(sport *models.Sport) error {
	if len(sport.Name) < 2 {
		return errors.New("название вида спорта должно содержать не менее 2 символов")
	}
	exists, err := s.repo.ExistsByName(sport.Name)
	if err != nil {
		return fmt.Errorf("ошибка при проверке существования вида спорта: %w", err)
	}
	if exists {
		return errors.New("вид спорта с таким названием уже существует")
	}
	return s.repo.Create(sport)
}

// GetAllSports возвращает все виды спорта.
func (s *SportService) GetAllSports() ([]models.Sport, error) {
	return s.repo.GetAll()
}

// GetSportByID возвращает вид спорта по ID.
func (s *SportService) GetSportByID(id int) (*models.Sport, error) {
	return s.repo.GetByID(id)
}

// UpdateSport обновляет название вида спорта.
func (s *SportService) UpdateSport(id int, sport *models.Sport) error {
	return s.repo.Update(id, sport)
}

// DeleteSport удаляет вид спорта.
func (s *SportService) DeleteSport(id int) error {
	return s.repo.Delete(id)
}
