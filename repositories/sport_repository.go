package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Dosada05/tournament-system/models"
	"github.com/lib/pq"
)

var (
	ErrSportNotFound     = errors.New("sport not found")
	ErrSportNameConflict = errors.New("sport name conflict")
	ErrSportInUse        = errors.New("sport cannot be deleted as it is in use")
)

type SportRepository interface {
	Create(ctx context.Context, sport *models.Sport) error
	GetByID(ctx context.Context, id int) (*models.Sport, error)
	GetAll(ctx context.Context) ([]models.Sport, error)
	Update(ctx context.Context, sport *models.Sport) error
	Delete(ctx context.Context, id int) error
	ExistsByName(ctx context.Context, name string) (bool, error)
	UpdateLogoKey(ctx context.Context, sportID int, logoKey *string) error
}

type postgresSportRepository struct {
	db *sql.DB
}

func NewPostgresSportRepository(db *sql.DB) SportRepository {
	return &postgresSportRepository{db: db}
}

func (r *postgresSportRepository) Create(ctx context.Context, sport *models.Sport) error {
	query := `INSERT INTO sports (name, logo_key) VALUES ($1, $2) RETURNING id` // Добавили logo_key
	err := r.db.QueryRowContext(ctx, query, sport.Name, sport.LogoKey).Scan(&sport.ID)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			if pqErr.Constraint == "sports_name_key" {
				return ErrSportNameConflict
			}
		}
		return err
	}
	return nil
}

func (r *postgresSportRepository) GetByID(ctx context.Context, id int) (*models.Sport, error) {
	query := `SELECT id, name, logo_key FROM sports WHERE id = $1` // Добавили logo_key
	var sport models.Sport
	err := r.db.QueryRowContext(ctx, query, id).Scan(&sport.ID, &sport.Name, &sport.LogoKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSportNotFound
		}
		return nil, err
	}
	return &sport, nil
}

func (r *postgresSportRepository) GetAll(ctx context.Context) ([]models.Sport, error) {
	query := `SELECT id, name, logo_key FROM sports ORDER BY name ASC` // Добавили logo_key
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sports := make([]models.Sport, 0)
	for rows.Next() {
		var sport models.Sport
		if scanErr := rows.Scan(&sport.ID, &sport.Name, &sport.LogoKey); scanErr != nil {
			return nil, scanErr
		}
		sports = append(sports, sport)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return sports, nil
}

func (r *postgresSportRepository) Update(ctx context.Context, sport *models.Sport) error {
	// При обычном обновлении имени логотип не трогаем здесь, для лого будет UpdateLogoKey
	query := `UPDATE sports SET name = $1 WHERE id = $2`
	result, err := r.db.ExecContext(ctx, query, sport.Name, sport.ID)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			if pqErr.Constraint == "sports_name_key" {
				return ErrSportNameConflict
			}
		}
		return err
	}
	return r.checkAffected(result)
}

func (r *postgresSportRepository) UpdateLogoKey(ctx context.Context, sportID int, logoKey *string) error {
	query := `UPDATE sports SET logo_key = $1 WHERE id = $2`
	result, err := r.db.ExecContext(ctx, query, logoKey, sportID)
	if err != nil {
		return fmt.Errorf("failed to update sport logo key: %w", err)
	}
	return r.checkAffected(result)
}

func (r *postgresSportRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM sports WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23503" {
			return ErrSportInUse
		}
		return err
	}
	return r.checkAffected(result)
}

func (r *postgresSportRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	query := `SELECT EXISTS (SELECT 1 FROM sports WHERE name = $1)`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, name).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *postgresSportRepository) checkAffected(result sql.Result) error { // Хелпер для проверки RowsAffected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}
	if rowsAffected == 0 {
		return ErrSportNotFound // Или соответствующая ошибка NotFound для сущности
	}
	return nil
}
