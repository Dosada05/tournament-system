package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Dosada05/tournament-system/models"
	"github.com/lib/pq"
)

var (
	ErrUserNotFound            = errors.New("user not found")
	ErrUserEmailConflict       = errors.New("user email conflict")
	ErrUserNicknameConflict    = errors.New("user nickname conflict")
	ErrUserTeamInvalid         = errors.New("user team conflict or invalid")
	ErrUserUpdateLogoKeyFailed = errors.New("failed to update user logo key")
)

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id int) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByConfirmationToken(ctx context.Context, token string) (*models.User, error)
	ConfirmEmail(ctx context.Context, userID int) error
	Update(ctx context.Context, user *models.User) error
	UpdateLogoKey(ctx context.Context, userID int, logoKey string) error
	Delete(ctx context.Context, id int) error
	ListByTeamID(ctx context.Context, teamID int) ([]models.User, error)
	List(ctx context.Context, filter models.UserFilter) ([]models.User, int, error)
	Count(ctx context.Context, filters map[string]interface{}) (int, error)
	SetPasswordResetToken(ctx context.Context, userID int, token string, expiresAt time.Time) error
	GetByPasswordResetToken(ctx context.Context, token string) (*models.User, error)
}

type postgresUserRepository struct {
	db *sql.DB
}

func NewPostgresUserRepository(db *sql.DB) UserRepository {
	return &postgresUserRepository{db: db}
}

func (r *postgresUserRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (first_name, last_name, nickname, email, password_hash, role, team_id, logo_key, email_confirmed, email_confirmation_token)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at`
	err := r.db.QueryRowContext(ctx, query,
		user.FirstName,
		user.LastName,
		user.Nickname,
		user.Email,
		user.PasswordHash,
		user.Role,
		user.TeamID,
		user.LogoKey,
		user.EmailConfirmed,
		user.EmailConfirmationToken,
	).Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == "23505" && strings.Contains(pqErr.Message, "email") {
				return ErrUserEmailConflict
			}
		}
		return err
	}
	return nil
}

func (r *postgresUserRepository) GetByID(ctx context.Context, id int) (*models.User, error) {
	query := `
		SELECT
			u.id, u.first_name, u.last_name, u.nickname, u.email, u.password_hash, u.role, u.team_id, u.logo_key, u.created_at,
			t.id, t.name, t.captain_id, t.sport_id, t.logo_key, t.created_at
		FROM users u
		LEFT JOIN teams t ON u.team_id = t.id
		WHERE u.id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	var user models.User
	var team models.Team
	var (
		teamID        sql.NullInt64
		teamName      sql.NullString
		teamCaptainID sql.NullInt64
		teamSportID   sql.NullInt64
		teamLogoKey   sql.NullString
		teamCreatedAt sql.NullTime
	)
	err := row.Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Nickname,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.TeamID,
		&user.LogoKey,
		&user.CreatedAt,
		&teamID,
		&teamName,
		&teamCaptainID,
		&teamSportID,
		&teamLogoKey,
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
		if teamLogoKey.Valid {
			team.LogoKey = &teamLogoKey.String
		}
		team.CreatedAt = teamCreatedAt.Time
		user.Team = &team
	}
	return &user, nil
}

