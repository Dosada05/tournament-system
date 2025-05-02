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
	ErrTeamCaptainInvalid = errors.New("team captain conflict or invalid") // Если FK на captain_id нарушен
	ErrTeamSportInvalid   = errors.New("team sport conflict or invalid")   // Если FK на sport_id нарушен
)

type TeamRepository interface {
	Create(ctx context.Context, team *models.Team) error
	GetByID(ctx context.Context, id int) (*models.Team, error)
	GetAll(ctx context.Context) ([]models.Team, error)
	Update(ctx context.Context, team *models.Team) error
	Delete(ctx context.Context, id int) error
	ExistsByName(ctx context.Context, name string) (bool, error)
	// Дополнительные методы, если нужны:
	// GetByCaptainID(ctx context.Context, captainID int) ([]models.Team, error)
	// GetBySportID(ctx context.Context, sportID int) ([]models.Team, error)
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
			case "23505": // unique_violation
				if pqErr.Constraint == "teams_name_key" { // Укажите реальное имя constraint
					return ErrTeamNameConflict
				}
			case "23503": // foreign_key_violation
				if pqErr.Constraint == "fk_teams_captain" || pqErr.Constraint == "teams_captain_id_fkey" { // Укажите реальное имя constraint
					return ErrTeamCaptainInvalid
				}
				if pqErr.Constraint == "fk_teams_sport" { // Укажите реальное имя constraint
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
		SELECT id, name, sport_id, captain_id, created_at
		FROM teams
		WHERE id = $1`

	var team models.Team
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&team.ID,
		&team.Name,
		&team.SportID,
		&team.CaptainID,
		&team.CreatedAt,
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
		SELECT id, name, sport_id, captain_id, created_at
		FROM teams
		ORDER BY name ASC` // Или ORDER BY created_at DESC, или по другому полю

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	teams := make([]models.Team, 0) // Инициализация пустого слайса
	for rows.Next() {
		var team models.Team
		if scanErr := rows.Scan(
			&team.ID,
			&team.Name,
			&team.SportID,
			&team.CaptainID,
			&team.CreatedAt,
		); scanErr != nil {
			// Если ошибка при сканировании одной строки, лучше вернуть её сразу
			return nil, scanErr
		}
		teams = append(teams, team)
	}

	// Важно проверить ошибку после завершения цикла rows.Next()
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
			case "23505": // unique_violation
				if pqErr.Constraint == "teams_name_key" {
					return ErrTeamNameConflict
				}
			case "23503": // foreign_key_violation
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
		// Теоретически, здесь может быть ошибка FK, если на команду кто-то ссылается
		// и нет ON DELETE CASCADE (например, users.team_id без ON DELETE SET NULL)
		// Можно добавить проверку на pqErr.Code "23503" (foreign_key_violation), если нужно
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
	// Более эффективный способ проверить существование
	query := `SELECT EXISTS (SELECT 1 FROM teams WHERE name = $1)`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, name).Scan(&exists)
	if err != nil {
		// Ошибки sql.ErrNoRows здесь быть не должно, т.к. EXISTS всегда возвращает строку
		return false, err
	}
	return exists, nil
}
