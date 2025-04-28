package repositories

import (
	"context"
	"database/sql"
	"github.com/Dosada05/tournament-system/models"
)

type ParticipantRepository interface {
	Create(ctx context.Context, p *models.Participant) error
	UpdateStatus(ctx context.Context, id int, status models.ParticipantStatus) error
	FindByID(ctx context.Context, id int) (*models.Participant, error)
	FindByUserAndTournament(ctx context.Context, userID, tournamentID int) (*models.Participant, error)
	FindByTeamAndTournament(ctx context.Context, teamID, tournamentID int) (*models.Participant, error)
	ListByTournament(ctx context.Context, tournamentID int) ([]*models.Participant, error)
	Delete(ctx context.Context, id int) error
}

type participantRepository struct {
	db *sql.DB
}

func NewParticipantRepository(db *sql.DB) ParticipantRepository {
	return &participantRepository{db: db}
}

func (r *participantRepository) Create(ctx context.Context, p *models.Participant) error {
	query := `
		INSERT INTO participants (user_id, team_id, tournament_id, status, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id, created_at`
	return r.db.QueryRowContext(ctx, query,
		p.UserID, p.TeamID, p.TournamentID, p.Status,
	).Scan(&p.ID, &p.CreatedAt)
}

func (r *participantRepository) UpdateStatus(ctx context.Context, id int, status models.ParticipantStatus) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE participants SET status = $1 WHERE id = $2`, status, id)
	return err
}

func (r *participantRepository) FindByID(ctx context.Context, id int) (*models.Participant, error) {
	query := `SELECT id, user_id, team_id, tournament_id, status, created_at
			  FROM participants WHERE id = $1`
	p := &models.Participant{}
	err := r.db.QueryRowContext(ctx, query, id).
		Scan(&p.ID, &p.UserID, &p.TeamID, &p.TournamentID, &p.Status, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (r *participantRepository) FindByUserAndTournament(ctx context.Context, userID, tournamentID int) (*models.Participant, error) {
	query := `SELECT id, user_id, team_id, tournament_id, status, created_at
			  FROM participants WHERE user_id = $1 AND tournament_id = $2`
	p := &models.Participant{}
	err := r.db.QueryRowContext(ctx, query, userID, tournamentID).
		Scan(&p.ID, &p.UserID, &p.TeamID, &p.TournamentID, &p.Status, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (r *participantRepository) FindByTeamAndTournament(ctx context.Context, teamID, tournamentID int) (*models.Participant, error) {
	query := `SELECT id, user_id, team_id, tournament_id, status, created_at
			  FROM participants WHERE team_id = $1 AND tournament_id = $2`
	p := &models.Participant{}
	err := r.db.QueryRowContext(ctx, query, teamID, tournamentID).
		Scan(&p.ID, &p.UserID, &p.TeamID, &p.TournamentID, &p.Status, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (r *participantRepository) ListByTournament(ctx context.Context, tournamentID int) ([]*models.Participant, error) {
	query := `SELECT id, user_id, team_id, tournament_id, status, created_at
			  FROM participants WHERE tournament_id = $1 ORDER BY id`
	rows, err := r.db.QueryContext(ctx, query, tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participants []*models.Participant
	for rows.Next() {
		var p models.Participant
		if err := rows.Scan(&p.ID, &p.UserID, &p.TeamID, &p.TournamentID, &p.Status, &p.CreatedAt); err != nil {
			return nil, err
		}
		participants = append(participants, &p)
	}
	return participants, rows.Err()
}

func (r *participantRepository) Delete(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM participants WHERE id = $1`, id)
	return err
}
