package repositories

import (
	"github.com/Dosada05/tournament-system/config"
	"github.com/Dosada05/tournament-system/models"
)

func CreateTournament(tournament *models.Tournament) error {
	query := `INSERT INTO tournaments (name, description, sport_type, format, organizer_id, reg_date, start_date, end_date, location, status, max_participants)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	_, err := config.DB.Exec(query, tournament.Name, tournament.Description, tournament.SportType, tournament.Format, tournament.OrganizerID, tournament.RegistrationDate, tournament.StartDate, tournament.EndDate, tournament.Location, tournament.Status, tournament.MaxParticipants)
	return err
}

func GetTournamentByID(id int) (*models.Tournament, error) {
	query := `SELECT id, name, description, sport_type, format, organizer_id, reg_date, start_date, end_date, location, status, max_participants
              FROM tournaments WHERE id = $1`
	row := config.DB.QueryRow(query, id)

	var tournament models.Tournament
	if err := row.Scan(&tournament.ID, &tournament.Name, &tournament.Description, &tournament.SportType, &tournament.Format, &tournament.OrganizerID, &tournament.RegistrationDate, &tournament.StartDate, &tournament.EndDate, &tournament.Location, &tournament.Status, &tournament.MaxParticipants); err != nil {
		return nil, err
	}

	return &tournament, nil
}

func UpdateTournament(id int, tournament *models.Tournament) error {
	query := `UPDATE tournaments SET name = $1, description = $2, sport_type = $3, format = $4, organizer_id = $5, reg_date = $6, start_date = $7, end_date = $8, location = $9, status = $10, max_participants = $11 WHERE id = $12`
	_, err := config.DB.Exec(query, tournament.Name, tournament.Description, tournament.SportType, tournament.Format, tournament.OrganizerID, tournament.RegistrationDate, tournament.StartDate, tournament.EndDate, tournament.Location, tournament.Status, tournament.MaxParticipants, id)
	return err
}

func DeleteTournament(id int) error {
	query := `DELETE FROM tournaments WHERE id = $1`
	_, err := config.DB.Exec(query, id)
	return err
}
