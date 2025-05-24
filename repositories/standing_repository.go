package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Dosada05/tournament-system/models"
)

var (
	ErrTournamentStandingNotFound = errors.New("tournament standing not found")
	ErrStandingParticipantInvalid = errors.New("standing participant conflict or invalid")
	ErrStandingTournamentInvalid  = errors.New("standing tournament conflict or invalid")
)

type TournamentStandingRepository interface {
	Create(ctx context.Context, exec SQLExecutor, standing *models.TournamentStanding) error
	GetByTournamentAndParticipant(ctx context.Context, exec SQLExecutor, tournamentID, participantID int) (*models.TournamentStanding, error)
	Update(ctx context.Context, exec SQLExecutor, standing *models.TournamentStanding) error
	ListByTournament(ctx context.Context, exec SQLExecutor, tournamentID int, sortByRank bool) ([]*models.TournamentStanding, error)
	GetOrCreate(ctx context.Context, exec SQLExecutor, tournamentID, participantID int) (*models.TournamentStanding, error)
	BatchCreate(ctx context.Context, exec SQLExecutor, standings []*models.TournamentStanding) error
	DeleteByTournamentID(ctx context.Context, exec SQLExecutor, tournamentID int) error
}

type postgresTournamentStandingRepository struct {
	db *sql.DB // Main DB connection, can be used if exec is nil
}

func NewPostgresTournamentStandingRepository(db *sql.DB) TournamentStandingRepository {
	return &postgresTournamentStandingRepository{db: db}
}

func (r *postgresTournamentStandingRepository) getExecutor(exec SQLExecutor) SQLExecutor {
	if exec != nil {
		return exec
	}
	return r.db
}

func (r *postgresTournamentStandingRepository) Create(ctx context.Context, exec SQLExecutor, standing *models.TournamentStanding) error {
	executor := r.getExecutor(exec)
	query := `
		INSERT INTO tournament_standings 
		    (tournament_id, participant_id, points, games_played, wins, draws, losses, score_for, score_against, score_difference, rank, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id`
	// Ensure updated_at is set before insert if not handled by DB default/trigger for creation
	if standing.UpdatedAt.IsZero() {
		standing.UpdatedAt = time.Now()
	}
	err := executor.QueryRowContext(ctx, query,
		standing.TournamentID, standing.ParticipantID, standing.Points, standing.GamesPlayed,
		standing.Wins, standing.Draws, standing.Losses, standing.ScoreFor, standing.ScoreAgainst,
		standing.ScoreDifference, standing.Rank, standing.UpdatedAt,
	).Scan(&standing.ID)

	// Handle potential pq errors for constraints if needed
	return err
}

