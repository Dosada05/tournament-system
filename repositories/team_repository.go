package repositories

import (
	"database/sql"

	"github.com/Dosada05/tournament-system/models"
)

type TeamRepository interface {
	Create(team *models.Team) error
	GetByID(id int) (*models.Team, error)
	GetAll() ([]models.Team, error)
	Update(id int, team *models.Team) error
	Delete(id int) error
	ExistsByName(name string) (bool, error)
}

type teamRepository struct {
	db *sql.DB
}

func NewTeamRepository(db *sql.DB) TeamRepository {
	return &teamRepository{db: db}
}

func (r *teamRepository) Create(team *models.Team) error {
	query := `INSERT INTO teams (name, sport_id, captain_id) VALUES ($1, $2, $3) RETURNING id`
	return r.db.QueryRow(query, team.Name, team.SportID, team.CaptainID).Scan(&team.ID)
}

func (r *teamRepository) GetByID(id int) (*models.Team, error) {
	query := `SELECT id, name, sport_id, captain_id FROM teams WHERE id = $1`
	row := r.db.QueryRow(query, id)

	var team models.Team
	if err := row.Scan(&team.ID, &team.Name, &team.SportID, &team.CaptainID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &team, nil
}

func (r *teamRepository) GetAll() ([]models.Team, error) {
	query := `SELECT id, name, sport_id, captain_id FROM teams ORDER BY id`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []models.Team
	for rows.Next() {
		var team models.Team
		if err := rows.Scan(&team.ID, &team.Name, &team.SportID, &team.CaptainID); err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	return teams, rows.Err()
}

func (r *teamRepository) Update(id int, team *models.Team) error {
	query := `UPDATE teams SET name = $1, sport_id = $2, captain_id = $3 WHERE id = $4`
	_, err := r.db.Exec(query, team.Name, team.SportID, team.CaptainID, id)
	return err
}

func (r *teamRepository) Delete(id int) error {
	query := `DELETE FROM teams WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *teamRepository) ExistsByName(name string) (bool, error) {
	query := `SELECT id FROM teams WHERE name = $1`
	row := r.db.QueryRow(query, name)

	var id int
	err := row.Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
