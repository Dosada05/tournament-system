package repositories

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"github.com/Dosada05/tournament-system/models"
	"github.com/lib/pq"
)

var (
	ErrParticipantNotFound          = errors.New("participant not found")
	ErrParticipantConflict          = errors.New("participant conflict: user or team already registered for this tournament")
	ErrParticipantUserInvalid       = errors.New("participant user conflict or invalid")
	ErrParticipantTeamInvalid       = errors.New("participant team conflict or invalid")
	ErrParticipantTournamentInvalid = errors.New("participant tournament conflict or invalid")
	ErrParticipantTypeViolation     = errors.New("participant type violation: either user_id or team_id must be set, but not both")
)

type ParticipantRepository interface {
	Create(ctx context.Context, p *models.Participant) error
	UpdateStatus(ctx context.Context, id int, status models.ParticipantStatus) error
	FindByID(ctx context.Context, id int) (*models.Participant, error)
	FindByUserAndTournament(ctx context.Context, userID, tournamentID int) (*models.Participant, error)
	FindByTeamAndTournament(ctx context.Context, teamID, tournamentID int) (*models.Participant, error)
	ListByTournament(ctx context.Context, tournamentID int, status *models.ParticipantStatus) ([]*models.Participant, error)
	Delete(ctx context.Context, id int) error
}

type postgresParticipantRepository struct {
	db *sql.DB
}

func NewPostgresParticipantRepository(db *sql.DB) ParticipantRepository {
	return &postgresParticipantRepository{db: db}
}

func (r *postgresParticipantRepository) Create(ctx context.Context, p *models.Participant) error {
	query := `
		INSERT INTO participants (user_id, team_id, tournament_id, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, query,
		p.UserID,
		p.TeamID,
		p.TournamentID,
		p.Status,
	).Scan(&p.ID, &p.CreatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			switch pqErr.Code {
			case "23505":
				if pqErr.Constraint == "participants_user_id_tournament_id_key" ||
					pqErr.Constraint == "participants_team_id_tournament_id_key" {
					return ErrParticipantConflict
				}
			case "23503":
				switch pqErr.Constraint {
				case "participants_user_id_fkey", "fk_participants_user":
					return ErrParticipantUserInvalid
				case "participants_team_id_fkey", "fk_participants_team":
					return ErrParticipantTeamInvalid
				case "participants_tournament_id_fkey", "fk_participants_tournament":
					return ErrParticipantTournamentInvalid
				}
			case "23514":
				if pqErr.Constraint == "chk_participant_type" {
					return ErrParticipantTypeViolation
				}
			}
		}
		return err
	}

	return nil
}

func (r *postgresParticipantRepository) UpdateStatus(ctx context.Context, id int, status models.ParticipantStatus) error {
	query := `UPDATE participants SET status = $1 WHERE id = $2`

	result, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrParticipantNotFound
	}

	return nil
}

func (r *postgresParticipantRepository) findOne(ctx context.Context, query string, args ...interface{}) (*models.Participant, error) {
	p := &models.Participant{}
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&p.ID,
		&p.UserID,
		&p.TeamID,
		&p.TournamentID,
		&p.Status,
		&p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrParticipantNotFound
		}
		return nil, err
	}
	return p, nil
}

func (r *postgresParticipantRepository) FindByID(ctx context.Context, id int) (*models.Participant, error) {
	query := `
		SELECT id, user_id, team_id, tournament_id, status, created_at
		FROM participants
		WHERE id = $1`
	return r.findOne(ctx, query, id)
}

func (r *postgresParticipantRepository) FindByUserAndTournament(ctx context.Context, userID, tournamentID int) (*models.Participant, error) {
	query := `
		SELECT id, user_id, team_id, tournament_id, status, created_at
		FROM participants
		WHERE user_id = $1 AND tournament_id = $2`
	return r.findOne(ctx, query, userID, tournamentID)
}

func (r *postgresParticipantRepository) FindByTeamAndTournament(ctx context.Context, teamID, tournamentID int) (*models.Participant, error) {
	query := `
		SELECT id, user_id, team_id, tournament_id, status, created_at
		FROM participants
		WHERE team_id = $1 AND tournament_id = $2`
	return r.findOne(ctx, query, teamID, tournamentID)
}

func (r *postgresParticipantRepository) ListByTournament(ctx context.Context, tournamentID int, statusFilter *models.ParticipantStatus) ([]*models.Participant, error) {
	var queryBuilder strings.Builder
	queryBuilder.WriteString(`
		SELECT id, user_id, team_id, tournament_id, status, created_at
		FROM participants
		WHERE tournament_id = $1`)

	args := []interface{}{tournamentID}
	placeholderIndex := 2 // Следующий плейсхолдер будет $2

	if statusFilter != nil {
		queryBuilder.WriteString(" AND status = $")
		queryBuilder.WriteString(strconv.Itoa(placeholderIndex)) // Добавляем номер плейсхолдера
		args = append(args, *statusFilter)
		placeholderIndex++ // Увеличиваем для следующего возможного плейсхолдера
	}

	// Добавляем ORDER BY с гарантированным пробелом перед ним
	queryBuilder.WriteString(" ORDER BY created_at ASC")

	finalQuery := queryBuilder.String()
	rows, err := r.db.QueryContext(ctx, finalQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	participants := make([]*models.Participant, 0)
	for rows.Next() {
		var p models.Participant
		if scanErr := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.TeamID,
			&p.TournamentID,
			&p.Status,
			&p.CreatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		participants = append(participants, &p)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return participants, nil
}

func (r *postgresParticipantRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM participants WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrParticipantNotFound
	}

	return nil
}
