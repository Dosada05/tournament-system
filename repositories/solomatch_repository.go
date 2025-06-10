package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/Dosada05/tournament-system/models"
	"github.com/lib/pq"
	"strconv"
	"strings"
)

type SQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

var (
	ErrSoloMatchNotFound                 = errors.New("solo match not found")
	ErrSoloMatchTournamentInvalid        = errors.New("solo match tournament conflict or invalid")
	ErrSoloMatchParticipantInvalid       = errors.New("solo match participant conflict or invalid")
	ErrSoloMatchWinnerParticipantInvalid = errors.New("solo match winner participant conflict or invalid")
)

type SoloMatchRepository interface {
	Create(ctx context.Context, exec SQLExecutor, match *models.SoloMatch) error
	GetByID(ctx context.Context, id int) (*models.SoloMatch, error)
	ListByTournament(ctx context.Context, tournamentID int, round *int, status *models.MatchStatus) ([]*models.SoloMatch, error)
	UpdateScoreStatusWinner(ctx context.Context, exec SQLExecutor, id int, score *string, status models.MatchStatus, winnerParticipantID *int) error
	Delete(ctx context.Context, id int) error
	UpdateNextMatchInfo(ctx context.Context, exec SQLExecutor, matchID int, nextMatchDBID *int, winnerToSlot *int) error
	UpdateParticipants(ctx context.Context, exec SQLExecutor, matchID int, p1ParticipantID *int, p2ParticipantID *int) error
	CountSoloMatches(ctx context.Context, filters map[string]interface{}) (int, error)
}

type postgresSoloMatchRepository struct {
	db *sql.DB
}

func NewPostgresSoloMatchRepository(db *sql.DB) SoloMatchRepository {
	return &postgresSoloMatchRepository{db: db}
}

func (r *postgresSoloMatchRepository) getExecutor(exec SQLExecutor) SQLExecutor {
	if exec != nil {
		return exec
	}
	return r.db
}

func (r *postgresSoloMatchRepository) Create(ctx context.Context, exec SQLExecutor, match *models.SoloMatch) error {
	executor := r.getExecutor(exec)
	query := `
		INSERT INTO solo_matches
			(tournament_id, p1_participant_id, p2_participant_id, score, match_time, 
			 status, winner_participant_id, round, bracket_match_uid, next_match_db_id, winner_to_slot)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at`

	err := executor.QueryRowContext(ctx, query,
		match.TournamentID,
		match.P1ParticipantID,
		match.P2ParticipantID,
		match.Score,
		match.MatchTime,
		match.Status,
		match.WinnerParticipantID,
		match.Round,
		match.BracketMatchUID,
		match.NextMatchDBID,
		match.WinnerToSlot,
	).Scan(&match.ID, &match.CreatedAt)

	return r.handleSoloMatchError(err)
}

func (r *postgresSoloMatchRepository) GetByID(ctx context.Context, id int) (*models.SoloMatch, error) {
	executor := r.getExecutor(nil) // GetByID typically doesn't run in an existing tx from service layer
	query := `
		SELECT id, tournament_id, p1_participant_id, p2_participant_id, score, match_time, status, 
		       winner_participant_id, round, created_at, bracket_match_uid, next_match_db_id, winner_to_slot
		FROM solo_matches
		WHERE id = $1`

	match := &models.SoloMatch{}
	err := executor.QueryRowContext(ctx, query, id).Scan(
		&match.ID,
		&match.TournamentID,
		&match.P1ParticipantID,
		&match.P2ParticipantID,
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
			return nil, ErrSoloMatchNotFound
		}
		return nil, fmt.Errorf("failed to scan solo match by id %d: %w", id, err)
	}
	return match, nil
}

