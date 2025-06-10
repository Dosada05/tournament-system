// File: repositories/roster_repository.go
package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Dosada05/tournament-system/models"
)

type TournamentTeamRosterRepository interface {
	Create(ctx context.Context, exec SQLExecutor, rosterEntry *models.TournamentTeamRoster) error
	CreateBatch(ctx context.Context, exec SQLExecutor, rosterEntries []*models.TournamentTeamRoster) error
	ListByTeamParticipantID(ctx context.Context, teamParticipantID int) ([]*models.TournamentTeamRoster, error)
	ListByUser(ctx context.Context, userID int) ([]*models.TournamentTeamRoster, error)
	DeleteByParticipantID(ctx context.Context, exec SQLExecutor, participantID int) error
}

type postgresTournamentTeamRosterRepository struct {
	db *sql.DB
}

func NewPostgresTournamentTeamRosterRepository(db *sql.DB) TournamentTeamRosterRepository {
	return &postgresTournamentTeamRosterRepository{db: db}
}

func (r *postgresTournamentTeamRosterRepository) getExecutor(exec SQLExecutor) SQLExecutor {
	if exec != nil {
		return exec
	}
	return r.db
}

func (r *postgresTournamentTeamRosterRepository) Create(ctx context.Context, exec SQLExecutor, rosterEntry *models.TournamentTeamRoster) error {
	executor := r.getExecutor(exec)
	query := `INSERT INTO tournament_team_rosters (participant_id, user_id) VALUES ($1, $2) RETURNING id, created_at`
	err := executor.QueryRowContext(ctx, query, rosterEntry.ParticipantID, rosterEntry.UserID).Scan(&rosterEntry.ID, &rosterEntry.CreatedAt)
	if err != nil {
		// Обработка ошибок pq для unique_violation и foreign_key_violation
		return fmt.Errorf("failed to create tournament team roster entry: %w", err)
	}
	return nil
}

func (r *postgresTournamentTeamRosterRepository) CreateBatch(ctx context.Context, exec SQLExecutor, rosterEntries []*models.TournamentTeamRoster) error {
	executor := r.getExecutor(exec)
	if len(rosterEntries) == 0 {
		return nil
	}

	tx, isExternalTx := executor.(*sql.Tx)
	if !isExternalTx {
		var err error
		tx, err = r.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("CreateBatch failed to begin transaction: %w", err)
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

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO tournament_team_rosters (participant_id, user_id) VALUES ($1, $2)`)
	if err != nil {
		return fmt.Errorf("CreateBatch failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, entry := range rosterEntries {
		_, err = stmt.ExecContext(ctx, entry.ParticipantID, entry.UserID)
		if err != nil {
			return fmt.Errorf("CreateBatch failed for participant_id %d, user_id %d: %w", entry.ParticipantID, entry.UserID, err)
		}
	}
	return err // nil if successful and no commit/rollback error from managed transaction
}

func (r *postgresTournamentTeamRosterRepository) ListByTeamParticipantID(ctx context.Context, teamParticipantID int) ([]*models.TournamentTeamRoster, error) {
	query := `SELECT id, participant_id, user_id, created_at FROM tournament_team_rosters WHERE participant_id = $1`
	rows, err := r.db.QueryContext(ctx, query, teamParticipantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tournament team roster by team_participant_id %d: %w", teamParticipantID, err)
	}
	defer rows.Close()

	entries := make([]*models.TournamentTeamRoster, 0)
	for rows.Next() {
		var entry models.TournamentTeamRoster
		if err := rows.Scan(&entry.ID, &entry.ParticipantID, &entry.UserID, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan tournament team roster entry: %w", err)
		}
		entries = append(entries, &entry)
	}
	return entries, rows.Err()
}

func (r *postgresTournamentTeamRosterRepository) ListByUser(ctx context.Context, userID int) ([]*models.TournamentTeamRoster, error) {
	// Этот метод вернет все participant_id (командные), в которых пользователь когда-либо числился.
	query := `
        SELECT ttr.id, ttr.participant_id, ttr.user_id, ttr.created_at,
               p.tournament_id, p.team_id, -- поля из participants
               t.id as tournament_db_id, t.name as tournament_name, t.sport_id as tournament_sport_id,
		       t.format_id as tournament_format_id, t.organizer_id as tournament_organizer_id,
		       t.start_date as tournament_start_date, t.end_date as tournament_end_date,
		       t.status as tournament_db_status, t.logo_key as tournament_logo_key,
               t.overall_winner_participant_id as tournament_overall_winner_pid
        FROM tournament_team_rosters ttr
        JOIN participants p ON ttr.participant_id = p.id
        JOIN tournaments t ON p.tournament_id = t.id
        WHERE ttr.user_id = $1
        ORDER BY t.start_date DESC, ttr.created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tournament team roster by user_id %d: %w", userID, err)
	}
	defer rows.Close()

	entries := make([]*models.TournamentTeamRoster, 0)
	for rows.Next() {
		var entry models.TournamentTeamRoster
		var p models.Participant    // Для хранения данных из participants
		var tourn models.Tournament // Для хранения данных из tournaments
		if err := rows.Scan(
			&entry.ID, &entry.ParticipantID, &entry.UserID, &entry.CreatedAt,
			&p.TournamentID, &p.TeamID, // из participants
			&tourn.ID, &tourn.Name, &tourn.SportID, &tourn.FormatID, &tourn.OrganizerID,
			&tourn.StartDate, &tourn.EndDate, &tourn.Status, &tourn.LogoKey, &tourn.OverallWinnerParticipantID, // из tournaments
		); err != nil {
			return nil, fmt.Errorf("failed to scan tournament team roster entry with joins: %w", err)
		}
		// Заполняем Participant в roster entry
		p.ID = entry.ParticipantID // participant_id из ttr это и есть ID записи в participants
		p.Tournament = &tourn      // Присоединяем данные турнира
		entry.Participant = &p

		entries = append(entries, &entry)
	}
	return entries, rows.Err()
}

func (r *postgresTournamentTeamRosterRepository) DeleteByParticipantID(ctx context.Context, exec SQLExecutor, participantID int) error {
	executor := r.getExecutor(exec)
	query := `DELETE FROM tournament_team_rosters WHERE participant_id = $1`
	_, err := executor.ExecContext(ctx, query, participantID)
	if err != nil {
		return fmt.Errorf("failed to delete tournament team roster entries by participant_id %d: %w", participantID, err)
	}
	return nil
}
