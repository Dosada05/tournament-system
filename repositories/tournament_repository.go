package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	_ "time"

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
	UpdateStatus(ctx context.Context, id int, status models.TournamentStatus) error
	Delete(ctx context.Context, id int) error
	UpdateLogoKey(ctx context.Context, tournamentID int, logoKey *string) error
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
		WHERE id = $12`

	result, err := r.db.ExecContext(ctx, query,
		t.Name, t.Description, t.SportID, t.FormatID, t.OrganizerID,
		t.RegDate, t.StartDate, t.EndDate, t.Location, t.Status, t.MaxParticipants,
		t.ID,
	)

	if err != nil {
		return r.handleTournamentError(err)
	}

	return r.checkAffected(result)
}

func (r *postgresTournamentRepository) UpdateStatus(ctx context.Context, id int, status models.TournamentStatus) error {
	query := `UPDATE tournaments SET status = $1 WHERE id = $2`
	result, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return r.handleTournamentError(err)
	}
	return r.checkAffected(result)
}

func (r *postgresTournamentRepository) UpdateLogoKey(ctx context.Context, tournamentID int, logoKey *string) error {
	query := `UPDATE tournaments SET logo_key = $1 WHERE id = $2`
	result, err := r.db.ExecContext(ctx, query, logoKey, tournamentID)
	if err != nil {
		return fmt.Errorf("failed to update tournament logo key: %w", err)
	}
	return r.checkAffected(result)
}

func (r *postgresTournamentRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM tournaments WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return r.handleTournamentError(err)
	}
	return r.checkAffected(result)
}

func (r *postgresTournamentRepository) handleTournamentError(err error) error {
	if err == nil {
		return nil
	}
	if pqErr, ok := err.(*pq.Error); ok {
		switch pqErr.Code {
		case "23505":
			if pqErr.Constraint == "tournaments_organizer_id_name_key" { // Предполагая, что такой constraint есть
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
			default:
				// Если это constraint, связанный с participants или matches, то это ErrTournamentInUse
				// Здесь сложно точно определить без знания всех constraint'ов,
				// но для удаления это будет основной причиной FK violation.
				return ErrTournamentInUse
			}
		}
	}
	return err
}

func (r *postgresTournamentRepository) checkAffected(result sql.Result) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}
	if rowsAffected == 0 {
		return ErrTournamentNotFound
	}
	return nil
}
