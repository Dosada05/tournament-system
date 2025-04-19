package repositories

import (
	"database/sql"
	"errors"
	"github.com/Dosada05/tournament-system/config"
	"github.com/Dosada05/tournament-system/models"
)

func CreateUser(user *models.User) error {
	query := `INSERT INTO users (first_name, last_name, role, email, password_hash)
              VALUES ($1, $2, $3, $4, $5)`
	_, err := config.DB.Exec(query, user.FirstName, user.LastName, user.Role, user.Email, user.PasswordHash)
	return err
}

func GetUserByEmail(email string) (*models.User, error) {
	query := `SELECT id, first_name, last_name, nickname, team_id, role, email, password_hash, created_at
              FROM users WHERE email = $1`

	row := config.DB.QueryRow(query, email)

	var user models.User

	if err := row.Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Nickname,
		&user.TeamID,
		&user.Role,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt); err != nil {

		return nil, err
	}

	return &user, nil
}

func GetUserByID(id int) (*models.User, error) {
	query := `SELECT id, first_name, last_name, nickname, team_id, role, email, password_hash, created_at
              FROM users WHERE id = $1`
	row := config.DB.QueryRow(query, id)

	var user models.User
	if err := row.Scan(&user.ID, &user.FirstName, &user.LastName, &user.Nickname, &user.TeamID, &user.Role, &user.Email, &user.PasswordHash, &user.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	return &user, nil
}

// Поменять
func UpdateUser(id int, user *models.User) error {
	query := `UPDATE users SET first_name = $1, last_name = $2, nickname = $3, team_id = $4, role = $5, email = $6, password_hash = $7 WHERE id = $8`
	_, err := config.DB.Exec(query, user.FirstName, user.LastName, user.Nickname, user.TeamID, user.Role, user.Email, user.PasswordHash, id)
	return err
}

func DeleteUser(id int) error {
	query := `DELETE FROM users WHERE id = $1`
	_, err := config.DB.Exec(query, id)
	return err
}
