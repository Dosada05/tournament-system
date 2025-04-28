package repositories

import (
	"database/sql"

	"github.com/Dosada05/tournament-system/models"
)

type SportRepository interface {
	Create(sport *models.Sport) error
	GetByID(id int) (*models.Sport, error)
	GetAll() ([]models.Sport, error)
	Update(id int, sport *models.Sport) error
	Delete(id int) error
	ExistsByName(name string) (bool, error)
}

type sportRepository struct {
	db *sql.DB
}

func NewSportRepository(db *sql.DB) SportRepository {
	return &sportRepository{db: db}
}

func (r *sportRepository) Create(sport *models.Sport) error {
	query := `INSERT INTO sports (name) VALUES ($1) RETURNING id`
	return r.db.QueryRow(query, sport.Name).Scan(&sport.ID)
}

func (r *sportRepository) GetByID(id int) (*models.Sport, error) {
	query := `SELECT id, name FROM sports WHERE id = $1`
	row := r.db.QueryRow(query, id)

	var sport models.Sport
	if err := row.Scan(&sport.ID, &sport.Name); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &sport, nil
}

func (r *sportRepository) GetAll() ([]models.Sport, error) {
	query := `SELECT id, name FROM sports ORDER BY id`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sports []models.Sport
	for rows.Next() {
		var sport models.Sport
		if err := rows.Scan(&sport.ID, &sport.Name); err != nil {
			return nil, err
		}
		sports = append(sports, sport)
	}
	return sports, rows.Err()
}

func (r *sportRepository) Update(id int, sport *models.Sport) error {
	query := `UPDATE sports SET name = $1 WHERE id = $2`
	_, err := r.db.Exec(query, sport.Name, id)
	return err
}

func (r *sportRepository) Delete(id int) error {
	query := `DELETE FROM sports WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *sportRepository) ExistsByName(name string) (bool, error) {
	query := `SELECT id FROM sports WHERE name = $1`
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
