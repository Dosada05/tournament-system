package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	Create(ctx context.Context, exec SQLExecutor, match *models.TeamMatch) error
	GetByID(ctx context.Context, id int) (*models.TeamMatch, error)
	ListByTournament(ctx context.Context, tournamentID int, round *int, status *models.MatchStatus) ([]*models.TeamMatch, error)
	UpdateScoreStatusWinner(ctx context.Context, exec SQLExecutor, id int, score *string, status models.MatchStatus, winnerParticipantID *int) error
	Delete(ctx context.Context, id int) error
	UpdateNextMatchInfo(ctx context.Context, exec SQLExecutor, matchID int, nextMatchDBID *int, winnerToSlot *int) error
	UpdateParticipants(ctx context.Context, exec SQLExecutor, matchID int, t1ParticipantID *int, t2ParticipantID *int) error
}

type postgresTeamMatchRepository struct {
	db *sql.DB
}

func NewPostgresTeamMatchRepository(db *sql.DB) TeamMatchRepository {
	return &postgresTeamMatchRepository{db: db}
}

func (r *postgresTeamMatchRepository) getExecutor(exec SQLExecutor) SQLExecutor {
	if exec != nil {
		return exec
	}
	return r.db
}

func (r *postgresTeamMatchRepository) Create(ctx context.Context, exec SQLExecutor, match *models.TeamMatch) error {
	executor := r.getExecutor(exec)
	query := `
        INSERT INTO team_matches
            (tournament_id, t1_participant_id, t2_participant_id, score, match_time, 
             status, winner_participant_id, round, bracket_match_uid, next_match_db_id, winner_to_slot)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        RETURNING id, created_at`

	err := executor.QueryRowContext(ctx, query,
		match.TournamentID,
		match.T1ParticipantID,
		match.T2ParticipantID,
		match.Score,
		match.MatchTime,
		match.Status,
		match.WinnerParticipantID,
		match.Round,
		match.BracketMatchUID,
		match.NextMatchDBID,
		match.WinnerToSlot,
	).Scan(&match.ID, &match.CreatedAt)

	return r.handleTeamMatchError(err)
}

func (r *postgresTeamMatchRepository) GetByID(ctx context.Context, id int) (*models.TeamMatch, error) {
	executor := r.getExecutor(nil)
	query := `
		SELECT id, tournament_id, t1_participant_id, t2_participant_id, score, match_time, status, 
		       winner_participant_id, round, created_at, bracket_match_uid, next_match_db_id, winner_to_slot
		FROM team_matches
		WHERE id = $1`

	match := &models.TeamMatch{}
	err := executor.QueryRowContext(ctx, query, id).Scan(
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
		&match.BracketMatchUID,
		&match.NextMatchDBID,
		&match.WinnerToSlot,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTeamMatchNotFound
		}
		return nil, fmt.Errorf("failed to scan team match by id %d: %w", id, err)
	}
	return match, nil
}

func (r *postgresTeamMatchRepository) ListByTournament(ctx context.Context, tournamentID int, roundFilter *int, statusFilter *models.MatchStatus) ([]*models.TeamMatch, error) {
	executor := r.getExecutor(nil)
	var queryBuilder strings.Builder
	queryBuilder.WriteString(`
		SELECT id, tournament_id, t1_participant_id, t2_participant_id, score, match_time, status, 
		       winner_participant_id, round, created_at, bracket_match_uid, next_match_db_id, winner_to_slot
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
	}

	queryBuilder.WriteString(" ORDER BY round ASC, COALESCE(bracket_match_uid, ''), id ASC")

	finalQuery := queryBuilder.String()
	rows, err := executor.QueryContext(ctx, finalQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query team matches for tournament %d: %w", tournamentID, err)
	}
	defer rows.Close()

	matches := make([]*models.TeamMatch, 0)
	for rows.Next() {
		var match models.TeamMatch
		if scanErr := rows.Scan(
			&match.ID, &match.TournamentID, &match.T1ParticipantID, &match.T2ParticipantID,
			&match.Score, &match.MatchTime, &match.Status, &match.WinnerParticipantID,
			&match.Round, &match.CreatedAt, &match.BracketMatchUID, &match.NextMatchDBID, &match.WinnerToSlot,
		); scanErr != nil {
			return nil, fmt.Errorf("failed to scan team match row: %w", scanErr)
		}
		matches = append(matches, &match)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during team match rows iteration: %w", err)
	}

	return matches, nil
}

func (r *postgresTeamMatchRepository) UpdateScoreStatusWinner(ctx context.Context, exec SQLExecutor, id int, score *string, status models.MatchStatus, winnerParticipantID *int) error {
	executor := r.getExecutor(exec)
	query := `
		UPDATE team_matches
		SET score = $1, status = $2, winner_participant_id = $3
		WHERE id = $4`

	result, err := executor.ExecContext(ctx, query, score, status, winnerParticipantID, id)
	if err != nil {
		return r.handleTeamMatchError(err)
	}
	return checkAffectedRows(result, ErrTeamMatchNotFound)
}

func (r *postgresTeamMatchRepository) Delete(ctx context.Context, id int) error {
	executor := r.getExecutor(nil)
	query := `DELETE FROM team_matches WHERE id = $1`
	result, err := executor.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	return checkAffectedRows(result, ErrTeamMatchNotFound)
}

func (r *postgresTeamMatchRepository) UpdateNextMatchInfo(ctx context.Context, exec SQLExecutor, matchID int, nextMatchDBID *int, winnerToSlot *int) error {
	executor := r.getExecutor(exec)
	query := `UPDATE team_matches SET next_match_db_id = $1, winner_to_slot = $2 WHERE id = $3`
	result, err := executor.ExecContext(ctx, query, nextMatchDBID, winnerToSlot, matchID)
	if err != nil {
		return fmt.Errorf("UpdateNextMatchInfo: failed to execute query for team_match %d: %w", matchID, err)
	}
	return checkAffectedRows(result, ErrTeamMatchNotFound)
}

func (r *postgresTeamMatchRepository) UpdateParticipants(ctx context.Context, exec SQLExecutor, matchID int, t1ParticipantID *int, t2ParticipantID *int) error {
	executor := r.getExecutor(exec)
	query := `UPDATE team_matches SET t1_participant_id = $1, t2_participant_id = $2 WHERE id = $3`
	result, err := executor.ExecContext(ctx, query, t1ParticipantID, t2ParticipantID, matchID)
	if err != nil {
		return fmt.Errorf("UpdateParticipants: failed to execute query for team_match %d: %w", matchID, err)
	}
	return checkAffectedRows(result, ErrTeamMatchNotFound)
}

func (r *postgresTeamMatchRepository) handleTeamMatchError(err error) error {
	if err == nil {
		return nil
	}
	if pqErr, ok := err.(*pq.Error); ok {
		switch pqErr.Constraint {
		case "team_matches_tournament_id_fkey":
			return ErrTeamMatchTournamentInvalid
		case "team_matches_t1_participant_id_fkey", "team_matches_t2_participant_id_fkey":
			return ErrTeamMatchParticipantInvalid
		case "team_matches_winner_participant_id_fkey":
			return ErrTeamMatchWinnerParticipantInvalid
		case "team_matches_bracket_match_uid_key":
			return fmt.Errorf("bracket_match_uid conflict: %w", err)
		}
	}
	return err
}
