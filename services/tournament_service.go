package services

import (
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

func CreateTournament(tournament *models.Tournament) error {
	return repositories.CreateTournament(tournament)
}

func GetTournamentByID(id int) (*models.Tournament, error) {
	return repositories.GetTournamentByID(id)
}

func UpdateTournament(id int, tournament *models.Tournament) error {
	return repositories.UpdateTournament(id, tournament)
}

func DeleteTournament(id int) error {
	return repositories.DeleteTournament(id)
}

func GetAllTournaments(limit, offset int) ([]models.Tournament, error) {
	return repositories.GetAllTournaments(limit, offset)
}
