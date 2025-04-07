package repositories

import (
	"github.com/Dosada05/tournament-system/config"
	"github.com/Dosada05/tournament-system/models"
)

func CreateTeam(team *models.Team) error {
	query := `INSERT INTO teams (name) VALUES ($1)`
	_, err := config.DB.Exec(query, team.Name)
	return err
}

func GetTeamByID(id int) (*models.Team, error) {
	query := `SELECT id, name FROM teams WHERE id = $1`
	row := config.DB.QueryRow(query, id)

	var team models.Team
	if err := row.Scan(&team.ID, &team.Name); err != nil {
		return nil, err
	}

	return &team, nil
}

func UpdateTeam(id int, team *models.Team) error {
	query := `UPDATE teams SET name = $1 WHERE id = $2`
	_, err := config.DB.Exec(query, team.Name, id)
	return err
}

func DeleteTeam(id int) error {
	query := `DELETE FROM teams WHERE id = $1`
	_, err := config.DB.Exec(query, id)
	return err
}
