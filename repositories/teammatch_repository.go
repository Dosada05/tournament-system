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
	ErrTeamMatchNotFound                 = errors.New("team match not found")
	ErrTeamMatchTournamentInvalid        = errors.New("team match tournament conflict or invalid")
	ErrTeamMatchParticipantInvalid       = errors.New("team match participant conflict or invalid")
	ErrTeamMatchWinnerParticipantInvalid = errors.New("team match winner participant conflict or invalid")
)

type TeamMatchRepository interface {
	Create(ctx context.Context, match *models.TeamMatch) error
	GetByID(ctx context.Context, id int) (*models.TeamMatch, error)
	ListByTournament(ctx context.Context, tournamentID int, round *int, status *models.MatchStatus) ([]*models.TeamMatch, error)
	UpdateScoreStatusWinner(ctx context.Context, id int, score *string, status models.MatchStatus, winnerParticipantID *int) error
	Delete(ctx context.Context, id int) error
}

type postgresTeamMatchRepository struct {
	db *sql.DB
}

func NewPostgresTeamMatchRepository(db *sql.DB) TeamMatchRepository {
	return &postgresTeamMatchRepository{db: db}
}

func (r *postgresTeamMatchRepository) Create(ctx context.Context, match *models.TeamMatch) error {
	query := `
		INSERT INTO team_matches
			(tournament_id, t1_participant_id, t2_participant_id, score, match_time, status, winner_participant_id, round)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, query,
		match.TournamentID,
		match.T1ParticipantID,
		match.T2ParticipantID,
		match.Score,
		match.MatchTime,
		match.Status,
		match.WinnerParticipantID,
		match.Round,
	).Scan(&match.ID, &match.CreatedAt)

	return r.handleTeamMatchError(err)
}

func (r *postgresTeamMatchRepository) GetByID(ctx context.Context, id int) (*models.TeamMatch, error) {
	query := `
		SELECT id, tournament_id, t1_participant_id, t2_participant_id, score, match_time, status, winner_participant_id, round, created_at
		FROM team_matches
		WHERE id = $1`

	match := &models.TeamMatch{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&match.ID,
		&match.TournamentID,
		&match.T1ParticipantID,
		&match.T2ParticipantID,
		&match.Score,
		&match.MatchTime,
		&match.Status,
		&match.WinnerParticipantID,
		&match.Round,
		&match.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTeamMatchNotFound
		}
		return nil, err
	}
	return match, nil
}

func (r *postgresTeamMatchRepository) ListByTournament(ctx context.Context, tournamentID int, roundFilter *int, statusFilter *models.MatchStatus) ([]*models.TeamMatch, error) {
	var queryBuilder strings.Builder
	queryBuilder.WriteString(`
		SELECT id, tournament_id, t1_participant_id, t2_participant_id, score, match_time, status, winner_participant_id, round, created_at
		FROM team_matches
		WHERE tournament_id = $1`)

	args := []interface{}{tournamentID}
	placeholderIndex := 2

	if roundFilter != nil {
		queryBuilder.WriteString(" AND round = $")
		queryBuilder.WriteString(strconv.Itoa(placeholderIndex))
		args = append(args, *roundFilter)
		placeholderIndex++
	}

	if statusFilter != nil {
		queryBuilder.WriteString(" AND status = $")
		queryBuilder.WriteString(strconv.Itoa(placeholderIndex))
		args = append(args, *statusFilter)
		placeholderIndex++
	}

	queryBuilder.WriteString(" ORDER BY round ASC, match_time ASC")

	finalQuery := queryBuilder.String()
	rows, err := r.db.QueryContext(ctx, finalQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	matches := make([]*models.TeamMatch, 0)
	for rows.Next() {
		var match models.TeamMatch
		if scanErr := rows.Scan(
			&match.ID,
			&match.TournamentID,
			&match.T1ParticipantID,
			&match.T2ParticipantID,
			&match.Score,
			&match.MatchTime,
			&match.Status,
			&match.WinnerParticipantID,
			&match.Round,
			&match.CreatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		matches = append(matches, &match)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return matches, nil
}

func (r *postgresTeamMatchRepository) UpdateScoreStatusWinner(ctx context.Context, id int, score *string, status models.MatchStatus, winnerParticipantID *int) error {
	query := `
		UPDATE team_matches
		SET score = $1, status = $2, winner_participant_id = $3
		WHERE id = $4`

	result, err := r.db.ExecContext(ctx, query, score, status, winnerParticipantID, id)
	if err != nil {
		return r.handleTeamMatchError(err)
	}

	rowsAffected, checkErr := checkRowsAffected(result) // Используем тот же общий хелпер
	if checkErr != nil {
		return checkErr
	}
	if rowsAffected == 0 {
		return ErrTeamMatchNotFound
	}

	return nil
}

func (r *postgresTeamMatchRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM team_matches WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, checkErr := checkRowsAffected(result)
	if checkErr != nil {
		return checkErr
	}
	if rowsAffected == 0 {
		return ErrTeamMatchNotFound
	}

	return nil
}

func (r *postgresTeamMatchRepository) handleTeamMatchError(err error) error {
	if err == nil {
		return nil
	}
	if pqErr, ok := err.(*pq.Error); ok {
		if pqErr.Code == "23503" { // foreign_key_violation
			// ЗАМЕНИТЕ имена constraint на реальные для team_matches!
			switch pqErr.Constraint {
			case "team_matches_tournament_id_fkey":
				return ErrTeamMatchTournamentInvalid
			case "team_matches_t1_participant_id_fkey", "team_matches_t2_participant_id_fkey":
				return ErrTeamMatchParticipantInvalid
			case "team_matches_winner_participant_id_fkey":
				return ErrTeamMatchWinnerParticipantInvalid
			}
		}
	}
	return err
}
