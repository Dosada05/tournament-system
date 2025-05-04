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
	ErrUserNotFound         = errors.New("user not found")
	ErrUserEmailConflict    = errors.New("user email conflict")
	ErrUserNicknameConflict = errors.New("user nickname conflict") // <--- ДОБАВЛЕНО ЗДЕСЬ
	ErrUserTeamInvalid      = errors.New("user team conflict or invalid")
)

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id int) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id int) error
	ListByTeamID(ctx context.Context, teamID int) ([]models.User, error)
	// ListUsers (возможно, с пагинацией и фильтрами) - можно добавить позже
}

type postgresUserRepository struct {
	db *sql.DB
}

func NewPostgresUserRepository(db *sql.DB) UserRepository {
	return &postgresUserRepository{db: db}
}

func (r *postgresUserRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (first_name, last_name, nickname, email, password_hash, role, team_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, query,
		user.FirstName,
		user.LastName,
		user.Nickname,
		user.Email,
		user.PasswordHash,
		user.Role,
		user.TeamID,
	).Scan(&user.ID, &user.CreatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			switch pqErr.Code {
			case "23505": // unique_violation
				// ЗАМЕНИТЕ имена constraint на реальные из вашей схемы!
				if pqErr.Constraint == "users_email_key" {
					return ErrUserEmailConflict
				}
				if pqErr.Constraint == "users_nickname_key" {
					return ErrUserNicknameConflict // <--- ИСПОЛЬЗУЕТСЯ ЗДЕСЬ
				}
			case "23503": // foreign_key_violation
				// ЗАМЕНИТЕ имя constraint на реальное из вашей схемы!
				if pqErr.Constraint == "users_team_id_fkey" {
					return ErrUserTeamInvalid
				}
			}
		}
		return err
	}
	return nil
}

func (r *postgresUserRepository) GetByID(ctx context.Context, id int) (*models.User, error) {
	query := `
		SELECT
			u.id, u.first_name, u.last_name, u.nickname, u.email, u.password_hash, u.role, u.team_id, u.created_at,
			t.id, t.name, t.captain_id, t.sport_id, t.created_at
		FROM
			users u
		LEFT JOIN
			teams t ON u.team_id = t.id
		WHERE
			u.id = $1`

	row := r.db.QueryRowContext(ctx, query, id)

	var user models.User
	var team models.Team

	var teamID sql.NullInt64
	var teamName sql.NullString
	var teamCaptainID sql.NullInt64
	var teamSportID sql.NullInt64
	var teamCreatedAt sql.NullTime

	err := row.Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Nickname,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.TeamID,
		&user.CreatedAt,
		// Поля команды (могут быть NULL)
		&teamID,
		&teamName,
		&teamCaptainID,
		&teamSportID,
		&teamCreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to scan user with team: %w", err)
	}

	if teamID.Valid {
		team.ID = int(teamID.Int64)
		team.Name = teamName.String
		team.CaptainID = int(teamCaptainID.Int64)
		team.SportID = int(teamSportID.Int64)
		team.CreatedAt = teamCreatedAt.Time
		user.Team = &team
	}

	return &user, nil
}

//func (r *postgresUserRepository) GetByID(ctx context.Context, id int) (*models.User, error) {
//	query := `
//		SELECT id, first_name, last_name, nickname, email, password_hash, role, team_id, created_at
//		FROM users
//		WHERE id = $1`
//	return r.scanUser(ctx, query, id)
//}

func (r *postgresUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, first_name, last_name, nickname, email, password_hash, role, team_id, created_at
		FROM users
		WHERE email = $1`
	return r.scanUser(ctx, query, email)
}

func (r *postgresUserRepository) Update(ctx context.Context, user *models.User) error {
	query := `
		UPDATE users SET
			first_name = $1,
			last_name = $2,
			nickname = $3,
			email = $4,
			password_hash = $5,
			role = $6,
			team_id = $7
		WHERE id = $8`

	result, err := r.db.ExecContext(ctx, query,
		user.FirstName,
		user.LastName,
		user.Nickname,
		user.Email,
		user.PasswordHash,
		user.Role,
		user.TeamID,
		user.ID,
	)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			switch pqErr.Code {
			case "23505":
				if pqErr.Constraint == "users_email_key" {
					return ErrUserEmailConflict
				}
				if pqErr.Constraint == "users_nickname_key" {
					return ErrUserNicknameConflict // <--- ИСПОЛЬЗУЕТСЯ ЗДЕСЬ
				}
			case "23503":
				if pqErr.Constraint == "users_team_id_fkey" {
					return ErrUserTeamInvalid
				}
			}
		}
		return err
	}

	rowsAffected, checkErr := checkRowsAffected(result) // Используем общий хелпер
	if checkErr != nil {
		return checkErr
	}
	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *postgresUserRepository) Delete(ctx context.Context, id int) error {
	// Подумать о логике: что происходит с командами/участниками/матчами при удалении пользователя?
	// Возможно, потребуется мягкое удаление или проверка зависимостей в сервисном слое.
	// Пока реализуем простое удаление.
	query := `DELETE FROM users WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		// Ошибка FK может возникнуть, если пользователь является организатором турнира
		// или если нет каскадного удаления для связанных сущностей.
		// Обработка pqErr.Code == "23503" может быть добавлена здесь при необходимости.
		return err
	}

	rowsAffected, checkErr := checkRowsAffected(result)
	if checkErr != nil {
		return checkErr
	}
	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

// ListByTeamID возвращает список пользователей, принадлежащих к указанной команде.
func (r *postgresUserRepository) ListByTeamID(ctx context.Context, teamID int) ([]models.User, error) {
	query := `
		SELECT id, first_name, last_name, nickname, email, password_hash, role, team_id, created_at
		FROM users
		WHERE team_id = $1
		ORDER BY nickname ASC`

	rows, err := r.db.QueryContext(ctx, query, teamID)
	if err != nil {
		return nil, err // Ошибка выполнения запроса
	}
	defer rows.Close() // Гарантируем закрытие курсора

	users := make([]models.User, 0) // Инициализируем пустой слайс
	for rows.Next() {
		var user models.User
		// Сканируем все поля, включая team_id (который является *int в модели)
		scanErr := rows.Scan(
			&user.ID,
			&user.FirstName,
			&user.LastName,
			&user.Nickname,
			&user.Email,
			&user.PasswordHash,
			&user.Role,
			&user.TeamID, // Сканируем в указатель *int
			&user.CreatedAt,
		)
		if scanErr != nil {
			return nil, scanErr // Ошибка при сканировании строки
		}
		users = append(users, user)
	}

	// Проверяем ошибки, возникшие во время итерации
	if err = rows.Err(); err != nil {
		return nil, err
	}

	// Возвращаем слайс пользователей (может быть пустым, если команда пуста)
	return users, nil
}

// scanUser - вспомогательный метод для сканирования одного пользователя
func (r *postgresUserRepository) scanUser(ctx context.Context, query string, args ...interface{}) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Nickname,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.TeamID,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}