func (r *postgresSoloMatchRepository) ListByTournament(ctx context.Context, tournamentID int, roundFilter *int, statusFilter *models.MatchStatus) ([]*models.SoloMatch, error) {
	executor := r.getExecutor(nil)
	var queryBuilder strings.Builder
	queryBuilder.WriteString(`
		SELECT id, tournament_id, p1_participant_id, p2_participant_id, score, match_time, status, 
		       winner_participant_id, round, created_at, bracket_match_uid, next_match_db_id, winner_to_slot
		FROM solo_matches
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
		return nil, fmt.Errorf("failed to query solo matches for tournament %d: %w", tournamentID, err)
	}
	defer rows.Close()

	matches := make([]*models.SoloMatch, 0)
	for rows.Next() {
		var match models.SoloMatch
		if scanErr := rows.Scan(
			&match.ID, &match.TournamentID, &match.P1ParticipantID, &match.P2ParticipantID,
			&match.Score, &match.MatchTime, &match.Status, &match.WinnerParticipantID,
			&match.Round, &match.CreatedAt, &match.BracketMatchUID, &match.NextMatchDBID, &match.WinnerToSlot,
		); scanErr != nil {
			return nil, fmt.Errorf("failed to scan solo match row: %w", scanErr)
		}
		matches = append(matches, &match)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during solo match rows iteration: %w", err)
	}
	return matches, nil
}

func (r *postgresSoloMatchRepository) UpdateScoreStatusWinner(ctx context.Context, exec SQLExecutor, id int, score *string, status models.MatchStatus, winnerParticipantID *int) error {
	executor := r.getExecutor(exec)
	query := `
		UPDATE solo_matches
		SET score = $1, status = $2, winner_participant_id = $3
		WHERE id = $4`

	result, err := executor.ExecContext(ctx, query, score, status, winnerParticipantID, id)
	if err != nil {
		return r.handleSoloMatchError(err)
	}
	return r.checkAffectedRows(result, ErrSoloMatchNotFound)
}

func (r *postgresSoloMatchRepository) Delete(ctx context.Context, id int) error {
	executor := r.getExecutor(nil)
	query := `DELETE FROM solo_matches WHERE id = $1`
	result, err := executor.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	return r.checkAffectedRows(result, ErrSoloMatchNotFound)
}

func (r *postgresSoloMatchRepository) UpdateNextMatchInfo(ctx context.Context, exec SQLExecutor, matchID int, nextMatchDBID *int, winnerToSlot *int) error {
	executor := r.getExecutor(exec)
	query := `UPDATE solo_matches SET next_match_db_id = $1, winner_to_slot = $2 WHERE id = $3`
	result, err := executor.ExecContext(ctx, query, nextMatchDBID, winnerToSlot, matchID)
	if err != nil {
		return fmt.Errorf("UpdateNextMatchInfo: failed to execute query for solo_match %d: %w", matchID, err)
	}
	return r.checkAffectedRows(result, ErrSoloMatchNotFound)
}

func (r *postgresSoloMatchRepository) UpdateParticipants(ctx context.Context, exec SQLExecutor, matchID int, p1ParticipantID *int, p2ParticipantID *int) error {
	executor := r.getExecutor(exec)
	query := `UPDATE solo_matches SET p1_participant_id = $1, p2_participant_id = $2 WHERE id = $3`
	result, err := executor.ExecContext(ctx, query, p1ParticipantID, p2ParticipantID, matchID)
	if err != nil {
		return fmt.Errorf("UpdateParticipants: failed to execute query for solo_match %d: %w", matchID, err)
	}
	return r.checkAffectedRows(result, ErrSoloMatchNotFound)
}

func (r *postgresSoloMatchRepository) handleSoloMatchError(err error) error {
	if err == nil {
		return nil
	}
	if pqErr, ok := err.(*pq.Error); ok {
		switch pqErr.Constraint {
		case "solo_matches_tournament_id_fkey":
			return ErrSoloMatchTournamentInvalid
		case "solo_matches_p1_participant_id_fkey", "solo_matches_p2_participant_id_fkey":
			return ErrSoloMatchParticipantInvalid
		case "solo_matches_winner_participant_id_fkey":
			return ErrSoloMatchWinnerParticipantInvalid
		case "solo_matches_bracket_match_uid_key":
			return fmt.Errorf("bracket_match_uid conflict: %w", err)
		}
	}
	return err
}

func (r *postgresSoloMatchRepository) checkAffectedRows(result sql.Result, notFoundError error) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}
	if rowsAffected == 0 {
		return notFoundError
	}
	return nil
}

func (r *postgresSoloMatchRepository) CountSoloMatches(ctx context.Context, filters map[string]interface{}) (int, error) {
	query := "SELECT COUNT(*) FROM solo_matches"
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
