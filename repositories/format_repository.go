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
	Update(ctx context.Context, format *models.Format) error
	Delete(ctx context.Context, id int) error
}

type postgresFormatRepository struct {
	db *sql.DB
}

func NewPostgresFormatRepository(db *sql.DB) FormatRepository {
	return &postgresFormatRepository{db: db}
}

func (r *postgresFormatRepository) Create(ctx context.Context, format *models.Format) error {
	query := `INSERT INTO formats (name) VALUES ($1) RETURNING id`
	err := r.db.QueryRowContext(ctx, query, format.Name).Scan(&format.ID)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == "23505" && pqErr.Constraint == "formats_name_key" {
				return ErrFormatNameConflict
			}
		}
		return err
	}
	return nil
}

func (r *postgresFormatRepository) GetByID(ctx context.Context, id int) (*models.Format, error) {
	query := `SELECT id, name FROM formats WHERE id = $1`
	format := &models.Format{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(&format.ID, &format.Name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFormatNotFound
		}
		return nil, err
	}
	return format, nil
}

func (r *postgresFormatRepository) GetAll(ctx context.Context) ([]models.Format, error) {
	query := `SELECT id, name FROM formats ORDER BY name ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	formats := make([]models.Format, 0)
	for rows.Next() {
		var format models.Format
		if scanErr := rows.Scan(&format.ID, &format.Name); scanErr != nil {
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
	query := `UPDATE formats SET name = $1 WHERE id = $2`
	result, err := r.db.ExecContext(ctx, query, format.Name, format.ID)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == "23505" && pqErr.Constraint == "formats_name_key" {
				return ErrFormatNameConflict
			}
		}
		return err
	}

	rowsAffected, checkErr := checkRowsAffected(result)
	if checkErr != nil {
		return checkErr
	}
	if rowsAffected == 0 {
		return ErrFormatNotFound
	}

	return nil
}

func (r *postgresFormatRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM formats WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == "23503" {
				// Предполагаем, что FK на formats есть только у tournaments
				// Имя constraint может быть другим!
				if pqErr.Constraint == "tournaments_format_id_fkey" {
					return ErrFormatInUse
				}
			}
		}
		return err
	}

	rowsAffected, checkErr := checkRowsAffected(result)
	if checkErr != nil {
		return checkErr
	}
	if rowsAffected == 0 {
		return ErrFormatNotFound
	}

	return nil
}
