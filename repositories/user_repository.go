package repositories

import (
	"database/sql"
	"errors"
	"time"

	"github.com/Dosada05/tournament-system/models"
)

type UserRepository interface {
	Create(user *models.User) error
	GetByEmail(email string) (*models.User, error)
	GetByID(id int) (*models.User, error)
	Update(id int, user *models.User) error
	Delete(id int) error
}

type userRepository struct {
	db *sql.DB
}

// Конструктор
func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(user *models.User) error {
	query := `INSERT INTO users (first_name, last_name, role, email, password_hash)
              VALUES ($1, $2, $3, $4, $5)`
	_, err := r.db.Exec(query, user.FirstName, user.LastName, user.Role, user.Email, user.PasswordHash)
	return err
}

func (r *userRepository) GetByEmail(email string) (*models.User, error) {
	query := `SELECT id, first_name, last_name, nickname, team_id, role, email, password_hash, created_at
              FROM users WHERE email = $1`

	row := r.db.QueryRow(query, email)
	var user models.User
	var createdAt time.Time
	if err := row.Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Nickname,
		&user.TeamID,
		&user.Role,
		&user.Email,
		&user.PasswordHash,
		&createdAt); err != nil {
		return nil, err
	}
	user.CreatedAt = createdAt
	return &user, nil
}

func (r *userRepository) GetByID(id int) (*models.User, error) {
	query := `SELECT id, first_name, last_name, nickname, team_id, role, email, password_hash, created_at
              FROM users WHERE id = $1`
	row := r.db.QueryRow(query, id)
	var user models.User
	var createdAt time.Time
	if err := row.Scan(
		&user.ID, &user.FirstName, &user.LastName, &user.Nickname, &user.TeamID,
		&user.Role, &user.Email, &user.PasswordHash, &createdAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	user.CreatedAt = createdAt
	return &user, nil
}

func (r *userRepository) Update(id int, user *models.User) error {
	query := `UPDATE users SET first_name = $1, last_name = $2, nickname = $3, team_id = $4, role = $5, email = $6, password_hash = $7 WHERE id = $8`
	_, err := r.db.Exec(query, user.FirstName, user.LastName, user.Nickname, user.TeamID, user.Role, user.Email, user.PasswordHash, id)
	return err
}

func (r *userRepository) Delete(id int) error {
	query := `DELETE FROM users WHERE id = $1`
	_, err := r.db.Exec(query, id)
	return err
}
