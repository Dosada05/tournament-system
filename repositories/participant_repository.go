package repositories

import (
	"github.com/Dosada05/tournament-system/config"
	"github.com/Dosada05/tournament-system/models"
)

func RegisterParticipant(participant *models.Participant) error {
	query := `INSERT INTO participants (user_id, team_id, tournament_id, status)
              VALUES ($1, $2, $3, $4, $5)`
	_, err := config.DB.Exec(query, participant.UserID, participant.TeamID, participant.TournamentID, participant.Status)
	return err
}

func GetParticipantsByTournamentID(tournamentID int) ([]*models.Participant, error) {
	query := `SELECT id, user_id, team_id, tournament_id, status, created_at
              FROM participants WHERE tournament_id = $1`
	rows, err := config.DB.Query(query, tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participants []*models.Participant
	for rows.Next() {
		var participant models.Participant
		if err := rows.Scan(&participant.ID, &participant.UserID, &participant.TeamID, &participant.TournamentID, &participant.Status, &participant.CreatedAt); err != nil {
			return nil, err
		}
		participants = append(participants, &participant)
	}

	return participants, nil
}
