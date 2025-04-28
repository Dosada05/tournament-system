package services

import (
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

type TournamentService struct {
	repo repositories.TournamentRepository
}

func NewTournamentService(repo repositories.TournamentRepository) *TournamentService {
	return &TournamentService{repo: repo}
}

func (s *TournamentService) CreateTournament(tournament *models.Tournament) error {
	return s.repo.Create(tournament)
}

func (s *TournamentService) GetTournamentByID(id int) (*models.Tournament, error) {
	return s.repo.GetByID(id)
}

func (s *TournamentService) UpdateTournament(id int, tournament *models.Tournament) error {
	return s.repo.Update(id, tournament)
}

func (s *TournamentService) DeleteTournament(id int) error {
	return s.repo.Delete(id)
}

func (s *TournamentService) GetAllTournaments(limit, offset int) ([]models.Tournament, error) {
	return s.repo.GetAll(limit, offset)
}
