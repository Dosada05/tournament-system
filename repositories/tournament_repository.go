package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Dosada05/tournament-system/models"
	"github.com/lib/pq"
)

var (
	ErrTournamentNotFound      = errors.New("tournament not found")
	ErrTournamentNameConflict  = errors.New("tournament name conflict for this organizer")
	ErrTournamentInUse         = errors.New("tournament is in use (participants/matches exist)")
	ErrTournamentInvalidSport  = errors.New("invalid sport reference")
	ErrTournamentInvalidFormat = errors.New("invalid format reference")
	ErrTournamentInvalidOrg    = errors.New("invalid organizer reference")
)

type ListTournamentsFilter struct {
	SportID     *int
	FormatID    *int
	OrganizerID *int
	Status      *models.TournamentStatus
	Limit       int
	Offset      int
}

type TournamentRepository interface {
	Create(ctx context.Context, tournament *models.Tournament) error
	GetByID(ctx context.Context, id int) (*models.Tournament, error)
	List(ctx context.Context, filter ListTournamentsFilter) ([]models.Tournament, error)
	Update(ctx context.Context, tournament *models.Tournament) error
	UpdateStatus(ctx context.Context, exec SQLExecutor, id int, status models.TournamentStatus) error
	Delete(ctx context.Context, id int) error
	UpdateLogoKey(ctx context.Context, tournamentID int, logoKey *string) error
	UpdateOverallWinner(ctx context.Context, exec SQLExecutor, tournamentID int, winnerParticipantID *int) error // Added
	GetTournamentsForAutoStatusUpdate(ctx context.Context, exec SQLExecutor, currentTime time.Time) ([]*models.Tournament, error)
}

type postgresTournamentRepository struct {
	db *sql.DB
}

func NewPostgresTournamentRepository(db *sql.DB) TournamentRepository {
	return &postgresTournamentRepository{db: db}
}

func (r *postgresTournamentRepository) getExecutor(exec SQLExecutor) SQLExecutor {
	if exec != nil {
		return exec
	}
	return r.db
}

func (r *postgresTournamentRepository) Create(ctx context.Context, t *models.Tournament) error {
	executor := r.getExecutor(nil)
	// overall_winner_participant_id is not set on creation
	query := `
		INSERT INTO tournaments (
			name, description, sport_id, format_id, organizer_id,
			reg_date, start_date, end_date, location, status, max_participants, logo_key 
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at`

	err := executor.QueryRowContext(ctx, query,
		t.Name, t.Description, t.SportID, t.FormatID, t.OrganizerID,
		t.RegDate, t.StartDate, t.EndDate, t.Location, t.Status, t.MaxParticipants, t.LogoKey,
	).Scan(&t.ID, &t.CreatedAt)

	return r.handleTournamentError(err)
}

func (r *postgresTournamentRepository) GetByID(ctx context.Context, id int) (*models.Tournament, error) {
	executor := r.getExecutor(nil)
	query := `
		SELECT
			id, name, description, sport_id, format_id, organizer_id,
			reg_date, start_date, end_date, location, status, max_participants, created_at, logo_key,
			overall_winner_participant_id
		FROM tournaments
		WHERE id = $1`

	t := &models.Tournament{}
	err := executor.QueryRowContext(ctx, query, id).Scan(
		&t.ID, &t.Name, &t.Description, &t.SportID, &t.FormatID, &t.OrganizerID,
		&t.RegDate, &t.StartDate, &t.EndDate, &t.Location, &t.Status, &t.MaxParticipants, &t.CreatedAt, &t.LogoKey,
		&t.OverallWinnerParticipantID, // Added scan for the new field
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTournamentNotFound
		}
		return nil, err
	}
	return t, nil
}

