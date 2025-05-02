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
	ErrSoloMatchNotFound                 = errors.New("solo match not found")
	ErrSoloMatchTournamentInvalid        = errors.New("solo match tournament conflict or invalid")
	ErrSoloMatchParticipantInvalid       = errors.New("solo match participant conflict or invalid")
	ErrSoloMatchWinnerParticipantInvalid = errors.New("solo match winner participant conflict or invalid")
	// Можно рассмотреть вынос этих ошибок в общий файл errors.go в пакете repositories
)

type SoloMatchRepository interface {
	Create(ctx context.Context, match *models.SoloMatch) error
	GetByID(ctx context.Context, id int) (*models.SoloMatch, error)
	ListByTournament(ctx context.Context, tournamentID int, round *int, status *models.MatchStatus) ([]*models.SoloMatch, error)
	UpdateScoreStatusWinner(ctx context.Context, id int, score *string, status models.MatchStatus, winnerParticipantID *int) error
	Delete(ctx context.Context, id int) error
}

type postgresSoloMatchRepository struct {
	db *sql.DB
}

func NewPostgresSoloMatchRepository(db *sql.DB) SoloMatchRepository {
	return &postgresSoloMatchRepository{db: db}
}

func (r *postgresSoloMatchRepository) Create(ctx context.Context, match *models.SoloMatch) error {
	query := `
		INSERT INTO solo_matches
			(tournament_id, p1_participant_id, p2_participant_id, score, match_time, status, winner_participant_id, round)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, query,
		match.TournamentID,
		match.P1ParticipantID,
		match.P2ParticipantID,
		match.Score,
		match.MatchTime,
		match.Status,
		match.WinnerParticipantID,
		match.Round,
	).Scan(&match.ID, &match.CreatedAt)

	return r.handleSoloMatchError(err)
}

func (r *postgresSoloMatchRepository) GetByID(ctx context.Context, id int) (*models.SoloMatch, error) {
	query := `
		SELECT id, tournament_id, p1_participant_id, p2_participant_id, score, match_time, status, winner_participant_id, round, created_at
		FROM solo_matches
		WHERE id = $1`

	match := &models.SoloMatch{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
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
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSoloMatchNotFound
		}
		return nil, err
	}
	return match, nil
}

func (r *postgresSoloMatchRepository) ListByTournament(ctx context.Context, tournamentID int, roundFilter *int, statusFilter *models.MatchStatus) ([]*models.SoloMatch, error) {
	var queryBuilder strings.Builder
	queryBuilder.WriteString(`
		SELECT id, tournament_id, p1_participant_id, p2_participant_id, score, match_time, status, winner_participant_id, round, created_at
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
		placeholderIndex++
	}

	queryBuilder.WriteString(" ORDER BY round ASC, match_time ASC")

	finalQuery := queryBuilder.String()
	rows, err := r.db.QueryContext(ctx, finalQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	matches := make([]*models.SoloMatch, 0)
	for rows.Next() {
		var match models.SoloMatch
		if scanErr := rows.Scan(
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

func (r *postgresSoloMatchRepository) UpdateScoreStatusWinner(ctx context.Context, id int, score *string, status models.MatchStatus, winnerParticipantID *int) error {
	query := `
		UPDATE solo_matches
		SET score = $1, status = $2, winner_participant_id = $3
		WHERE id = $4`

	result, err := r.db.ExecContext(ctx, query, score, status, winnerParticipantID, id)
	if err != nil {
		// Проверяем FK ошибки, которые могли возникнуть при обновлении winner_participant_id
		return r.handleSoloMatchError(err)
	}

	rowsAffected, checkErr := checkRowsAffected(result) // Используем общий хелпер
	if checkErr != nil {
		return checkErr
	}
	if rowsAffected == 0 {
		return ErrSoloMatchNotFound
	}

	return nil
}

func (r *postgresSoloMatchRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM solo_matches WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, checkErr := checkRowsAffected(result)
	if checkErr != nil {
		return checkErr
	}
	if rowsAffected == 0 {
		return ErrSoloMatchNotFound
	}

	return nil
}

// handleSoloMatchError обрабатывает ошибки FK для solo_matches.
func (r *postgresSoloMatchRepository) handleSoloMatchError(err error) error {
	if err == nil {
		return nil
	}
	if pqErr, ok := err.(*pq.Error); ok {
		if pqErr.Code == "23503" { // foreign_key_violation
			// ЗАМЕНИТЕ имена constraint на реальные для solo_matches!
			switch pqErr.Constraint {
			case "solo_matches_tournament_id_fkey":
				return ErrSoloMatchTournamentInvalid
			case "solo_matches_p1_participant_id_fkey", "solo_matches_p2_participant_id_fkey":
				return ErrSoloMatchParticipantInvalid
			case "solo_matches_winner_participant_id_fkey":
				return ErrSoloMatchWinnerParticipantInvalid
			}
		}
	}
	return err
}

func checkRowsAffected(result sql.Result) (int64, error) {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rowsAffected, nil
}
