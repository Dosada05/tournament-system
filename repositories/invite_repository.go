package repositories

import (
	"context"
	"database/sql"
	"errors"
	_ "time"

	"github.com/Dosada05/tournament-system/models"
	"github.com/lib/pq"
)

var (
	ErrInviteNotFound      = errors.New("invite not found")
	ErrInviteTokenConflict = errors.New("invite token conflict")
	ErrInviteTeamInvalid   = errors.New("invite team conflict or invalid")
)

// InviteRepository определяет интерфейс для работы с приглашениями.
type InviteRepository interface {
	// Create создает новое приглашение в базе данных.
	// Заполняет поля ID, CreatedAt, ExpiresAt у переданного объекта invite.
	Create(ctx context.Context, invite *models.Invite) error

	// GetByToken ищет приглашение по его уникальному токену.
	GetByToken(ctx context.Context, token string) (*models.Invite, error)

	// ListByTeamID возвращает список действующих (не удаленных) приглашений для команды.
	ListByTeamID(ctx context.Context, teamID int) ([]*models.Invite, error)

	// Delete удаляет приглашение по его ID.
	Delete(ctx context.Context, id int) error

	// DeleteExpired удаляет все приглашения, срок действия которых истек.
	// Возвращает количество удаленных приглашений и ошибку.
	DeleteExpired(ctx context.Context) (int64, error)
}

// postgresInviteRepository реализует InviteRepository для PostgreSQL.
type postgresInviteRepository struct {
	db *sql.DB
}

// NewPostgresInviteRepository создает новый экземпляр репозитория приглашений.
func NewPostgresInviteRepository(db *sql.DB) InviteRepository {
	return &postgresInviteRepository{db: db}
}

func (r *postgresInviteRepository) Create(ctx context.Context, invite *models.Invite) error {
	// Предполагаем, что ExpiresAt УЖЕ установлено в сервисном слое перед вызовом Create.
	// Если нет, можно установить здесь: invite.ExpiresAt = time.Now().Add(срок_действия)
	query := `
		INSERT INTO invites (team_id, token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at` // created_at берем из БД (DEFAULT)

	err := r.db.QueryRowContext(ctx, query,
		invite.TeamID,
		invite.Token,
		invite.ExpiresAt,
	).Scan(&invite.ID, &invite.CreatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			switch pqErr.Code {
			case "23505": // unique_violation
				// ЗАМЕНИТЕ имя constraint на реальное из вашей схемы!
				if pqErr.Constraint == "invites_token_key" {
					return ErrInviteTokenConflict
				}
			case "23503": // foreign_key_violation
				// ЗАМЕНИТЕ имя constraint на реальное из вашей схемы!
				if pqErr.Constraint == "invites_team_id_fkey" || pqErr.Constraint == "fk_invites_team" {
					return ErrInviteTeamInvalid
				}
			}
		}
		return err
	}

	return nil
}

func (r *postgresInviteRepository) GetByToken(ctx context.Context, token string) (*models.Invite, error) {
	query := `
		SELECT id, team_id, token, expires_at, created_at
		FROM invites
		WHERE token = $1`

	invite := &models.Invite{}
	err := r.db.QueryRowContext(ctx, query, token).Scan(
		&invite.ID,
		&invite.TeamID,
		&invite.Token, // Сканируем токен, хотя он уже есть
		&invite.ExpiresAt,
		&invite.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInviteNotFound
		}
		return nil, err
	}

	// Проверка на истечение срока действия должна быть в сервисном слое,
	// так как репозиторий должен просто вернуть найденные данные.
	// if time.Now().After(invite.ExpiresAt) {
	//     return nil, ErrInviteExpired // Или ErrInviteNotFound, если не хотим раскрывать, что он был, но истек
	// }

	return invite, nil
}

func (r *postgresInviteRepository) ListByTeamID(ctx context.Context, teamID int) ([]*models.Invite, error) {
	query := `
		SELECT id, team_id, token, expires_at, created_at
		FROM invites
		WHERE team_id = $1
		ORDER BY created_at DESC` // Сначала самые новые

	rows, err := r.db.QueryContext(ctx, query, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	invites := make([]*models.Invite, 0)
	for rows.Next() {
		var invite models.Invite
		if scanErr := rows.Scan(
			&invite.ID,
			&invite.TeamID,
			&invite.Token,
			&invite.ExpiresAt,
			&invite.CreatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		invites = append(invites, &invite)
	}

	if err = rows.Err(); err != nil { // Проверка после цикла
		return nil, err
	}

	return invites, nil
}

func (r *postgresInviteRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM invites WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		// Ошибки FK здесь не ожидаются при удалении
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrInviteNotFound
	}

	return nil
}

func (r *postgresInviteRepository) DeleteExpired(ctx context.Context) (int64, error) {
	query := `DELETE FROM invites WHERE expires_at <= NOW()`

	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		// Эта ошибка маловероятна при DELETE без WHERE id=..., но лучше обработать
		return 0, err
	}

	return rowsAffected, nil
}
