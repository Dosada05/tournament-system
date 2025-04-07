package services

import (
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
)

func RegisterParticipant(participant *models.Participant) error {
	return repositories.RegisterParticipant(participant)
}

func GetParticipantsByTournamentID(tournamentID int) ([]*models.Participant, error) {
	return repositories.GetParticipantsByTournamentID(tournamentID)
}
