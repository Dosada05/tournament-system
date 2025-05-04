package repositories

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Dosada05/tournament-system/models"
	"github.com/lib/pq"
)

var (
	ErrSportNotFound     = errors.New("sport not found")
	ErrSportNameConflict = errors.New("sport name conflict")
	ErrSportInUse        = errors.New("sport cannot be deleted as it is in use") // Для ошибки FK при удалении
)

type SportRepository interface {
	Create(ctx context.Context, sport *models.Sport) error
	GetByID(ctx context.Context, id int) (*models.Sport, error)
	GetAll(ctx context.Context) ([]models.Sport, error)
	Update(ctx context.Context, sport *models.Sport) error
	Delete(ctx context.Context, id int) error
	ExistsByName(ctx context.Context, name string) (bool, error)
}

type postgresSportRepository struct {
	db *sql.DB
}

func NewPostgresSportRepository(db *sql.DB) SportRepository {
	return &postgresSportRepository{db: db}
}

func (r *postgresSportRepository) Create(ctx context.Context, sport *models.Sport) error {
	query := `INSERT INTO sports (name) VALUES ($1) RETURNING id`

	err := r.db.QueryRowContext(ctx, query, sport.Name).Scan(&sport.ID)
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
	query := `SELECT id, name FROM sports WHERE id = $1`

	var sport models.Sport
	err := r.db.QueryRowContext(ctx, query, id).Scan(&sport.ID, &sport.Name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSportNotFound
		}
		return nil, err
	}
	return &sport, nil
}

func (r *postgresSportRepository) GetAll(ctx context.Context) ([]models.Sport, error) {
	query := `SELECT id, name FROM sports ORDER BY name ASC` // Сортировка по имени

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Оптимальное создание слайса
	sports := make([]models.Sport, 0)
	estimatedCapacity := 10 // Можно задать ожидаемую емкость, если известно
	if rows.Next() {        // Проверяем, есть ли хоть одна строка, перед созданием с capacity
		sports = make([]models.Sport, 0, estimatedCapacity)
		var sport models.Sport
		if scanErr := rows.Scan(&sport.ID, &sport.Name); scanErr != nil {
			return nil, scanErr
		}
		sports = append(sports, sport)
	} else {
		if err = rows.Err(); err != nil { // Проверка ошибки, даже если строк не было
			return nil, err
		}
		return sports, nil // Возвращаем пустой слайс, если строк нет
	}

	// Продолжаем цикл для остальных строк
	for rows.Next() {
		var sport models.Sport
		if scanErr := rows.Scan(&sport.ID, &sport.Name); scanErr != nil {
			return nil, scanErr
		}
		sports = append(sports, sport)
	}

	// Критически важная проверка ошибки после цикла
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return sports, nil
}

func (r *postgresSportRepository) Update(ctx context.Context, sport *models.Sport) error {
	query := `UPDATE sports SET name = $1 WHERE id = $2`

	result, err := r.db.ExecContext(ctx, query, sport.Name, sport.ID)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" { // unique_violation
			if pqErr.Constraint == "sports_name_key" { // ЗАМЕНИТЕ на реальное имя constraint
				return ErrSportNameConflict
			}
		}
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrSportNotFound
	}

	return nil
}

func (r *postgresSportRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM sports WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		// Проверка на ошибку FK (т.к. у нас ON DELETE RESTRICT для teams и tournaments)
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23503" { // foreign_key_violation
			return ErrSportInUse
		}
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrSportNotFound
	}

	return nil
}

func (r *postgresSportRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	query := `SELECT EXISTS (SELECT 1 FROM sports WHERE name = $1)`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, name).Scan(&exists)
	if err != nil {
		// Ошибка sql.ErrNoRows здесь не ожидается
		return false, err
	}
	return exists, nil
}
