package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	ListByTournament(ctx context.Context, tournamentID int, statusFilter *models.ParticipantStatus, includeNested bool) ([]*models.Participant, error) // Добавлен флаг includeNested
	Delete(ctx context.Context, id int) error
	GetWithDetails(ctx context.Context, participantID int) (*models.Participant, error)
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
			case "23505": // unique_violation
				if pqErr.Constraint == "participants_user_id_tournament_id_key" ||
					pqErr.Constraint == "participants_team_id_tournament_id_key" {
					return ErrParticipantConflict
				}
			case "23503": // foreign_key_violation
				switch pqErr.Constraint {
				case "participants_user_id_fkey":
					return ErrParticipantUserInvalid
				case "participants_team_id_fkey":
					return ErrParticipantTeamInvalid
				case "participants_tournament_id_fkey":
					return ErrParticipantTournamentInvalid
				}
			case "23514": // check_violation
				if pqErr.Constraint == "chk_participant_type" {
					return ErrParticipantTypeViolation
				}
			}
		}
		return fmt.Errorf("failed to create participant: %w", err)
	}
	return nil
}

func (r *postgresParticipantRepository) UpdateStatus(ctx context.Context, id int, status models.ParticipantStatus) error {
	query := `UPDATE participants SET status = $1 WHERE id = $2`
	result, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update participant status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows for participant status update: %w", err)
	}
	if rowsAffected == 0 {
		return ErrParticipantNotFound
	}
	return nil
}

func (r *postgresParticipantRepository) scanParticipant(rowScanner interface {
	Scan(dest ...interface{}) error
}, p *models.Participant) error {
	return rowScanner.Scan(
		&p.ID,
		&p.UserID,
		&p.TeamID,
		&p.TournamentID,
		&p.Status,
		&p.CreatedAt,
	)
}

func (r *postgresParticipantRepository) findOne(ctx context.Context, query string, args ...interface{}) (*models.Participant, error) {
	p := &models.Participant{}
	row := r.db.QueryRowContext(ctx, query, args...)
	err := r.scanParticipant(row, p)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrParticipantNotFound
		}
		return nil, fmt.Errorf("failed to find participant: %w", err)
	}
	return p, nil
}

func (r *postgresParticipantRepository) FindByID(ctx context.Context, id int) (*models.Participant, error) {
	query := `SELECT id, user_id, team_id, tournament_id, status, created_at FROM participants WHERE id = $1`
	return r.findOne(ctx, query, id)
}

func (r *postgresParticipantRepository) FindByUserAndTournament(ctx context.Context, userID, tournamentID int) (*models.Participant, error) {
	query := `SELECT id, user_id, team_id, tournament_id, status, created_at FROM participants WHERE user_id = $1 AND tournament_id = $2`
	return r.findOne(ctx, query, userID, tournamentID)
}

func (r *postgresParticipantRepository) FindByTeamAndTournament(ctx context.Context, teamID, tournamentID int) (*models.Participant, error) {
	query := `SELECT id, user_id, team_id, tournament_id, status, created_at FROM participants WHERE team_id = $1 AND tournament_id = $2`
	return r.findOne(ctx, query, teamID, tournamentID)
}

func (r *postgresParticipantRepository) ListByTournament(ctx context.Context, tournamentID int, statusFilter *models.ParticipantStatus, includeNested bool) ([]*models.Participant, error) {
	var queryBuilder strings.Builder
	args := []interface{}{tournamentID}
	argCounter := 1

	queryBuilder.WriteString(fmt.Sprintf(`
		SELECT
			p.id, p.user_id, p.team_id, p.tournament_id, p.status, p.created_at
			%s
		FROM participants p
`, selectParticipantNestedFieldsSQL(includeNested)))

	if includeNested {
		queryBuilder.WriteString(joinParticipantNestedFieldsSQL())
	}

	queryBuilder.WriteString(fmt.Sprintf(" WHERE p.tournament_id = $%d", argCounter))
	argCounter++

	if statusFilter != nil {
		queryBuilder.WriteString(fmt.Sprintf(" AND p.status = $%d", argCounter))
		args = append(args, *statusFilter)
		argCounter++
	}
	queryBuilder.WriteString(" ORDER BY p.created_at ASC")

	rows, err := r.db.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list participants by tournament: %w", err)
	}
	defer rows.Close()

	participants := make([]*models.Participant, 0)
	for rows.Next() {
		var p models.Participant
		var u models.User
		var t models.Team
		scanDest := []interface{}{&p.ID, &p.UserID, &p.TeamID, &p.TournamentID, &p.Status, &p.CreatedAt}

		if includeNested {
			scanDest = append(scanDest,
				&u.ID, &u.FirstName, &u.LastName, &u.Nickname, &u.LogoKey, // User fields
				&t.ID, &t.Name, &t.LogoKey, // Team fields
			)
		}

		if err := rows.Scan(scanDest...); err != nil {
			return nil, fmt.Errorf("failed to scan participant row: %w", err)
		}

		if includeNested {
			if p.UserID != nil {
				if u.ID > 0 { // Check if user data was actually scanned
					p.User = &u
				}
			}
			if p.TeamID != nil {
				if t.ID > 0 { // Check if team data was actually scanned
					p.Team = &t
				}
			}
		}
		participants = append(participants, &p)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating participant rows: %w", err)
	}
	return participants, nil
}

func (r *postgresParticipantRepository) GetWithDetails(ctx context.Context, participantID int) (*models.Participant, error) {
	query := fmt.Sprintf(`
		SELECT
			p.id, p.user_id, p.team_id, p.tournament_id, p.status, p.created_at
			%s
		FROM participants p
		%s
		WHERE p.id = $1
	`, selectParticipantNestedFieldsSQL(true), joinParticipantNestedFieldsSQL())

	var p models.Participant
	var u models.User
	var t models.Team

	row := r.db.QueryRowContext(ctx, query, participantID)
	err := row.Scan(
		&p.ID, &p.UserID, &p.TeamID, &p.TournamentID, &p.Status, &p.CreatedAt,
		&u.ID, &u.FirstName, &u.LastName, &u.Nickname, &u.LogoKey,
		&t.ID, &t.Name, &t.LogoKey,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrParticipantNotFound
		}
		return nil, fmt.Errorf("failed to get participant with details: %w", err)
	}

	if p.UserID != nil && u.ID > 0 {
		p.User = &u
	}
	if p.TeamID != nil && t.ID > 0 {
		p.Team = &t
	}
	return &p, nil
}

func (r *postgresParticipantRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM participants WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete participant: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows for participant deletion: %w", err)
	}
	if rowsAffected == 0 {
		return ErrParticipantNotFound
	}
	return nil
}

func selectParticipantNestedFieldsSQL(includeNested bool) string {
	if !includeNested {
		return ""
	}
	return `,
		COALESCE(u.id, 0) as user_db_id, COALESCE(u.first_name, '') as user_first_name, COALESCE(u.last_name, '') as user_last_name, u.nickname as user_nickname, u.logo_key as user_logo_key,
		COALESCE(t.id, 0) as team_db_id, COALESCE(t.name, '') as team_name, t.logo_key as team_logo_key`
}

func joinParticipantNestedFieldsSQL() string {
	return `
		LEFT JOIN users u ON p.user_id = u.id
		LEFT JOIN teams t ON p.team_id = t.id`
}
