package repositories

import (
	"github.com/Dosada05/tournament-system/config"
	"github.com/Dosada05/tournament-system/models"
)

func CreateMatch(match *models.Match) error {
	query := `INSERT INTO matches (tournament_id, p1_id, p2_id, score, date, winner_id)
              VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := config.DB.Exec(query, match.TournamentID, match.Player1ID, match.Player2ID, match.Score, match.Date, match.WinnerID)
	return err
}

func GetMatchByID(id int) (*models.Match, error) {
	query := `SELECT id, tournament_id, p1_id, p2_id, score, date, winner_id
              FROM matches WHERE id = $1`
	row := config.DB.QueryRow(query, id)

	var match models.Match
	if err := row.Scan(&match.ID, &match.TournamentID, &match.Player1ID, &match.Player2ID, &match.Score, &match.Date, &match.WinnerID); err != nil {
		return nil, err
	}

	return &match, nil
}

func UpdateMatch(id int, match *models.Match) error {
	query := `UPDATE matches SET tournament_id = $1, p1_id = $2, p2_id = $3, score = $4, date = $5, winner_id = $6 WHERE id = $7`
	_, err := config.DB.Exec(query, match.TournamentID, match.Player1ID, match.Player2ID, match.Score, match.Date, match.WinnerID, id)
	return err
}

func DeleteMatch(id int) error {
	query := `DELETE FROM matches WHERE id = $1`
	_, err := config.DB.Exec(query, id)
	return err
}