func (r *postgresUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, first_name, last_name, nickname, email, password_hash, role, team_id, logo_key, created_at, email_confirmed, email_confirmation_token
		FROM users
		WHERE email = $1`
	return scanUserRow(ctx, r.db, query, email)
}

func (r *postgresUserRepository) GetByConfirmationToken(ctx context.Context, token string) (*models.User, error) {
	query := `SELECT id, first_name, last_name, nickname, email, password_hash, role, team_id, logo_key, created_at, email_confirmed, email_confirmation_token FROM users WHERE email_confirmation_token = $1`
	return scanUserRow(ctx, r.db, query, token)
}

func (r *postgresUserRepository) ConfirmEmail(ctx context.Context, userID int) error {
	query := `UPDATE users SET email_confirmed = TRUE, email_confirmation_token = NULL WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}
	return nil
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
			team_id = $7,
			logo_key = $8,
			email_confirmed = $9,
			email_confirmation_token = $10
		WHERE id = $11`
	result, err := r.db.ExecContext(ctx, query,
		user.FirstName,
		user.LastName,
		user.Nickname,
		user.Email,
		user.PasswordHash,
		user.Role,
		user.TeamID,
		user.LogoKey,
		user.EmailConfirmed,
		user.EmailConfirmationToken,
		user.ID,
	)
	if err != nil {
		return mapPQError(err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *postgresUserRepository) UpdateLogoKey(ctx context.Context, userID int, logoKey string) error {
	query := `UPDATE users SET logo_key = $1 WHERE id = $2`
	result, err := r.db.ExecContext(ctx, query, logoKey, userID)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUserUpdateLogoKeyFailed, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: failed to check affected rows: %w", ErrUserUpdateLogoKeyFailed, err)
	}
	if rows == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *postgresUserRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM users WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *postgresUserRepository) ListByTeamID(ctx context.Context, teamID int) ([]models.User, error) {
	query := `
		SELECT id, first_name, last_name, nickname, email, password_hash, role, team_id, logo_key, created_at
		FROM users
		WHERE team_id = $1
		ORDER BY nickname ASC`
	rows, err := r.db.QueryContext(ctx, query, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID,
			&user.FirstName,
			&user.LastName,
			&user.Nickname,
			&user.Email,
			&user.PasswordHash,
			&user.Role,
			&user.TeamID,
			&user.LogoKey,
			&user.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

func scanUserRow(ctx context.Context, db *sql.DB, query string, args ...interface{}) (*models.User, error) {
	user := &models.User{}
	err := db.QueryRowContext(ctx, query, args...).Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Nickname,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.TeamID,
		&user.LogoKey,
		&user.CreatedAt,
		&user.EmailConfirmed,
		&user.EmailConfirmationToken,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *postgresUserRepository) GetByPasswordResetToken(ctx context.Context, token string) (*models.User, error) {
	query := `SELECT id, first_name, last_name, nickname, email, password_hash, role, team_id, logo_key, created_at, email_confirmed, email_confirmation_token, password_reset_token, password_reset_expires_at FROM users WHERE password_reset_token = $1`
	return scanUserRowWithReset(ctx, r.db, query, token)
}

func scanUserRowWithReset(ctx context.Context, db *sql.DB, query string, args ...interface{}) (*models.User, error) {
	user := &models.User{}
	var (
		passwordResetToken     sql.NullString
		passwordResetExpiresAt sql.NullTime
	)
	err := db.QueryRowContext(ctx, query, args...).Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Nickname,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.TeamID,
		&user.LogoKey,
		&user.CreatedAt,
		&user.EmailConfirmed,
		&user.EmailConfirmationToken,
		&passwordResetToken,
		&passwordResetExpiresAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if passwordResetToken.Valid {
		user.PasswordResetToken = &passwordResetToken.String
	}
	if passwordResetExpiresAt.Valid {
		user.PasswordResetExpiresAt = &passwordResetExpiresAt.Time
	}
	return user, nil
}

func mapPQError(err error) error {
	if pqErr, ok := err.(*pq.Error); ok {
		switch pqErr.Code {
		case "23505":
			if pqErr.Constraint == "users_email_key" {
				return ErrUserEmailConflict
			}
			if pqErr.Constraint == "users_nickname_key" {
				return ErrUserNicknameConflict
			}
		case "23503":
			if pqErr.Constraint == "users_team_id_fkey" {
				return ErrUserTeamInvalid
			}
		}
	}
	return err
}

func (r *postgresUserRepository) List(ctx context.Context, filter models.UserFilter) ([]models.User, int, error) {
	var (
		users      []models.User
		args       []interface{}
		whereParts []string
		idx        = 1
	)

	query := `SELECT id, first_name, last_name, nickname, email, role, team_id, password_hash, created_at FROM users`
	countQuery := `SELECT count(*) FROM users`

	if filter.Search != "" {
		whereParts = append(whereParts,
			fmt.Sprintf("(LOWER(first_name) LIKE LOWER('%%' || $%d || '%%') OR LOWER(last_name) LIKE LOWER('%%' || $%d || '%%') OR LOWER(email) LIKE LOWER('%%' || $%d || '%%') OR LOWER(nickname) LIKE LOWER('%%' || $%d || '%%'))", idx, idx, idx, idx))
		args = append(args, filter.Search)
		idx++
	}
	if filter.Role != nil {
		whereParts = append(whereParts, fmt.Sprintf("role = $%d", idx))
		args = append(args, *filter.Role)
		idx++
	}
	if filter.Status != nil {
		whereParts = append(whereParts, fmt.Sprintf("status = $%d", idx))
		args = append(args, *filter.Status)
		idx++
	}
	if len(whereParts) > 0 {
		query += " WHERE " + strings.Join(whereParts, " AND ")
		countQuery += " WHERE " + strings.Join(whereParts, " AND ")
	}

	query += " ORDER BY id DESC"
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if filter.Page == 0 {
		filter.Page = 1
	}
	offset := (filter.Page - 1) * filter.Limit
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", filter.Limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.FirstName, &u.LastName, &u.Nickname, &u.Email, &u.Role, &u.TeamID, &u.PasswordHash, &u.CreatedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}

	var totalCount int
	err = r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}
	return users, totalCount, nil
}

func (r *postgresUserRepository) Count(ctx context.Context, filters map[string]interface{}) (int, error) {
	query := "SELECT COUNT(*) FROM users"
	var args []interface{}
	var where []string
	i := 1
	for k, v := range filters {
		where = append(where, fmt.Sprintf("%s = $%d", k, i))
		args = append(args, v)
		i++
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *postgresUserRepository) SetPasswordResetToken(ctx context.Context, userID int, token string, expiresAt time.Time) error {
	query := `UPDATE users SET password_reset_token = $1, password_reset_expires_at = $2 WHERE id = $3`
	result, err := r.db.ExecContext(ctx, query, token, expiresAt, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}
	return nil
}
