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
	GetTournamentsForAutoStatusUpdate(ctx context.Context, exec SQLExecutor, currentTime time.Time) ([]*models.Tournament, error)
}

type postgresTournamentRepository struct {
	db *sql.DB
}

func NewPostgresTournamentRepository(db *sql.DB) TournamentRepository {
	return &postgresTournamentRepository{db: db}
}

func (r *postgresTournamentRepository) Create(ctx context.Context, t *models.Tournament) error {
	query := `
		INSERT INTO tournaments (
			name, description, sport_id, format_id, organizer_id,
			reg_date, start_date, end_date, location, status, max_participants, logo_key
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, query,
		t.Name, t.Description, t.SportID, t.FormatID, t.OrganizerID,
		t.RegDate, t.StartDate, t.EndDate, t.Location, t.Status, t.MaxParticipants, t.LogoKey,
	).Scan(&t.ID, &t.CreatedAt)

	return r.handleTournamentError(err)
}

func (r *postgresTournamentRepository) GetByID(ctx context.Context, id int) (*models.Tournament, error) {
	query := `
		SELECT
			id, name, description, sport_id, format_id, organizer_id,
			reg_date, start_date, end_date, location, status, max_participants, created_at, logo_key
		FROM tournaments
		WHERE id = $1`

	t := &models.Tournament{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&t.ID, &t.Name, &t.Description, &t.SportID, &t.FormatID, &t.OrganizerID,
		&t.RegDate, &t.StartDate, &t.EndDate, &t.Location, &t.Status, &t.MaxParticipants, &t.CreatedAt, &t.LogoKey,
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
	query := `
		SELECT
			id, name, description, sport_id, format_id, organizer_id,
			reg_date, start_date, end_date, location, status, max_participants, created_at, logo_key
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
		argID++
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
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
		WHERE id = $12` // Assuming logo_key is updated separately or not in this general update

	result, err := r.db.ExecContext(ctx, query,
		t.Name, t.Description, t.SportID, t.FormatID, t.OrganizerID,
		t.RegDate, t.StartDate, t.EndDate, t.Location, t.Status, t.MaxParticipants,
		t.ID,
	)

	if err != nil {
		return r.handleTournamentError(err)
	}

	return checkAffectedRows(result, ErrTournamentNotFound) // Using shared helper
}

func (r *postgresTournamentRepository) UpdateStatus(ctx context.Context, exec SQLExecutor, id int, status models.TournamentStatus) error {
	query := `UPDATE tournaments SET status = $1 WHERE id = $2`
	result, err := exec.ExecContext(ctx, query, status, id)
	if err != nil {
		return r.handleTournamentError(err)
	}
	return checkAffectedRows(result, ErrTournamentNotFound) // Using shared helper
}

func (r *postgresTournamentRepository) UpdateLogoKey(ctx context.Context, tournamentID int, logoKey *string) error {
	query := `UPDATE tournaments SET logo_key = $1 WHERE id = $2`
	result, err := r.db.ExecContext(ctx, query, logoKey, tournamentID)
	if err != nil {
		return fmt.Errorf("failed to update tournament logo key: %w", err)
	}
	return checkAffectedRows(result, ErrTournamentNotFound) // Using shared helper
}

func (r *postgresTournamentRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM tournaments WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return r.handleTournamentError(err)
	}
	return checkAffectedRows(result, ErrTournamentNotFound) // Using shared helper
}

// GetTournamentsForAutoStatusUpdate fetches tournaments that might need a status update.
// It considers tournaments not yet 'completed' or 'canceled'.
// It checks:
// - 'soon' tournaments if reg_date has passed.
// - 'registration' tournaments if start_date has passed.
// - 'active' tournaments if end_date has passed.
func (r *postgresTournamentRepository) GetTournamentsForAutoStatusUpdate(ctx context.Context, exec SQLExecutor, currentTime time.Time) ([]*models.Tournament, error) {
	query := `
		SELECT
			id, name, description, sport_id, format_id, organizer_id,
			reg_date, start_date, end_date, location, status, max_participants, created_at, logo_key
		FROM tournaments
		WHERE status NOT IN ($1, $2) -- 'completed', 'canceled'
		AND (
			(status = $3 AND reg_date <= $4) OR    -- 'soon' AND reg_date <= now
			(status = $5 AND start_date <= $4) OR -- 'registration' AND start_date <= now
			(status = $6 AND end_date <= $4)      -- 'active' AND end_date <= now
		)`
	args := []interface{}{
		models.StatusCompleted,    // $1
		models.StatusCanceled,     // $2
		models.StatusSoon,         // $3
		currentTime,               // $4
		models.StatusRegistration, // $5
		models.StatusActive,       // $6
	}

	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tournaments for auto status update: %w", err)
	}
	defer rows.Close()

	tournaments := make([]*models.Tournament, 0)
	for rows.Next() {
		var t models.Tournament
		if scanErr := rows.Scan(
			&t.ID, &t.Name, &t.Description, &t.SportID, &t.FormatID, &t.OrganizerID,
			&t.RegDate, &t.StartDate, &t.EndDate, &t.Location, &t.Status, &t.MaxParticipants, &t.CreatedAt, &t.LogoKey,
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
		case "23505": // unique_violation
			if pqErr.Constraint == "tournaments_organizer_id_name_key" { // Ensure this constraint exists for name uniqueness per organizer
				return ErrTournamentNameConflict
			}
		case "23503": // foreign_key_violation
			switch pqErr.Constraint {
			case "tournaments_sport_id_fkey":
				return ErrTournamentInvalidSport
			case "tournaments_format_id_fkey":
				return ErrTournamentInvalidFormat
			case "tournaments_organizer_id_fkey":
				return ErrTournamentInvalidOrg
			default:
				// This case can cover FK violations from participants or matches tables pointing to tournaments,
				// indicating the tournament is in use.
				return ErrTournamentInUse
			}
		}
	}
	return err
}