func (r *postgresTournamentRepository) List(ctx context.Context, filter ListTournamentsFilter) ([]models.Tournament, error) {
	executor := r.getExecutor(nil)
	query := `
		SELECT
			id, name, description, sport_id, format_id, organizer_id,
			reg_date, start_date, end_date, location, status, max_participants, created_at, logo_key,
			overall_winner_participant_id
		FROM tournaments
		WHERE 1=1`

	args := []interface{}{}
	argID := 1

	if filter.SportID != nil {
		query += fmt.Sprintf(" AND sport_id = $%d", argID)
		args = append(args, *filter.SportID)
		argID++
	}
	if filter.FormatID != nil {
		query += fmt.Sprintf(" AND format_id = $%d", argID)
		args = append(args, *filter.FormatID)
		argID++
	}
	if filter.OrganizerID != nil {
		query += fmt.Sprintf(" AND organizer_id = $%d", argID)
		args = append(args, *filter.OrganizerID)
		argID++
	}
	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argID)
		args = append(args, *filter.Status)
		argID++
	}

	query += " ORDER BY start_date DESC, created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argID)
		args = append(args, filter.Limit)
		argID++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argID)
		args = append(args, filter.Offset)
		// argID++ // No need to increment here
	}

	rows, err := executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tournaments := make([]models.Tournament, 0)
	for rows.Next() {
		var t models.Tournament
		if scanErr := rows.Scan(
			&t.ID, &t.Name, &t.Description, &t.SportID, &t.FormatID, &t.OrganizerID,
			&t.RegDate, &t.StartDate, &t.EndDate, &t.Location, &t.Status, &t.MaxParticipants, &t.CreatedAt, &t.LogoKey,
			&t.OverallWinnerParticipantID, // Added scan
		); scanErr != nil {
			return nil, scanErr
		}
		tournaments = append(tournaments, t)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return tournaments, nil
}

func (r *postgresTournamentRepository) Update(ctx context.Context, t *models.Tournament) error {
	executor := r.getExecutor(nil)
	// Assuming logo_key and overall_winner_participant_id are updated by their specific methods
	query := `
		UPDATE tournaments SET
			name = $1,
			description = $2,
			sport_id = $3,
			format_id = $4,
			organizer_id = $5, 
			reg_date = $6,
			start_date = $7,
			end_date = $8,
			location = $9,
			status = $10,
			max_participants = $11
			-- overall_winner_participant_id is NOT updated here by default
		WHERE id = $12`

	result, err := executor.ExecContext(ctx, query,
		t.Name, t.Description, t.SportID, t.FormatID, t.OrganizerID,
		t.RegDate, t.StartDate, t.EndDate, t.Location, t.Status, t.MaxParticipants,
		t.ID,
	)

	if err != nil {
		return r.handleTournamentError(err)
	}

	return checkAffectedRows(result, ErrTournamentNotFound)
}

func (r *postgresTournamentRepository) UpdateStatus(ctx context.Context, exec SQLExecutor, id int, status models.TournamentStatus) error {
	executor := r.getExecutor(exec)
	query := `UPDATE tournaments SET status = $1 WHERE id = $2`
	result, err := executor.ExecContext(ctx, query, status, id)
	if err != nil {
		return r.handleTournamentError(err)
	}
	return checkAffectedRows(result, ErrTournamentNotFound)
}

func (r *postgresTournamentRepository) UpdateLogoKey(ctx context.Context, tournamentID int, logoKey *string) error {
	executor := r.getExecutor(nil)
	query := `UPDATE tournaments SET logo_key = $1 WHERE id = $2`
	result, err := executor.ExecContext(ctx, query, logoKey, tournamentID)
	if err != nil {
		return fmt.Errorf("failed to update tournament logo key: %w", err)
	}
	return checkAffectedRows(result, ErrTournamentNotFound)
}

// UpdateOverallWinner sets or clears the overall winner of the tournament.
func (r *postgresTournamentRepository) UpdateOverallWinner(ctx context.Context, exec SQLExecutor, tournamentID int, winnerParticipantID *int) error {
	executor := r.getExecutor(exec)
	query := `UPDATE tournaments SET overall_winner_participant_id = $1 WHERE id = $2`
	result, err := executor.ExecContext(ctx, query, winnerParticipantID, tournamentID)
	if err != nil {
		// Check for foreign key violation if winnerParticipantID is invalid, though SET NULL FK should handle non-existent participants gracefully if that's desired.
		// For now, a general error.
		return fmt.Errorf("failed to update tournament overall winner for tournament %d: %w", tournamentID, err)
	}
	return checkAffectedRows(result, ErrTournamentNotFound)
}

