package repositories

import (
	"database/sql"
	"errors"

	"github.com/Dosada05/tournament-system/models"
)

type TournamentRepository interface {
	Create(t *models.Tournament) error
	GetByID(id int) (*models.Tournament, error)
	Update(id int, t *models.Tournament) error
	Delete(id int) error
	GetAll(limit, offset int) ([]models.Tournament, error)
}

type tournamentRepository struct {
	db *sql.DB
}

func NewTournamentRepository(db *sql.DB) TournamentRepository {
	return &tournamentRepository{db: db}
}

func (r *tournamentRepository) Create(t *models.Tournament) error {
	query := `INSERT INTO tournaments
		(name, description, sport_id, format_id, organizer_id, reg_date, start_date, end_date, location, status, max_participants)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	_, err := r.db.Exec(query,
		t.Name,
		t.Description,
		t.SportID,
		t.FormatID,
		t.OrganizerID,
		t.RegDate,
		t.StartDate,
		t.EndDate,
		t.Location,
		t.Status,
		t.MaxParticipants,
	)
	return err
}

func (r *tournamentRepository) GetByID(id int) (*models.Tournament, error) {
	query := `SELECT id, name, description, sport_id, format_id, organizer_id, reg_date, start_date, end_date, location, status, max_participants
			  FROM tournaments WHERE id = $1`
	row := r.db.QueryRow(query, id)

	var t models.Tournament
	if err := row.Scan(
		&t.ID,
		&t.Name,
		&t.Description,
		&t.SportID,
		&t.FormatID,
		&t.OrganizerID,
		&t.RegDate,
		&t.StartDate,
		&t.EndDate,
		&t.Location,
		&t.Status,
		&t.MaxParticipants,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (r *tournamentRepository) Update(id int, update *models.Tournament) error {
	current, err := r.GetByID(id)
	if err != nil {
		return err
	}
	if current == nil {
		return sql.ErrNoRows
	}
	if update.Name != "" {
		current.Name = update.Name
	}
	if update.Description != nil && *update.Description != "" {
		current.Description = update.Description
	}
	if update.SportID != 0 {
		current.SportID = update.SportID
	}
	if update.FormatID != 0 {
		current.FormatID = update.FormatID
	}
	if update.OrganizerID != 0 {
		current.OrganizerID = update.OrganizerID
	}
	if !update.RegDate.IsZero() {
		current.RegDate = update.RegDate
	}
	if !update.StartDate.IsZero() {
		current.StartDate = update.StartDate
	}
	if !update.EndDate.IsZero() {
		current.EndDate = update.EndDate
	}
	if update.Location != nil && *update.Location != "" {
		current.Location = update.Location
	}
	if update.Status != "" {
		current.Status = update.Status
	}
	if update.MaxParticipants != 0 {
		current.MaxParticipants = update.MaxParticipants
	}

	query := `UPDATE tournaments SET 
              name = $1, description = $2, sport_id = $3, format_id = $4,
              organizer_id = $5, reg_date = $6, start_date = $7, end_date = $8,
              location = $9, status = $10, max_participants = $11
              WHERE id = $12`
	_, err = r.db.Exec(
		query,
		current.Name,
		current.Description,
		current.SportID,
		current.FormatID,
		current.OrganizerID,
		current.RegDate,
		current.StartDate,
		current.EndDate,
		current.Location,
		current.Status,
		current.MaxParticipants,
		id,
	)
	return err
}

func (r *tournamentRepository) Delete(id int) error {
	query := `DELETE FROM tournaments WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *tournamentRepository) GetAll(limit, offset int) ([]models.Tournament, error) {
	query := `SELECT id, name, description, sport_id, format_id, organizer_id, reg_date, start_date, end_date, location, status, max_participants
			  FROM tournaments ORDER BY id LIMIT $1 OFFSET $2`
	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tournaments := make([]models.Tournament, 0, limit)
	for rows.Next() {
		var t models.Tournament
		if err := rows.Scan(
			&t.ID,
			&t.Name,
			&t.Description,
			&t.SportID,
			&t.FormatID,
			&t.OrganizerID,
			&t.RegDate,
			&t.StartDate,
			&t.EndDate,
			&t.Location,
			&t.Status,
			&t.MaxParticipants,
		); err != nil {
			return nil, err
		}
		tournaments = append(tournaments, t)
	}
	return tournaments, rows.Err()
}
