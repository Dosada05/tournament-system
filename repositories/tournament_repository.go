package repositories

import (
	"github.com/Dosada05/tournament-system/config"
	"github.com/Dosada05/tournament-system/models"
)

func CreateTournament(tournament *models.Tournament) error {
	query := `INSERT INTO tournaments (name, description, sport_id, format_id, organizer_id, reg_date, start_date, end_date, location, status, max_participants)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	_, err := config.DB.Exec(query, tournament.Name, tournament.Description, tournament.SportID, tournament.FormatID, tournament.OrganizerID, tournament.RegDate, tournament.StartDate, tournament.EndDate, tournament.Location, tournament.Status, tournament.MaxParticipants)
	return err
}

func GetTournamentByID(id int) (*models.Tournament, error) {
	query := `SELECT id, name, description, sport_id, format_id, organizer_id, reg_date, start_date, end_date, location, status, max_participants
              FROM tournaments WHERE id = $1`
	row := config.DB.QueryRow(query, id)

	var tournament models.Tournament
	if err := row.Scan(&tournament.ID, &tournament.Name, &tournament.Description, &tournament.SportID, &tournament.FormatID, &tournament.OrganizerID, &tournament.RegDate, &tournament.StartDate, &tournament.EndDate, &tournament.Location, &tournament.Status, &tournament.MaxParticipants); err != nil {
		return nil, err
	}

	return &tournament, nil
}

func UpdateTournament(id int, tournament *models.Tournament) error {
	currentTournament, err := GetTournamentByID(id)
	if err != nil {
		return err
	}
	// Обновляем только непустые поля, сохраняя текущие значения для остальных
	if tournament.Name != "" {
		currentTournament.Name = tournament.Name
	}
	if tournament.Description != nil && *tournament.Description != "" {
		currentTournament.Description = tournament.Description
	}
	if tournament.SportID != 0 {
		currentTournament.SportID = tournament.SportID
	}
	if tournament.FormatID != 0 {
		currentTournament.FormatID = tournament.FormatID
	}
	if !tournament.RegDate.IsZero() {
		currentTournament.RegDate = tournament.RegDate
	}
	if !tournament.StartDate.IsZero() {
		currentTournament.StartDate = tournament.StartDate
	}
	if !tournament.EndDate.IsZero() {
		currentTournament.EndDate = tournament.EndDate
	}
	if tournament.Location != nil && *tournament.Location != "" {
		currentTournament.Location = tournament.Location
	}
	if tournament.Status != "" {
		currentTournament.Status = tournament.Status
	}
	if tournament.MaxParticipants != 0 || currentTournament.MaxParticipants != 0 {
		currentTournament.MaxParticipants = tournament.MaxParticipants
	}

	// Обновление с учетом всех полей, включая organizer_id
	query := `UPDATE tournaments SET 
              name = $1, description = $2, sport_id = $3, format_id = $4,
              organizer_id = $5, reg_date = $6, start_date = $7, end_date = $8,
              location = $9, status = $10, max_participants = $11
              WHERE id = $12`

	_, err = config.DB.Exec(
		query,
		currentTournament.Name,
		currentTournament.Description,
		currentTournament.SportID,
		currentTournament.FormatID,
		currentTournament.OrganizerID,
		currentTournament.RegDate,
		currentTournament.StartDate,
		currentTournament.EndDate,
		currentTournament.Location,
		currentTournament.Status,
		currentTournament.MaxParticipants,
		id,
	)

	return err
}

func DeleteTournament(id int) error {
	query := `DELETE FROM tournaments WHERE id = $1`
	_, err := config.DB.Exec(query, id)
	return err
}

func GetAllTournaments(limit, offset int) ([]models.Tournament, error) {
	query := `SELECT id, name, description, sport_id, format_id, organizer_id, reg_date, start_date, end_date, location, status, max_participants
              FROM tournaments ORDER BY id LIMIT $1 OFFSET $2`

	rows, err := config.DB.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tournaments := make([]models.Tournament, 0, limit)
	for rows.Next() {
		var tournament models.Tournament
		if err := rows.Scan(&tournament.ID, &tournament.Name, &tournament.Description, &tournament.SportID, &tournament.FormatID, &tournament.OrganizerID, &tournament.RegDate, &tournament.StartDate, &tournament.EndDate, &tournament.Location, &tournament.Status, &tournament.MaxParticipants); err != nil {
			return nil, err
		}
		tournaments = append(tournaments, tournament)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return tournaments, nil
}
