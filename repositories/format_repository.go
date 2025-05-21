package repositories

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Dosada05/tournament-system/models"
	"github.com/lib/pq"
)

var (
	ErrFormatNotFound     = errors.New("format not found")
	ErrFormatNameConflict = errors.New("format name conflict")
	ErrFormatInUse        = errors.New("format is in use by a tournament")
)

type FormatRepository interface {
	Create(ctx context.Context, format *models.Format) error
	GetByID(ctx context.Context, id int) (*models.Format, error)
	GetAll(ctx context.Context) ([]models.Format, error)
	Update(ctx context.Context, format *models.Format) error // Обновление также должно учитывать все поля
	Delete(ctx context.Context, id int) error
}

type postgresFormatRepository struct {
	db *sql.DB
}

func NewPostgresFormatRepository(db *sql.DB) FormatRepository {
	return &postgresFormatRepository{db: db}
}

func (r *postgresFormatRepository) Create(ctx context.Context, format *models.Format) error {
	// При создании теперь нужно передавать все поля
	query := `
		INSERT INTO formats (name, bracket_type, participant_type, settings_json) 
		VALUES ($1, $2, $3, $4) 
		RETURNING id`
	err := r.db.QueryRowContext(ctx, query,
		format.Name,
		format.BracketType,
		format.ParticipantType,
		format.SettingsJSON,
	).Scan(&format.ID)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == "23505" && pqErr.Constraint == "formats_name_key" {
				return ErrFormatNameConflict
			}
			// Обработка нарушения CHECK constraint chk_format_participant_type
			if pqErr.Code == "23514" && pqErr.Constraint == "chk_format_participant_type" {
				return errors.New("invalid participant_type value for format")
			}
		}
		return err
	}
	return nil
}

func (r *postgresFormatRepository) GetByID(ctx context.Context, id int) (*models.Format, error) {
	// Теперь выбираем все поля
	query := `
		SELECT id, name, bracket_type, participant_type, settings_json 
		FROM formats 
		WHERE id = $1`
	format := &models.Format{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&format.ID,
		&format.Name,
		&format.BracketType,
		&format.ParticipantType,
		&format.SettingsJSON, // settings_json может быть NULL, поэтому &format.SettingsJSON
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFormatNotFound
		}
		return nil, err
	}
	return format, nil
}

func (r *postgresFormatRepository) GetAll(ctx context.Context) ([]models.Format, error) {
	// Также выбираем все поля
	query := `
		SELECT id, name, bracket_type, participant_type, settings_json 
		FROM formats 
		ORDER BY name ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	formats := make([]models.Format, 0)
	for rows.Next() {
		var format models.Format
		if scanErr := rows.Scan(
			&format.ID,
			&format.Name,
			&format.BracketType,
			&format.ParticipantType,
			&format.SettingsJSON,
		); scanErr != nil {
			return nil, scanErr
		}
		formats = append(formats, format)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return formats, nil
}

func (r *postgresFormatRepository) Update(ctx context.Context, format *models.Format) error {
	// Обновляем все поля, которые могут быть изменены
	query := `
		UPDATE formats 
		SET name = $1, bracket_type = $2, participant_type = $3, settings_json = $4
		WHERE id = $5`
	result, err := r.db.ExecContext(ctx, query,
		format.Name,
		format.BracketType,
		format.ParticipantType,
		format.SettingsJSON,
		format.ID,
	)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == "23505" && pqErr.Constraint == "formats_name_key" {
				return ErrFormatNameConflict
			}
			if pqErr.Code == "23514" && pqErr.Constraint == "chk_format_participant_type" {
				return errors.New("invalid participant_type value for format update")
			}
		}
		return err
	}

	return checkAffectedRows(result, ErrFormatNotFound)
}

func (r *postgresFormatRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM formats WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == "23503" { // foreign_key_violation
				if pqErr.Constraint == "tournaments_format_id_fkey" { // Пример имени constraint
					return ErrFormatInUse
				}
			}
		}
		return err
	}

	return checkAffectedRows(result, ErrFormatNotFound)
}