func (r *postgresTournamentRepository) Delete(ctx context.Context, id int) error {
	executor := r.getExecutor(nil)
	query := `DELETE FROM tournaments WHERE id = $1`
	result, err := executor.ExecContext(ctx, query, id)
	if err != nil {
		return r.handleTournamentError(err)
	}
	return checkAffectedRows(result, ErrTournamentNotFound)
}

func (r *postgresTournamentRepository) GetTournamentsForAutoStatusUpdate(ctx context.Context, exec SQLExecutor, currentTime time.Time) ([]*models.Tournament, error) {
	executor := r.getExecutor(exec) // Use the passed executor
	query := `
		SELECT
			id, name, description, sport_id, format_id, organizer_id,
			reg_date, start_date, end_date, location, status, max_participants, created_at, logo_key,
			overall_winner_participant_id
		FROM tournaments
		WHERE status NOT IN ($1, $2) 
		AND (
			(status = $3 AND reg_date <= $4) OR    
			(status = $5 AND start_date <= $4) OR 
			(status = $6 AND end_date <= $4 AND overall_winner_participant_id IS NULL) -- Only move active to completed if no winner yet via this auto-process
                                                                                   -- or remove overall_winner_participant_id IS NULL if EndDate strictly means completed regardless of winner.
		)`
	args := []interface{}{
		models.StatusCompleted,    // $1
		models.StatusCanceled,     // $2
		models.StatusSoon,         // $3
		currentTime,               // $4
		models.StatusRegistration, // $5
		models.StatusActive,       // $6
	}

	rows, err := executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tournaments for auto status update: %w", err)
	}
	defer rows.Close()

	var tournaments []*models.Tournament // Changed to slice of pointers
	for rows.Next() {
		var t models.Tournament
		if scanErr := rows.Scan(
			&t.ID, &t.Name, &t.Description, &t.SportID, &t.FormatID, &t.OrganizerID,
			&t.RegDate, &t.StartDate, &t.EndDate, &t.Location, &t.Status, &t.MaxParticipants, &t.CreatedAt, &t.LogoKey,
			&t.OverallWinnerParticipantID, // Added scan
		); scanErr != nil {
			return nil, fmt.Errorf("failed to scan tournament for auto status update: %w", scanErr)
		}
		tournaments = append(tournaments, &t)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during tournament rows iteration for auto status update: %w", err)
	}
	return tournaments, nil
}

func (r *postgresTournamentRepository) handleTournamentError(err error) error {
	if err == nil {
		return nil
	}
	if pqErr, ok := err.(*pq.Error); ok {
		switch pqErr.Code {
		case "23505":
			if pqErr.Constraint == "tournaments_organizer_id_name_key" {
				return ErrTournamentNameConflict
			}
		case "23503":
			switch pqErr.Constraint {
			case "tournaments_sport_id_fkey":
				return ErrTournamentInvalidSport
			case "tournaments_format_id_fkey":
				return ErrTournamentInvalidFormat
			case "tournaments_organizer_id_fkey":
				return ErrTournamentInvalidOrg
			case "fk_tournaments_overall_winner": // If a non-existent participant ID is used
				return ErrParticipantNotFound // Or a more specific error like ErrWinnerParticipantInvalid
			default:
				// This case can cover FK violations from participants or matches tables pointing to tournaments,
				// indicating the tournament is in use when attempting to delete the tournament itself.
				// It might also catch other FK issues if constraints are named differently.
				// For creation/update, this FK part is less likely to be the primary error source from *this* table's operation itself.
				return ErrTournamentInUse // Or a more generic FK violation error
			}
		}
	}
	return err
}