func (r *postgresTournamentStandingRepository) BatchCreate(ctx context.Context, exec SQLExecutor, standings []*models.TournamentStanding) error {
	executor := r.getExecutor(exec)
	if len(standings) == 0 {
		return nil
	}

	// Start transaction if not already in one (though BatchCreate is usually called within one)
	tx, ok := executor.(*sql.Tx)
	if !ok && exec == r.db { // If we are using the main db conn and not a tx
		var err error
		tx, err = r.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("BatchCreate failed to begin transaction: %w", err)
		}
		defer func() {
			if p := recover(); p != nil {
				tx.Rollback()
				panic(p)
			} else if err != nil {
				tx.Rollback()
			} else {
				err = tx.Commit()
			}
		}()
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO tournament_standings
		    (tournament_id, participant_id, points, games_played, wins, draws, losses, score_for, score_against, score_difference, rank, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`)
	if err != nil {
		return fmt.Errorf("BatchCreate failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, standing := range standings {
		if standing.UpdatedAt.IsZero() {
			standing.UpdatedAt = time.Now()
		}
		_, err = stmt.ExecContext(ctx,
			standing.TournamentID, standing.ParticipantID, standing.Points, standing.GamesPlayed,
			standing.Wins, standing.Draws, standing.Losses, standing.ScoreFor, standing.ScoreAgainst,
			standing.ScoreDifference, standing.Rank, standing.UpdatedAt,
		)
		if err != nil {
			// Rollback is handled by defer if tx was started here
			return fmt.Errorf("BatchCreate failed for participant %d: %w", standing.ParticipantID, err)
		}
	}
	return err // Will be nil if successful, or commit/rollback error
}

func (r *postgresTournamentStandingRepository) scanStanding(rowScanner interface{ Scan(...interface{}) error }) (*models.TournamentStanding, error) {
	var s models.TournamentStanding
	err := rowScanner.Scan(
		&s.ID, &s.TournamentID, &s.ParticipantID, &s.Points, &s.GamesPlayed,
		&s.Wins, &s.Draws, &s.Losses, &s.ScoreFor, &s.ScoreAgainst,
		&s.ScoreDifference, &s.Rank, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTournamentStandingNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *postgresTournamentStandingRepository) GetByTournamentAndParticipant(ctx context.Context, exec SQLExecutor, tournamentID, participantID int) (*models.TournamentStanding, error) {
	executor := r.getExecutor(exec)
	query := `
		SELECT id, tournament_id, participant_id, points, games_played, wins, draws, losses, 
		       score_for, score_against, score_difference, rank, updated_at
		FROM tournament_standings 
		WHERE tournament_id = $1 AND participant_id = $2`
	row := executor.QueryRowContext(ctx, query, tournamentID, participantID)
	return r.scanStanding(row)
}

func (r *postgresTournamentStandingRepository) Update(ctx context.Context, exec SQLExecutor, standing *models.TournamentStanding) error {
	executor := r.getExecutor(exec)
	// updated_at will be set by the trigger
	query := `
		UPDATE tournament_standings SET
			points = $1, games_played = $2, wins = $3, draws = $4, losses = $5, 
			score_for = $6, score_against = $7, score_difference = $8, rank = $9
			-- updated_at = NOW() -- Or rely on trigger
		WHERE id = $10`
	result, err := executor.ExecContext(ctx, query,
		standing.Points, standing.GamesPlayed, standing.Wins, standing.Draws, standing.Losses,
		standing.ScoreFor, standing.ScoreAgainst, standing.ScoreDifference, standing.Rank,
		standing.ID,
	)
	if err != nil {
		return err
	}
	return checkAffectedRows(result, ErrTournamentStandingNotFound)
}

func (r *postgresTournamentStandingRepository) ListByTournament(ctx context.Context, exec SQLExecutor, tournamentID int, sortByRank bool) ([]*models.TournamentStanding, error) {
	executor := r.getExecutor(exec)
	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`
		SELECT id, ts.tournament_id, ts.participant_id, points, games_played, wins, draws, losses, 
		       score_for, score_against, score_difference, rank, ts.updated_at
		FROM tournament_standings ts
	`)
	// Optionally join with participants to fetch participant details
	// queryBuilder.WriteString(` JOIN participants p ON ts.participant_id = p.id `)
	// queryBuilder.WriteString(` JOIN users u ON p.user_id = u.id `) // If solo
	// queryBuilder.WriteString(` JOIN teams t ON p.team_id = t.id `) // If team

	queryBuilder.WriteString(" WHERE ts.tournament_id = $1")

	if sortByRank {
		// This order should match the idx_tournament_standings_ranking for efficiency
		queryBuilder.WriteString(" ORDER BY points DESC, score_difference DESC, score_for DESC, ts.participant_id ASC") // participant_id for stable sort
	} else {
		queryBuilder.WriteString(" ORDER BY ts.participant_id ASC")
	}

	rows, err := executor.QueryContext(ctx, queryBuilder.String(), tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	standings := make([]*models.TournamentStanding, 0)
	for rows.Next() {
		s, errScan := r.scanStanding(rows)
		if errScan != nil {
			return nil, errScan
		}
		standings = append(standings, s)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return standings, nil
}

func (r *postgresTournamentStandingRepository) GetOrCreate(ctx context.Context, exec SQLExecutor, tournamentID, participantID int) (*models.TournamentStanding, error) {
	executor := r.getExecutor(exec)
	standing, err := r.GetByTournamentAndParticipant(ctx, executor, tournamentID, participantID)
	if err != nil {
		if errors.Is(err, ErrTournamentStandingNotFound) {
			newStanding := &models.TournamentStanding{
				TournamentID:  tournamentID,
				ParticipantID: participantID,
				Points:        0,
				GamesPlayed:   0,
				Wins:          0,
				Draws:         0,
				Losses:        0,
				ScoreFor:      0,
				ScoreAgainst:  0,
				UpdatedAt:     time.Now(),
			}
			if createErr := r.Create(ctx, executor, newStanding); createErr != nil {
				return nil, fmt.Errorf("failed to create standing for t:%d p:%d: %w", tournamentID, participantID, createErr)
			}
			return newStanding, nil
		}
		return nil, fmt.Errorf("failed to get standing for t:%d p:%d: %w", tournamentID, participantID, err)
	}
	return standing, nil
}

func (r *postgresTournamentStandingRepository) DeleteByTournamentID(ctx context.Context, exec SQLExecutor, tournamentID int) error {
	executor := r.getExecutor(exec)
	query := `DELETE FROM tournament_standings WHERE tournament_id = $1`
	_, err := executor.ExecContext(ctx, query, tournamentID)
	return err
}
