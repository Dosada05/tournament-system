package repositories

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Dosada05/tournament-system/models"
	"github.com/lib/pq"
)

var (
	ErrTeamNotFound       = errors.New("team not found")
	ErrTeamNameConflict   = errors.New("team name conflict")
	ErrTeamCaptainInvalid = errors.New("team captain conflict or invalid")
	ErrTeamSportInvalid   = errors.New("team sport conflict or invalid")
)

type TeamRepository interface {
	Create(ctx context.Context, team *models.Team) error
	GetByID(ctx context.Context, id int) (*models.Team, error)
	GetAll(ctx context.Context) ([]models.Team, error)
	Update(ctx context.Context, team *models.Team) error
	Delete(ctx context.Context, id int) error
	ExistsByName(ctx context.Context, name string) (bool, error)
	UpdateLogoKey(ctx context.Context, teamID int, logoKey *string) error
}

type postgresTeamRepository struct {
	db *sql.DB
}

func NewPostgresTeamRepository(db *sql.DB) TeamRepository {
	return &postgresTeamRepository{db: db}
}

func (r *postgresTeamRepository) Create(ctx context.Context, team *models.Team) error {
	query := `
		INSERT INTO teams (name, sport_id, captain_id)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, query,
		team.Name,
		team.SportID,
		team.CaptainID,
	).Scan(&team.ID, &team.CreatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			switch pqErr.Code {
			case "23505":
				if pqErr.Constraint == "teams_name_key" {
					return ErrTeamNameConflict
				}
			case "23503":
				if pqErr.Constraint == "fk_teams_captain" || pqErr.Constraint == "teams_captain_id_fkey" {
					return ErrTeamCaptainInvalid
				}
				if pqErr.Constraint == "fk_teams_sport" {
					return ErrTeamSportInvalid
				}
			}
		}
		return err
	}

	return nil
}

func (r *postgresTeamRepository) GetByID(ctx context.Context, id int) (*models.Team, error) {
	query := `
		SELECT id, name, sport_id, captain_id, created_at, logo_key
		FROM teams
		WHERE id = $1`

	var team models.Team
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&team.ID,
		&team.Name,
		&team.SportID,
		&team.CaptainID,
		&team.CreatedAt,
		&team.LogoKey,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTeamNotFound
		}
		return nil, err
	}

	return &team, nil
}

func (r *postgresTeamRepository) GetAll(ctx context.Context) ([]models.Team, error) {
	query := `
		SELECT id, name, sport_id, captain_id, created_at, logo_key
		FROM teams
		ORDER BY name ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	teams := make([]models.Team, 0)
	for rows.Next() {
		var team models.Team
		if scanErr := rows.Scan(
			&team.ID,
			&team.Name,
			&team.SportID,
			&team.CaptainID,
			&team.CreatedAt,
			&team.LogoKey,
		); scanErr != nil {
			return nil, scanErr
		}
		teams = append(teams, team)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return teams, nil
}

func (r *postgresTeamRepository) Update(ctx context.Context, team *models.Team) error {
	query := `
		UPDATE teams
		SET name = $1, sport_id = $2, captain_id = $3
		WHERE id = $4`

	result, err := r.db.ExecContext(ctx, query,
		team.Name,
		team.SportID,
		team.CaptainID,
		team.ID,
	)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			switch pqErr.Code {
			case "23505":
				if pqErr.Constraint == "teams_name_key" {
					return ErrTeamNameConflict
				}
			case "23503":
				if pqErr.Constraint == "fk_teams_captain" || pqErr.Constraint == "teams_captain_id_fkey" {
					return ErrTeamCaptainInvalid
				}
				if pqErr.Constraint == "fk_teams_sport" {
					return ErrTeamSportInvalid
				}
			}
		}
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrTeamNotFound
	}

	return nil
}

func (r *postgresTeamRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM teams WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrTeamNotFound
	}

	return nil
}

func (r *postgresTeamRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	query := `SELECT EXISTS (SELECT 1 FROM teams WHERE name = $1)`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, name).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *postgresTeamRepository) UpdateLogoKey(ctx context.Context, teamID int, logoKey *string) error {
	query := `
		UPDATE teams
		SET logo_key = $1
		WHERE id = $2`

	result, err := r.db.ExecContext(ctx, query, logoKey, teamID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrTeamNotFound
	}
	return nil
}
