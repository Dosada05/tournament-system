package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	_ "time"

	"github.com/Dosada05/tournament-system/models"
	"github.com/lib/pq"
)

var (
	ErrInviteNotFound      = errors.New("invite not found")
	ErrInviteTokenConflict = errors.New("invite token conflict")
	ErrInviteTeamInvalid   = errors.New("invalid team for invite") // Ошибка FK
)

type InviteRepository interface {
	Create(ctx context.Context, invite *models.Invite) error
	GetByToken(ctx context.Context, token string) (*models.Invite, error)
	GetValidByTeamID(ctx context.Context, teamID int) (*models.Invite, error)
	Update(ctx context.Context, invite *models.Invite) error
	DeleteByTeamID(ctx context.Context, teamID int) (int64, error) // Возвращает кол-во удаленных
	CleanupExpired(ctx context.Context) (int64, error)             // Возвращает кол-во удаленных
}

type postgresInviteRepository struct {
	db *sql.DB
}

func NewPostgresInviteRepository(db *sql.DB) InviteRepository {
	return &postgresInviteRepository{db: db}
}

func (r *postgresInviteRepository) Create(ctx context.Context, invite *models.Invite) error {
	query := `
		INSERT INTO invites (team_id, token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, query,
		invite.TeamID,
		invite.Token,
		invite.ExpiresAt,
	).Scan(&invite.ID, &invite.CreatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			switch pqErr.Code {
			case "23505": // unique_violation
				// Предполагаем, что уникальный constraint на token называется invites_token_key
				if pqErr.Constraint == "invites_token_key" {
					return ErrInviteTokenConflict
				}
			case "23503": // foreign_key_violation
				// Предполагаем, что FK constraint на team_id называется invites_team_id_fkey
				if pqErr.Constraint == "invites_team_id_fkey" {
					return ErrInviteTeamInvalid
				}
			}
		}
		return fmt.Errorf("failed to create invite: %w", err)
	}
	return nil
}

func (r *postgresInviteRepository) GetByToken(ctx context.Context, token string) (*models.Invite, error) {
	query := `
		SELECT id, team_id, token, expires_at, created_at
		FROM invites
		WHERE token = $1`

	row := r.db.QueryRowContext(ctx, query, token)
	invite := &models.Invite{}

	err := row.Scan(
		&invite.ID,
		&invite.TeamID,
		&invite.Token,
		&invite.ExpiresAt,
		&invite.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInviteNotFound
		}
		return nil, fmt.Errorf("failed to get invite by token: %w", err)
	}
	return invite, nil
}

func (r *postgresInviteRepository) GetValidByTeamID(ctx context.Context, teamID int) (*models.Invite, error) {
	query := `
		SELECT id, team_id, token, expires_at, created_at
		FROM invites
		WHERE team_id = $1 AND expires_at > NOW()
		ORDER BY created_at DESC
		LIMIT 1`

	row := r.db.QueryRowContext(ctx, query, teamID)
	invite := &models.Invite{}

	err := row.Scan(
		&invite.ID,
		&invite.TeamID,
		&invite.Token,
		&invite.ExpiresAt,
		&invite.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInviteNotFound // Не найдено валидных приглашений для команды
		}
		return nil, fmt.Errorf("failed to get valid invite by team id: %w", err)
	}
	return invite, nil
}

func (r *postgresInviteRepository) Update(ctx context.Context, invite *models.Invite) error {
	query := `
		UPDATE invites SET
			token = $1,
			expires_at = $2
		WHERE id = $3 AND team_id = $4` // Обновляем только токен и время жизни

	result, err := r.db.ExecContext(ctx, query,
		invite.Token,
		invite.ExpiresAt,
		invite.ID,
		invite.TeamID, // Доп. проверка, что мы обновляем инвайт нужной команды
	)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == "23505" && pqErr.Constraint == "invites_token_key" {
				return ErrInviteTokenConflict
			}
		}
		return fmt.Errorf("failed to update invite: %w", err)
	}

	return checkAffectedRows(result, ErrInviteNotFound)
}

func (r *postgresInviteRepository) DeleteByTeamID(ctx context.Context, teamID int) (int64, error) {
	query := `DELETE FROM invites WHERE team_id = $1`
	result, err := r.db.ExecContext(ctx, query, teamID)
	if err != nil {
		return 0, fmt.Errorf("failed to delete invites by team id: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to check affected rows on invite delete by team id: %w", err)
	}
	return rowsAffected, nil
}

func (r *postgresInviteRepository) CleanupExpired(ctx context.Context) (int64, error) {
	query := `DELETE FROM invites WHERE expires_at <= NOW()`
	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired invites: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to check affected rows on expired invite cleanup: %w", err)
	}
	return rowsAffected, nil
}
