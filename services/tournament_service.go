// File: tournament-system/services/tournament_service.go
package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Dosada05/tournament-system/brackets"
	"github.com/Dosada05/tournament-system/db"
	"github.com/Dosada05/tournament-system/models"
	"github.com/Dosada05/tournament-system/repositories"
	"github.com/Dosada05/tournament-system/storage"
	"golang.org/x/sync/errgroup"
)

const (
	tournamentLogoPrefix = "logos/tournaments"
)

var (
	ErrTournamentNameRequired       = errors.New("tournament name is required")
	ErrTournamentDatesRequired      = errors.New("registration, start, and end dates are required")
	ErrTournamentSportNotFound      = errors.New("specified sport not found")
	ErrTournamentFormatNotFound     = errors.New("specified format not found")
	ErrTournamentOrganizerNotFound  = errors.New("specified organizer user not found")
	ErrTournamentCreationFailed     = errors.New("failed to create tournament")
	ErrTournamentUpdateFailed       = errors.New("failed to update tournament")
	ErrTournamentDeleteFailed       = errors.New("failed to delete tournament")
	ErrTournamentListFailed         = errors.New("failed to list tournaments")
	ErrTournamentUpdateNotAllowed   = errors.New("tournament update not allowed in current status")
	ErrTournamentDeletionNotAllowed = errors.New("tournament deletion not allowed")
	ErrTournamentInUse              = repositories.ErrTournamentInUse
	// ErrStandingsNotFound is a new error specific to this service
	ErrStandingsNotFound = errors.New("tournament standings not found or not applicable")
)

type FullTournamentBracketView struct {
	TournamentID               int                        `json:"tournament_id"`
	Name                       string                     `json:"name"`
	Status                     models.TournamentStatus    `json:"status"`
	Sport                      *models.Sport              `json:"sport,omitempty"`
	Format                     *models.Format             `json:"format,omitempty"`
	Rounds                     []RoundView                `json:"rounds,omitempty"`    // For elimination brackets
	Matches                    []MatchView                `json:"matches,omitempty"`   // All matches for RoundRobin, or detailed matches for SE
	Standings                  []TournamentStandingView   `json:"standings,omitempty"` // For RoundRobin
	ParticipantsMap            map[int]ParticipantView    `json:"participants_map,omitempty"`
	OverallWinnerParticipantID *int                       `json:"overall_winner_participant_id,omitempty"`
	TournamentSettings         *models.RoundRobinSettings `json:"tournament_settings,omitempty"` // Parsed settings for RR
}

type RoundView struct {
	RoundNumber int         `json:"round_number"`
	Matches     []MatchView `json:"matches"`
}

type MatchView struct {
	MatchID               int                `json:"match_id"`
	BracketMatchUID       *string            `json:"bracket_match_uid,omitempty"`
	Status                models.MatchStatus `json:"status"`
	Round                 int                `json:"round"`
	OrderInRound          int                `json:"order_in_round"`
	Participant1          *ParticipantView   `json:"participant1,omitempty"`
	Participant2          *ParticipantView   `json:"participant2,omitempty"`
	ScoreP1               *int               `json:"score_p1,omitempty"` // Populated from ScoreString if applicable
	ScoreP2               *int               `json:"score_p2,omitempty"` // Populated from ScoreString if applicable
	IsDraw                bool               `json:"is_draw"`            // Indicates if the match was a draw
	ScoreString           *string            `json:"score_string,omitempty"`
	WinnerParticipantDBID *int               `json:"winner_participant_db_id,omitempty"`
	NextMatchDBID         *int               `json:"next_match_db_id,omitempty"`
	WinnerToSlot          *int               `json:"winner_to_slot,omitempty"`
	MatchTime             time.Time          `json:"match_time"`
}

type ParticipantView struct {
	ParticipantDBID int     `json:"participant_db_id"`
	Type            string  `json:"type"` // "team" or "user"
	Name            string  `json:"name"`
	LogoURL         *string `json:"logo_url,omitempty"`
	OriginalUserID  *int    `json:"original_user_id,omitempty"`
	OriginalTeamID  *int    `json:"original_team_id,omitempty"`
}

// TournamentStandingView for API response
type TournamentStandingView struct {
	Participant     ParticipantView `json:"participant"`
	Rank            int             `json:"rank"` // Calculated rank
	Points          int             `json:"points"`
	GamesPlayed     int             `json:"games_played"`
	Wins            int             `json:"wins"`
	Draws           int             `json:"draws"`
	Losses          int             `json:"losses"`
	ScoreFor        int             `json:"score_for"`
	ScoreAgainst    int             `json:"score_against"`
	ScoreDifference int             `json:"score_difference"`
}

type CreateTournamentInput struct {
	Name            string    `json:"name" validate:"required"`
	Description     *string   `json:"description"`
	SportID         int       `json:"sport_id" validate:"required,gt=0"`
	FormatID        int       `json:"format_id" validate:"required,gt=0"`
	RegDate         time.Time `json:"reg_date" validate:"required"`
	StartDate       time.Time `json:"start_date" validate:"required"`
	EndDate         time.Time `json:"end_date" validate:"required"`
	Location        *string   `json:"location"`
	MaxParticipants int       `json:"max_participants" validate:"required,gt=0"`
}

type UpdateTournamentDetailsInput struct {
	Name            *string    `json:"name"`
	Description     *string    `json:"description"`
	RegDate         *time.Time `json:"reg_date"`
	StartDate       *time.Time `json:"start_date"`
	EndDate         *time.Time `json:"end_date"`
	Location        *string    `json:"location"`
	MaxParticipants *int       `json:"max_participants" validate:"omitempty,gt=0"`
}

type ListTournamentsFilter struct {
	SportID     *int
	FormatID    *int
	OrganizerID *int
	Status      *models.TournamentStatus
	Limit       int
	Offset      int
}

type TournamentService interface {
	CreateTournament(ctx context.Context, organizerID int, input CreateTournamentInput) (*models.Tournament, error)
	GetTournamentByID(ctx context.Context, id int, currentUserID int) (*models.Tournament, error)
	ListTournaments(ctx context.Context, filter ListTournamentsFilter) ([]models.Tournament, error)
	UpdateTournamentDetails(ctx context.Context, id int, currentUserID int, input UpdateTournamentDetailsInput) (*models.Tournament, error)
	UpdateTournamentStatus(ctx context.Context, id int, currentUserID int, newStatus models.TournamentStatus, exec repositories.SQLExecutor) (*models.Tournament, error)
	DeleteTournament(ctx context.Context, id int, currentUserID int) error
	UploadTournamentLogo(ctx context.Context, tournamentID int, currentUserID int, file io.Reader, contentType string) (*models.Tournament, error)
	FinalizeTournament(ctx context.Context, tournamentID int, winnerParticipantDBID *int, currentUserID int) (*models.Tournament, error)
	GetTournamentBracketData(ctx context.Context, tournamentID int) (*FullTournamentBracketView, error)
	AutoUpdateTournamentStatusesByDates(ctx context.Context) error
}

type tournamentService struct {
	db              *sql.DB
	tournamentRepo  repositories.TournamentRepository
	sportRepo       repositories.SportRepository
	formatRepo      repositories.FormatRepository
	userRepo        repositories.UserRepository
	participantRepo repositories.ParticipantRepository
	soloMatchRepo   repositories.SoloMatchRepository
	teamMatchRepo   repositories.TeamMatchRepository
	standingRepo    repositories.TournamentStandingRepository
	bracketService  BracketService
	matchService    MatchService
	uploader        storage.FileUploader
	hub             *brackets.Hub
	logger          *slog.Logger
}

func NewTournamentService(
	db *sql.DB,
	tournamentRepo repositories.TournamentRepository,
	sportRepo repositories.SportRepository,
	formatRepo repositories.FormatRepository,
	userRepo repositories.UserRepository,
	participantRepo repositories.ParticipantRepository,
	soloMatchRepo repositories.SoloMatchRepository,
	teamMatchRepo repositories.TeamMatchRepository,
	standingRepo repositories.TournamentStandingRepository, // Added
	bracketService BracketService,
	matchService MatchService,
	uploader storage.FileUploader,
	hub *brackets.Hub,
	logger *slog.Logger,
) TournamentService {
	return &tournamentService{
		db:              db,
		tournamentRepo:  tournamentRepo,
		sportRepo:       sportRepo,
		formatRepo:      formatRepo,
		userRepo:        userRepo,
		participantRepo: participantRepo,
		soloMatchRepo:   soloMatchRepo,
		teamMatchRepo:   teamMatchRepo,
		standingRepo:    standingRepo,
		bracketService:  bracketService,
		matchService:    matchService,
		uploader:        uploader,
		hub:             hub,
		logger:          logger,
	}
}

func (s *tournamentService) CreateTournament(ctx context.Context, organizerID int, input CreateTournamentInput) (*models.Tournament, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrTournamentNameRequired
	}
	if err := validateTournamentDates(input.RegDate, input.StartDate, input.EndDate); err != nil {
		return nil, err
	}
	if input.MaxParticipants <= 0 {
		return nil, ErrTournamentInvalidCapacity
	}

	_, err := s.sportRepo.GetByID(ctx, input.SportID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentSportNotFound, "failed to verify sport %d", input.SportID)
	}
	_, err = s.formatRepo.GetByID(ctx, input.FormatID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentFormatNotFound, "failed to verify format %d", input.FormatID)
	}
	_, err = s.userRepo.GetByID(ctx, organizerID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentOrganizerNotFound, "failed to verify organizer %d", organizerID)
	}

	tournament := &models.Tournament{
		Name:            name,
		Description:     input.Description,
		SportID:         input.SportID,
		FormatID:        input.FormatID,
		OrganizerID:     organizerID,
		RegDate:         input.RegDate,
		StartDate:       input.StartDate,
		EndDate:         input.EndDate,
		Location:        input.Location,
		MaxParticipants: input.MaxParticipants,
		Status:          models.StatusSoon,
	}

	err = s.tournamentRepo.Create(ctx, tournament)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNameConflict) {
			return nil, ErrTournamentNameConflict
		}
		return nil, fmt.Errorf("%w: %w", ErrTournamentCreationFailed, err)
	}
	s.populateTournamentDetails(ctx, tournament)
	return tournament, nil
}

func (s *tournamentService) GetTournamentByID(ctx context.Context, id int, currentUserID int) (*models.Tournament, error) {
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament by id %d", id)
	}
	s.populateTournamentDetails(ctx, tournament) // Populates Sport, Format, Organizer, and LogoURL

	// Enhanced details loading
	g, gCtx := errgroup.WithContext(ctx)

	// Load confirmed participants
	g.Go(func() error {
		confirmedStatus := models.StatusParticipant
		dbParticipants, participantsErr := s.participantRepo.ListByTournament(gCtx, id, &confirmedStatus, true) // true for nested details
		if participantsErr != nil {
			s.logger.WarnContext(gCtx, "Failed to fetch confirmed participants", slog.Int("tournament_id", id), slog.Any("error", participantsErr))
			// Decide if this is a critical error or if we can proceed
		} else {
			populateParticipantListDetailsFunc(dbParticipants, s.uploader) // Using helper
			tournament.Participants = ParticipantsToInterface(dbParticipants)
		}
		return nil // Return nil to not fail the group for this non-critical load
	})

	// Load matches if tournament is active or completed
	if tournament.Status == models.StatusActive || tournament.Status == models.StatusCompleted {
		if tournament.Format != nil { // Ensure format is loaded before trying to list matches
			g.Go(func() error {
				if tournament.Format.ParticipantType == models.FormatParticipantSolo {
					soloMatches, soloErr := s.matchService.ListSoloMatchesByTournament(gCtx, id)
					if soloErr != nil {
						s.logger.WarnContext(gCtx, "Failed to fetch solo matches", slog.Int("tournament_id", id), slog.Any("error", soloErr))
					} else {
						tournament.SoloMatches = SoloMatchesToInterface(soloMatches)
					}
				}
				return nil
			})

			g.Go(func() error {
				if tournament.Format.ParticipantType == models.FormatParticipantTeam {
					teamMatches, teamErr := s.matchService.ListTeamMatchesByTournament(gCtx, id)
					if teamErr != nil {
						s.logger.WarnContext(gCtx, "Failed to fetch team matches", slog.Int("tournament_id", id), slog.Any("error", teamErr))
					} else {
						tournament.TeamMatches = TeamMatchesToInterface(teamMatches)
					}
				}
				return nil
			})
		} else {
			s.logger.WarnContext(ctx, "Format not loaded for tournament, cannot fetch matches", slog.Int("tournament_id", id))
		}
	}

	if err := g.Wait(); err != nil {
		s.logger.ErrorContext(ctx, "Error during parallel fetching of additional tournament details", slog.Int("tournament_id", id), slog.Any("error", err))
	}

	return tournament, nil
}

func (s *tournamentService) ListTournaments(ctx context.Context, filter ListTournamentsFilter) ([]models.Tournament, error) {
	repoFilter := repositories.ListTournamentsFilter{
		SportID:     filter.SportID,
		FormatID:    filter.FormatID,
		OrganizerID: filter.OrganizerID,
		Status:      filter.Status,
		Limit:       filter.Limit,
		Offset:      filter.Offset,
	}
	tournaments, err := s.tournamentRepo.List(ctx, repoFilter)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTournamentListFailed, err)
	}
	if tournaments == nil {
		return []models.Tournament{}, nil
	}
	for i := range tournaments {
		s.populateTournamentDetails(ctx, &tournaments[i])
	}
	return tournaments, nil
}

func (s *tournamentService) UpdateTournamentDetails(ctx context.Context, id int, currentUserID int, input UpdateTournamentDetailsInput) (*models.Tournament, error) {
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament %d for update", id)
	}

	if tournament.OrganizerID != currentUserID {
		return nil, ErrForbiddenOperation
	}

	if tournament.Status != models.StatusSoon && tournament.Status != models.StatusRegistration {
		return nil, fmt.Errorf("%w: cannot update tournament in status '%s'", ErrTournamentUpdateNotAllowed, tournament.Status)
	}

	updated := false
	if input.Name != nil {
		trimmedName := strings.TrimSpace(*input.Name)
		if trimmedName == "" {
			return nil, ErrTournamentNameRequired
		}
		if trimmedName != tournament.Name {
			tournament.Name = trimmedName
			updated = true
		}
	}
	if input.Description != nil && derefString(input.Description) != derefString(tournament.Description) {
		tournament.Description = input.Description
		updated = true
	}
	datesChanged := false
	if input.RegDate != nil && !input.RegDate.IsZero() && !input.RegDate.Equal(tournament.RegDate) {
		tournament.RegDate = *input.RegDate
		updated = true
		datesChanged = true
	}
	if input.StartDate != nil && !input.StartDate.IsZero() && !input.StartDate.Equal(tournament.StartDate) {
		tournament.StartDate = *input.StartDate
		updated = true
		datesChanged = true
	}
	if input.EndDate != nil && !input.EndDate.IsZero() && !input.EndDate.Equal(tournament.EndDate) {
		tournament.EndDate = *input.EndDate
		updated = true
		datesChanged = true
	}
	if datesChanged {
		if err := validateTournamentDates(tournament.RegDate, tournament.StartDate, tournament.EndDate); err != nil {
			return nil, err
		}
	}

	if input.Location != nil && derefString(input.Location) != derefString(tournament.Location) {
		tournament.Location = input.Location
		updated = true
	}
	if input.MaxParticipants != nil && *input.MaxParticipants != tournament.MaxParticipants {
		if *input.MaxParticipants <= 0 {
			return nil, ErrTournamentInvalidCapacity
		}
		tournament.MaxParticipants = *input.MaxParticipants
		updated = true
	}

	if !updated {
		s.populateTournamentDetails(ctx, tournament)
		return tournament, nil
	}

	err = s.tournamentRepo.Update(ctx, tournament)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentNameConflict) {
			return nil, ErrTournamentNameConflict
		}
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "%w: error from repository on update", ErrTournamentUpdateFailed)
	}
	s.populateTournamentDetails(ctx, tournament)
	return tournament, nil
}

func (s *tournamentService) UpdateTournamentStatus(ctx context.Context, id int, currentUserID int, newStatus models.TournamentStatus, exec repositories.SQLExecutor) (*models.Tournament, error) {
	s.logger.InfoContext(ctx, "UpdateTournamentStatus called", slog.Int("tournament_id", id), slog.String("new_status", string(newStatus)), slog.Int("user_id", currentUserID))

	var tournament *models.Tournament
	var err error

	tournament, err = s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "UpdateTournamentStatus: failed to get tournament %d", id)
	}

	if currentUserID != 0 && tournament.OrganizerID != currentUserID {
		return nil, ErrForbiddenOperation
	}

	// 3. Load format if not already loaded and needed (especially for 'active' transition)
	if tournament.Format == nil && tournament.FormatID > 0 {
		format, formatErr := s.formatRepo.GetByID(ctx, tournament.FormatID)
		if formatErr != nil {
			s.logger.WarnContext(ctx, "UpdateTournamentStatus: could not load format", slog.Int("tournament_id", id), slog.Int("format_id", tournament.FormatID), slog.Any("error", formatErr))
		} else {
			tournament.Format = format
		}
	}
	if tournament.Format == nil && newStatus == models.StatusActive {
		s.logger.ErrorContext(ctx, "UpdateTournamentStatus: Format not loaded, cannot generate bracket", slog.Int("tournament_id", id))
		return nil, fmt.Errorf("format is required to activate tournament %d and generate bracket", id)
	}

	// 4. Validate status transition
	currentStatus := tournament.Status
	if !isValidStatusTransition(currentStatus, newStatus) {
		return nil, fmt.Errorf("%w: from '%s' to '%s' for tournament %d", ErrTournamentInvalidStatusTransition, currentStatus, newStatus, id)
	}
	if currentStatus == newStatus { // No change
		s.populateTournamentDetails(ctx, tournament)
		return tournament, nil
	}

	// 5. Transaction management
	var ownTx *sql.Tx
	var opErr error
	executor := exec

	if executor == nil {
		tmpTx, txErr := s.db.BeginTx(ctx, nil)
		if txErr != nil {
			s.logger.ErrorContext(ctx, "UpdateTournamentStatus: Failed to begin new transaction", slog.Int("tournament_id", id), slog.Any("error", txErr))
			return nil, fmt.Errorf("failed to begin transaction for status update: %w", txErr)
		}
		ownTx = tmpTx
		executor = ownTx
		defer func() {
			if ownTx != nil { // If ownTx is not nil, it means Commit wasn't called or failed
				s.logger.WarnContext(ctx, "Rolling back own transaction in UpdateTournamentStatus", slog.Int("tournament_id", id), slog.Any("operation_error", opErr))
				if rbErr := ownTx.Rollback(); rbErr != nil {
					s.logger.ErrorContext(ctx, "Failed to rollback own transaction", slog.Int("tournament_id", id), slog.Any("rollback_error", rbErr))
				}
			}
		}()
	}

	// 6. Update status in DB
	opErr = s.tournamentRepo.UpdateStatus(ctx, executor, id, newStatus)
	if opErr != nil {
		s.logger.ErrorContext(ctx, "Failed to update tournament status in DB", slog.Int("tournament_id", id), slog.String("new_status", string(newStatus)), slog.Any("error", opErr))
		// If using ownTx, defer will handle rollback. If using passed exec, caller handles rollback.
		return nil, fmt.Errorf("UpdateTournamentStatus: failed to update status in repo for tournament %d: %w", id, opErr)
	}
	tournament.Status = newStatus // Reflect change in local model
	s.logger.InfoContext(ctx, "Tournament status updated in DB", slog.Int("tournament_id", id), slog.String("new_status", string(newStatus)))

	// 7. Handle side effects of status change (e.g., bracket generation)
	if newStatus == models.StatusActive && currentStatus != models.StatusActive {
		s.logger.InfoContext(ctx, "Tournament became active, attempting bracket generation", slog.Int("tournament_id", id))
		if tournament.Format == nil { // Should have been loaded earlier
			opErr = fmt.Errorf("format is nil, cannot generate bracket for tournament %d", id)
			s.logger.ErrorContext(ctx, opErr.Error(), slog.Int("tournament_id", id))
			return nil, opErr // This will trigger rollback if ownTx
		}

		// Check if matches already exist (idempotency for bracket generation)
		var existingMatches bool
		// This check should ideally also use the executor if reads need to be consistent with the current transaction.
		// For simplicity, if repo methods don't support exec for reads, this might use a separate connection.
		if tournament.Format.ParticipantType == models.FormatParticipantSolo {
			solos, _ := s.soloMatchRepo.ListByTournament(ctx, tournament.ID, nil, nil) // Assumes read doesn't need to be in this tx or repo supports it
			existingMatches = len(solos) > 0
		} else {
			teams, _ := s.teamMatchRepo.ListByTournament(ctx, tournament.ID, nil, nil)
			existingMatches = len(teams) > 0
		}

		if !existingMatches {
			_, bracketErr := s.bracketService.GenerateAndSaveBracket(ctx, executor, tournament)
			if bracketErr != nil {
				opErr = fmt.Errorf("status updated to active, but bracket/standings generation failed: %w", bracketErr)
				s.logger.ErrorContext(ctx, "Bracket/standings generation failed", slog.Int("tournament_id", id), slog.Any("error", opErr))
				return nil, opErr
			}
			s.logger.InfoContext(ctx, "Bracket/standings generated successfully", slog.Int("tournament_id", id))
		} else {
			s.logger.InfoContext(ctx, "Matches already exist, skipping bracket/standings generation", slog.Int("tournament_id", id))
		}
	}
	if newStatus == models.StatusCanceled && currentStatus != models.StatusCanceled {
		if tournament.Format != nil && tournament.Format.BracketType == "RoundRobin" {
			s.logger.InfoContext(ctx, "Tournament canceled, deleting standings", slog.Int("tournament_id", id))
			if errDelStandings := s.standingRepo.DeleteByTournamentID(ctx, executor, id); errDelStandings != nil {
				s.logger.ErrorContext(ctx, "Failed to delete standings for canceled RoundRobin tournament", slog.Int("tournament_id", id), slog.Any("error", errDelStandings))
			}
		}
	}

	// 8. Commit transaction if owned by this function
	if ownTx != nil {
		if errCommit := ownTx.Commit(); errCommit != nil {
			ownTx = nil // Nullify to prevent deferred rollback from trying again
			opErr = fmt.Errorf("failed to commit transaction for status update: %w", errCommit)
			s.logger.ErrorContext(ctx, "Failed to commit own transaction", slog.Int("tournament_id", id), slog.Any("error", opErr))
			return nil, opErr
		}
		ownTx = nil // Mark as committed
		s.logger.InfoContext(ctx, "Own transaction committed successfully", slog.Int("tournament_id", id))

		// Send WebSocket notifications only after successful commit of an owned transaction
		if s.hub != nil {
			roomID := "tournament_" + strconv.Itoa(tournament.ID)
			statusPayload := map[string]interface{}{"tournament_id": tournament.ID, "new_status": newStatus, "old_status": currentStatus}
			s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "TOURNAMENT_STATUS_UPDATED", Payload: statusPayload, RoomID: roomID})

			if newStatus == models.StatusActive && currentStatus != models.StatusActive {
				fullBracketData, errData := s.GetTournamentBracketData(ctx, tournament.ID)
				if errData == nil {
					s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "BRACKET_UPDATED", Payload: fullBracketData, RoomID: roomID})
					if tournament.Format != nil && tournament.Format.BracketType == "RoundRobin" {
						s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "STANDINGS_UPDATED", Payload: fullBracketData.Standings, RoomID: roomID})
					}
				} else {
					s.logger.WarnContext(ctx, "Failed to get full bracket data for WebSocket broadcast after status update", slog.Int("tournament_id", id), slog.Any("error", errData))
				}
			}
		}
	}

	updatedTournament, fetchErr := s.GetTournamentByID(ctx, id, 0)
	if fetchErr != nil {
		s.logger.ErrorContext(ctx, "Failed to re-fetch tournament after status update, returning potentially stale data", slog.Int("tournament_id", id), slog.Any("error", fetchErr))
		s.populateTournamentDetails(ctx, tournament)
		return tournament, nil
	}
	return updatedTournament, nil
}

func (s *tournamentService) DeleteTournament(ctx context.Context, id int, currentUserID int) error {
	tournament, err := s.tournamentRepo.GetByID(ctx, id)
	if err != nil {
		return handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament %d for deletion check", id)
	}

	if tournament.OrganizerID != currentUserID { // Add admin role check if necessary
		return ErrForbiddenOperation
	}

	// Allow deletion only for 'soon', 'registration', or 'canceled' tournaments
	if tournament.Status != models.StatusSoon && tournament.Status != models.StatusRegistration && tournament.Status != models.StatusCanceled {
		return fmt.Errorf("%w: cannot delete tournament with status '%s'", ErrTournamentDeletionNotAllowed, tournament.Status)
	}

	// Check for dependent entities (participants, matches) before attempting to delete
	// This might involve new repository methods or checks here.
	// For now, relying on DB foreign key constraints to signal ErrTournamentInUse.

	oldLogoKey := tournament.LogoKey
	err = s.tournamentRepo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrTournamentInUse) {
			// This specific error from repo indicates FK violation, meaning it has participants/matches
			return fmt.Errorf("%w: tournament might have participants or matches: %w", ErrTournamentDeletionNotAllowed, err)
		}
		return handleRepositoryError(err, ErrTournamentNotFound, "%w on delete", ErrTournamentDeleteFailed)
	}

	// If deletion was successful, attempt to delete logo from storage
	if oldLogoKey != nil && *oldLogoKey != "" && s.uploader != nil {
		go func(keyToDelete string) { // Asynchronous deletion
			deleteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if deleteErr := s.uploader.Delete(deleteCtx, keyToDelete); deleteErr != nil {
				s.logger.WarnContext(context.Background(), "Failed to delete tournament logo from storage",
					slog.String("key", keyToDelete), slog.Int("tournament_id", id), slog.Any("error", deleteErr))
			}
		}(*oldLogoKey)
	}
	return nil
}

func (s *tournamentService) UploadTournamentLogo(ctx context.Context, tournamentID int, currentUserID int, file io.Reader, contentType string) (*models.Tournament, error) {
	tournament, err := s.tournamentRepo.GetByID(ctx, tournamentID)
	if err != nil {
		return nil, handleRepositoryError(err, ErrTournamentNotFound, "failed to get tournament %d for logo upload", tournamentID)
	}

	if tournament.OrganizerID != currentUserID { // Add admin role check if necessary
		return nil, ErrForbiddenOperation
	}
	// Potentially check tournament status to disallow logo changes for active/completed tournaments

	if !strings.HasPrefix(contentType, "image/") {
		return nil, ErrInvalidLogoFormat // Assuming this error is defined in services/errors.go
	}
	if s.uploader == nil {
		return nil, errors.New("file uploader is not configured")
	}

	oldLogoKey := tournament.LogoKey
	ext, errExt := GetExtensionFromContentType(contentType) // Using helper
	if errExt != nil {
		return nil, errExt
	}
	newKey := fmt.Sprintf("%s/%d/logo_%d%s", tournamentLogoPrefix, tournamentID, time.Now().UnixNano(), ext)

	if _, errUpload := s.uploader.Upload(ctx, newKey, contentType, file); errUpload != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrLogoUploadFailed, newKey, errUpload)
	}

	if errDbUpdate := s.tournamentRepo.UpdateLogoKey(ctx, tournamentID, &newKey); errDbUpdate != nil {
		// Attempt to clean up the newly uploaded file if DB update fails
		go func(keyToDelete string) {
			s.logger.InfoContext(context.Background(), "Attempting to delete orphaned logo from storage", slog.String("key", keyToDelete))
			if delErr := s.uploader.Delete(context.Background(), keyToDelete); delErr != nil {
				s.logger.WarnContext(context.Background(), "Failed to delete orphaned logo", slog.String("key", keyToDelete), slog.Any("error", delErr))
			}
		}(newKey)
		return nil, handleRepositoryError(errDbUpdate, ErrTournamentNotFound, "%w: failed to update logo key in db", ErrLogoUpdateDatabaseFailed)
	}

	// If update was successful and there was an old logo, delete it from storage
	if oldLogoKey != nil && *oldLogoKey != "" && *oldLogoKey != newKey {
		go func(keyToDelete string) { // Asynchronous deletion
			deleteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if deleteErr := s.uploader.Delete(deleteCtx, keyToDelete); deleteErr != nil {
				s.logger.WarnContext(context.Background(), "Failed to delete old tournament logo from storage",
					slog.String("key", keyToDelete), slog.Int("tournament_id", tournamentID), slog.Any("error", deleteErr))
			}
		}(*oldLogoKey)
	}

	tournament.LogoKey = &newKey
	populateTournamentLogoURLFunc(tournament, s.uploader) // Update LogoURL in the model
	return tournament, nil
}

func (s *tournamentService) FinalizeTournament(ctx context.Context, tournamentID int, winnerParticipantDBID *int, currentUserID int) (*models.Tournament, error) {
	s.logger.InfoContext(ctx, "FinalizeTournament called", slog.Int("tournament_id", tournamentID), slog.Any("winner_pid", winnerParticipantDBID), slog.Int("user_id", currentUserID))

	tournament, err := s.GetTournamentByID(ctx, tournamentID, currentUserID)
	if err != nil {
		return nil, fmt.Errorf("FinalizeTournament: failed to get tournament %d: %w", tournamentID, err)
	}

	if currentUserID != 0 && tournament.OrganizerID != currentUserID {
		return nil, ErrForbiddenOperation
	}

	if tournament.Status == models.StatusCompleted {
		s.logger.InfoContext(ctx, "Tournament already completed", slog.Int("tournament_id", tournamentID))
		return tournament, nil
	}
	if tournament.Status != models.StatusActive {
		return nil, fmt.Errorf("cannot finalize tournament %d with status '%s', expected '%s'",
			tournamentID, tournament.Status, models.StatusActive)
	}

	finalWinnerPID := winnerParticipantDBID // Use the provided one if available
	var winnerView *ParticipantView

	if tournament.Format != nil && tournament.Format.BracketType == "RoundRobin" {
		// For RoundRobin, determine winner from standings if not explicitly provided
		standings, standingsErr := s.standingRepo.ListByTournament(ctx, s.db, tournamentID, true) // Sort by rank
		if standingsErr == nil && len(standings) > 0 {
			topStandingParticipantID := standings[0].ParticipantID
			if finalWinnerPID == nil { // If no winner was provided, use top of standings
				finalWinnerPID = &topStandingParticipantID
			}
			// Even if a winner was provided, we can fetch details for the top of standings for logging/consistency
			// Or, fetch details for finalWinnerPID
			if finalWinnerPID != nil {
				winnerParticipant, errP := s.participantRepo.GetWithDetails(ctx, *finalWinnerPID)
				if errP == nil {
					if winnerParticipant.User != nil {
						populateUserDetailsFunc(winnerParticipant.User, s.uploader)
					}
					if winnerParticipant.Team != nil && winnerParticipant.Team.LogoKey != nil {
						url := s.uploader.GetPublicURL(*winnerParticipant.Team.LogoKey)
						if url != "" {
							winnerParticipant.Team.LogoURL = &url
						}
					}
					tmpView := participantToParticipantViewFunc(winnerParticipant, s.uploader)
					winnerView = &tmpView
				} else {
					s.logger.WarnContext(ctx, "Could not fetch details for determined winner", slog.Any("participant_id", finalWinnerPID), slog.Any("error", errP))
				}
			}
		} else if standingsErr != nil {
			s.logger.WarnContext(ctx, "Could not determine RoundRobin winner from standings due to error", slog.Int("tournament_id", tournamentID), slog.Any("error", standingsErr))
		} else {
			s.logger.WarnContext(ctx, "No standings found for RoundRobin tournament", slog.Int("tournament_id", tournamentID))
		}
	} else if finalWinnerPID != nil { // For SE, if winner provided, fetch details
		winnerParticipant, errP := s.participantRepo.GetWithDetails(ctx, *finalWinnerPID)
		if errP != nil {
			return nil, handleRepositoryError(errP, ErrParticipantNotFound, "FinalizeTournament: winner participant ID %d not found or details missing", *finalWinnerPID)
		}
		if winnerParticipant.TournamentID != tournamentID { // This check might be redundant if winner came from last match
			return nil, fmt.Errorf("winner participant ID %d does not belong to tournament %d", *finalWinnerPID, tournamentID)
		}
		if winnerParticipant.User != nil {
			populateUserDetailsFunc(winnerParticipant.User, s.uploader)
		}
		if winnerParticipant.Team != nil && winnerParticipant.Team.LogoKey != nil {
			url := s.uploader.GetPublicURL(*winnerParticipant.Team.LogoKey)
			if url != "" {
				winnerParticipant.Team.LogoURL = &url
			}
		}
		tmpView := participantToParticipantViewFunc(winnerParticipant, s.uploader)
		winnerView = &tmpView
	}

	var updatedTournament *models.Tournament
	err = s.withTransaction(ctx, func(tx repositories.SQLExecutor) error {
		var txErr error
		updatedTournament, txErr = s.UpdateTournamentStatus(ctx, tournamentID, currentUserID, models.StatusCompleted, tx)
		if txErr != nil {
			return fmt.Errorf("FinalizeTournament: failed to update tournament status to completed: %w", txErr)
		}

		if finalWinnerPID != nil {
			if errUpdateWinner := s.tournamentRepo.UpdateOverallWinner(ctx, tx, tournamentID, finalWinnerPID); errUpdateWinner != nil {
				// Log this error, but don't necessarily fail the whole finalization if status update succeeded
				s.logger.ErrorContext(ctx, "FinalizeTournament: failed to persist overall winner", slog.Int("tournament_id", tournamentID), slog.Any("winner_pid", finalWinnerPID), slog.Any("error", errUpdateWinner))
				// Depending on strictness, you might choose to return errUpdateWinner here.
			} else {
				if updatedTournament != nil { // Make sure updatedTournament is not nil
					updatedTournament.OverallWinnerParticipantID = finalWinnerPID
				}
				s.logger.InfoContext(ctx, "Tournament overall winner persisted", slog.Int("tournament_id", tournamentID), slog.Any("winner_pid", finalWinnerPID))
			}
		}
		return nil
	})

	if err != nil {
		return nil, err // err from withTransaction (commit/rollback or the inner function)
	}

	s.logger.InfoContext(ctx, "Tournament finalized", slog.Int("tournament_id", tournamentID), slog.Any("winner_pid", finalWinnerPID))

	if s.hub != nil {
		roomID := "tournament_" + strconv.Itoa(tournamentID)
		completionPayload := map[string]interface{}{
			"tournament_id":            tournamentID,
			"winner_participant_db_id": finalWinnerPID,
			"winner_details":           winnerView,
		}
		s.hub.BroadcastToRoom(roomID, brackets.WebSocketMessage{Type: "TOURNAMENT_COMPLETED", Payload: completionPayload, RoomID: roomID})
		s.logger.InfoContext(ctx, "Sent TOURNAMENT_COMPLETED to room", slog.String("room_id", roomID), slog.Any("winner_pid", finalWinnerPID))
	}

	// Re-fetch to ensure the returned object has the overall winner ID if it was just set
	finalUpdatedTournament, fetchErr := s.GetTournamentByID(ctx, tournamentID, 0)
	if fetchErr != nil {
		s.logger.ErrorContext(ctx, "Failed to re-fetch tournament after finalization, returning previous state", slog.Int("tournament_id", tournamentID), slog.Any("error", fetchErr))
		return updatedTournament, nil // Return the one from UpdateTournamentStatus if re-fetch fails
	}
	return finalUpdatedTournament, nil
}

func (s *tournamentService) AutoUpdateTournamentStatusesByDates(ctx context.Context) error {
	s.logger.InfoContext(ctx, "Scheduler: Starting automatic tournament status update cycle.")

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.logger.ErrorContext(ctx, "Scheduler: failed to begin transaction", slog.Any("error", err))
		return fmt.Errorf("scheduler: failed to begin transaction: %w", err)
	}

	var opErr error
	defer func() {
		if p := recover(); p != nil {
			s.logger.ErrorContext(ctx, "Scheduler: recovered from panic, rolling back transaction", slog.Any("panic_value", p))
			if rbErr := tx.Rollback(); rbErr != nil {
				s.logger.ErrorContext(ctx, "Scheduler: failed to rollback transaction after panic", slog.Any("rollback_error", rbErr))
			}
			panic(p)
		} else if opErr != nil {
			s.logger.ErrorContext(ctx, "Scheduler: rolling back transaction due to error", slog.Any("error", opErr))
			if rbErr := tx.Rollback(); rbErr != nil {
				s.logger.ErrorContext(ctx, "Scheduler: failed to rollback transaction", slog.Any("rollback_error", rbErr))
			}
		} else {
			if cErr := tx.Commit(); cErr != nil {
				s.logger.ErrorContext(ctx, "Scheduler: failed to commit transaction", slog.Any("error", cErr))
				opErr = fmt.Errorf("scheduler: failed to commit transaction: %w", cErr)
			} else {
				s.logger.InfoContext(ctx, "Scheduler: Transaction committed successfully for status updates.")
			}
		}
	}()

	acquiredLock, lockErr := db.TryAcquireTransactionalLock(ctx, tx, db.SchedulerAdvisoryLockID, s.logger)
	if lockErr != nil {
		opErr = fmt.Errorf("scheduler: error trying to acquire lock: %w", lockErr)
		return opErr
	}

	if !acquiredLock {
		s.logger.InfoContext(ctx, "Scheduler: Could not acquire lock, another instance is likely working. Skipping cycle.")
		return nil
	}
	s.logger.InfoContext(ctx, "Scheduler: Lock acquired. Proceeding with status updates.")

	currentTime := time.Now()
	// GetTournamentsForAutoStatusUpdate already uses the transaction 'tx'
	tournamentsToUpdate, err := s.tournamentRepo.GetTournamentsForAutoStatusUpdate(ctx, tx, currentTime)
	if err != nil {
		opErr = fmt.Errorf("scheduler: failed to get tournaments for status update: %w", err)
		return opErr
	}

	if len(tournamentsToUpdate) == 0 {
		s.logger.InfoContext(ctx, "Scheduler: No tournaments found requiring status update based on dates.")
		return nil
	}

	s.logger.InfoContext(ctx, "Scheduler: Found tournaments to potentially update.", slog.Int("count", len(tournamentsToUpdate)))

	updatedCount := 0
	for _, t := range tournamentsToUpdate { // t is *models.Tournament
		originalStatus := t.Status
		var newStatus models.TournamentStatus

		if currentTime.Before(t.RegDate) && originalStatus == models.StatusSoon { // Should ideally not be picked by query if already soon
			continue // No change needed or already in the correct pre-registration state.
		} else if currentTime.Before(t.StartDate) && (originalStatus == models.StatusSoon || originalStatus == models.StatusRegistration) {
			newStatus = models.StatusRegistration
		} else if currentTime.Before(t.EndDate) && (originalStatus == models.StatusRegistration || originalStatus == models.StatusActive) {
			newStatus = models.StatusActive
		} else if currentTime.After(t.EndDate) && originalStatus == models.StatusActive {
			newStatus = models.StatusCompleted
		} else {
			continue
		}

		if newStatus != "" && newStatus != originalStatus {
			s.logger.InfoContext(ctx, "Scheduler: Attempting to update tournament status",
				slog.Int("tournament_id", t.ID), slog.String("name", t.Name),
				slog.String("old_status", string(originalStatus)), slog.String("new_status", string(newStatus)))

			updatedTournament, updateErr := s.UpdateTournamentStatus(ctx, t.ID, 0, newStatus, tx) // Pass 0 for system user, and tx
			if updateErr != nil {
				s.logger.ErrorContext(ctx, "Scheduler: Error from UpdateTournamentStatus for tournament",
					slog.Int("tournament_id", t.ID), slog.Any("error", updateErr))
				opErr = fmt.Errorf("scheduler: error processing tournament %d (current status %s) to %s: %w", t.ID, originalStatus, newStatus, updateErr)
				return opErr
			}

			if updatedTournament.Status == newStatus {
				s.logger.InfoContext(ctx, "Scheduler: Successfully updated tournament status",
					slog.Int("tournament_id", t.ID), slog.String("new_status", string(updatedTournament.Status)))
				updatedCount++
			} else {
				s.logger.WarnContext(ctx, "Scheduler: Tournament status was not updated as expected",
					slog.Int("tournament_id", t.ID),
					slog.String("expected_status", string(newStatus)),
					slog.String("actual_status", string(updatedTournament.Status)))
			}
		}
	}

	s.logger.InfoContext(ctx, "Scheduler: Processed tournaments for status update.", slog.Int("eligible_count", len(tournamentsToUpdate)), slog.Int("updated_count", updatedCount))
	return opErr
}

func (s *tournamentService) populateTournamentDetails(ctx context.Context, tournament *models.Tournament) {
	if tournament == nil {
		return
	}
	populateTournamentLogoURLFunc(tournament, s.uploader)

	g := errgroup.Group{} // Use errgroup for concurrent fetches

	if tournament.Sport == nil && tournament.SportID > 0 {
		g.Go(func() error {
			sport, err := s.sportRepo.GetByID(ctx, tournament.SportID)
			if err == nil && sport != nil {
				populateSportLogoURLFunc(sport, s.uploader)
				tournament.Sport = sport
			} else if err != nil {
				s.logger.WarnContext(ctx, "Failed to populate sport details", slog.Int("tournament_id", tournament.ID), slog.Int("sport_id", tournament.SportID), slog.Any("error", err))
			}
			return nil // Don't fail all for one missing detail
		})
	}

	if tournament.Format == nil && tournament.FormatID > 0 {
		g.Go(func() error {
			format, err := s.formatRepo.GetByID(ctx, tournament.FormatID)
			if err == nil && format != nil {
				tournament.Format = format
				// Try to parse RoundRobin settings if applicable
				if format.BracketType == "RoundRobin" {
					rrSettings, _ := format.GetRoundRobinSettings() // Ignore error for display
					tournament.Format.ParsedRoundRobinSettings = rrSettings
				}
			} else if err != nil {
				s.logger.WarnContext(ctx, "Failed to populate format details", slog.Int("tournament_id", tournament.ID), slog.Int("format_id", tournament.FormatID), slog.Any("error", err))
			}
			return nil
		})
	}

	if tournament.Organizer == nil && tournament.OrganizerID > 0 {
		g.Go(func() error {
			organizer, err := s.userRepo.GetByID(ctx, tournament.OrganizerID)
			if err == nil && organizer != nil {
				populateUserDetailsFunc(organizer, s.uploader)
				tournament.Organizer = organizer
			} else if err != nil {
				s.logger.WarnContext(ctx, "Failed to populate organizer details", slog.Int("tournament_id", tournament.ID), slog.Int("organizer_id", tournament.OrganizerID), slog.Any("error", err))
			}
			return nil
		})
	}
	_ = g.Wait() // Wait for all fetches
}

// GetTournamentBracketData retrieves all necessary data to display a tournament bracket.
func (s *tournamentService) GetTournamentBracketData(ctx context.Context, tournamentID int) (*FullTournamentBracketView, error) {
	s.logger.InfoContext(ctx, "GetTournamentBracketData: fetching data for tournament", slog.Int("tournament_id", tournamentID))

	tournament, err := s.GetTournamentByID(ctx, tournamentID, 0) // currentUserID 0 as it's for data fetching
	if err != nil {
		return nil, fmt.Errorf("GetTournamentBracketData: failed to get tournament %d: %w", tournamentID, err)
	}

	if tournament.Format == nil {
		return nil, fmt.Errorf("format information is missing or could not be loaded for tournament %d", tournamentID)
	}
	// Ensure RoundRobinSettings are parsed if it's a RoundRobin tournament
	var rrSettings *models.RoundRobinSettings
	if tournament.Format.BracketType == "RoundRobin" && tournament.Format.ParsedRoundRobinSettings != nil {
		rrSettings = tournament.Format.ParsedRoundRobinSettings
	} else if tournament.Format.BracketType == "RoundRobin" { // Fallback if ParsedRoundRobinSettings was not populated
		rrSettings, _ = tournament.Format.GetRoundRobinSettings()
	}

	statusConfirmed := models.StatusParticipant
	dbParticipants, err := s.participantRepo.ListByTournament(ctx, tournamentID, &statusConfirmed, true)
	if err != nil {
		s.logger.WarnContext(ctx, "GetTournamentBracketData: failed to list participants", slog.Int("tournament_id", tournamentID), slog.Any("error", err))
	}
	populateParticipantListDetailsFunc(dbParticipants, s.uploader)
	participantsMap := make(map[int]ParticipantView)
	for _, p := range dbParticipants {
		if p == nil {
			continue
		}
		participantsMap[p.ID] = participantToParticipantViewFunc(p, s.uploader)
	}

	var allMatchesView []MatchView
	roundsMap := make(map[int][]*MatchView)

	if tournament.Format.ParticipantType == models.FormatParticipantSolo {
		soloMatches, listErr := s.soloMatchRepo.ListByTournament(ctx, tournamentID, nil, nil)
		if listErr != nil {
			return nil, fmt.Errorf("GetTournamentBracketData: failed to list solo matches: %w", listErr)
		}
		for _, sm := range soloMatches {
			if sm == nil {
				continue
			}
			mv := s.toMatchView(sm, nil, participantsMap)
			allMatchesView = append(allMatchesView, mv)
			if tournament.Format.BracketType == "SingleElimination" && sm.Round != nil {
				roundsMap[*sm.Round] = append(roundsMap[*sm.Round], &mv)
			}
		}
	} else if tournament.Format.ParticipantType == models.FormatParticipantTeam {
		teamMatches, listErr := s.teamMatchRepo.ListByTournament(ctx, tournamentID, nil, nil)
		if listErr != nil {
			return nil, fmt.Errorf("GetTournamentBracketData: failed to list team matches: %w", listErr)
		}
		for _, tm := range teamMatches {
			if tm == nil {
				continue
			}
			mv := s.toMatchView(nil, tm, participantsMap)
			allMatchesView = append(allMatchesView, mv)
			if tournament.Format.BracketType == "SingleElimination" && tm.Round != nil {
				roundsMap[*tm.Round] = append(roundsMap[*tm.Round], &mv)
			}
		}
	}
	// Sort allMatchesView for RoundRobin display consistency if needed
	if tournament.Format.BracketType == "RoundRobin" {
		sort.Slice(allMatchesView, func(i, j int) bool {
			if allMatchesView[i].MatchTime.Equal(allMatchesView[j].MatchTime) {
				return allMatchesView[i].MatchID < allMatchesView[j].MatchID
			}
			return allMatchesView[i].MatchTime.Before(allMatchesView[j].MatchTime)
		})
	}

	var roundsViewList []RoundView
	if tournament.Format.BracketType == "SingleElimination" {
		roundNumbers := make([]int, 0, len(roundsMap))
		for rNum := range roundsMap {
			roundNumbers = append(roundNumbers, rNum)
		}
		sort.Ints(roundNumbers)
		for _, rNum := range roundNumbers {
			matchesInRound := roundsMap[rNum]
			sort.Slice(matchesInRound, func(i, j int) bool {
				if matchesInRound[i].OrderInRound != 0 && matchesInRound[j].OrderInRound != 0 {
					if matchesInRound[i].OrderInRound != matchesInRound[j].OrderInRound {
						return matchesInRound[i].OrderInRound < matchesInRound[j].OrderInRound
					}
				}
				return matchesInRound[i].MatchID < matchesInRound[j].MatchID
			})
			roundsViewList = append(roundsViewList, RoundView{RoundNumber: rNum, Matches: dereferenceMatchViews(matchesInRound)})
		}
	}

	var standingsViewList []TournamentStandingView
	if tournament.Format.BracketType == "RoundRobin" {
		dbStandings, standingsErr := s.standingRepo.ListByTournament(ctx, s.db, tournamentID, true) // Sort by rank
		if standingsErr != nil {
			s.logger.ErrorContext(ctx, "GetTournamentBracketData: failed to list standings", slog.Int("tournament_id", tournamentID), slog.Any("error", standingsErr))
			// Return error or empty standings? For now, let's allow empty if DB error.
		} else {
			for i, ds := range dbStandings {
				pView, ok := participantsMap[ds.ParticipantID]
				if !ok { // Should not happen if data is consistent
					s.logger.WarnContext(ctx, "Participant from standings not found in participantsMap", slog.Int("participant_id", ds.ParticipantID))
					continue
				}
				standingsViewList = append(standingsViewList, TournamentStandingView{
					Participant:     pView,
					Rank:            i + 1, // Assuming ListByTournament sorted by rank
					Points:          ds.Points,
					GamesPlayed:     ds.GamesPlayed,
					Wins:            ds.Wins,
					Draws:           ds.Draws,
					Losses:          ds.Losses,
					ScoreFor:        ds.ScoreFor,
					ScoreAgainst:    ds.ScoreAgainst,
					ScoreDifference: ds.ScoreDifference,
				})
			}
		}
	}

	overallWinnerPID := tournament.OverallWinnerParticipantID

	return &FullTournamentBracketView{
		TournamentID:               tournament.ID,
		Name:                       tournament.Name,
		Status:                     tournament.Status,
		Sport:                      tournament.Sport,
		Format:                     tournament.Format,
		TournamentSettings:         rrSettings,
		Rounds:                     roundsViewList,
		Matches:                    allMatchesView,
		Standings:                  standingsViewList,
		ParticipantsMap:            participantsMap,
		OverallWinnerParticipantID: overallWinnerPID,
	}, nil
}

func (s *tournamentService) toMatchView(sm *models.SoloMatch, tm *models.TeamMatch, participantsMap map[int]ParticipantView) MatchView {
	mv := MatchView{}
	var p1ID, p2ID, winnerID, nextMatchID, winnerSlot *int
	var roundVal int
	var bracketUID, scoreStr *string
	var matchTimeVal time.Time
	var statusVal models.MatchStatus
	var p1Score, p2Score *int
	isDraw := false

	if sm != nil {
		mv.MatchID = sm.ID
		bracketUID = sm.BracketMatchUID
		p1ID = sm.P1ParticipantID
		p2ID = sm.P2ParticipantID
		scoreStr = sm.Score
		statusVal = sm.Status
		winnerID = sm.WinnerParticipantID
		if sm.Round != nil {
			roundVal = *sm.Round
		}
		nextMatchID = sm.NextMatchDBID
		winnerSlot = sm.WinnerToSlot
		matchTimeVal = sm.MatchTime
	} else if tm != nil {
		mv.MatchID = tm.ID
		bracketUID = tm.BracketMatchUID
		p1ID = tm.T1ParticipantID
		p2ID = tm.T2ParticipantID
		scoreStr = tm.Score
		statusVal = tm.Status
		winnerID = tm.WinnerParticipantID
		if tm.Round != nil {
			roundVal = *tm.Round
		}
		nextMatchID = tm.NextMatchDBID
		winnerSlot = tm.WinnerToSlot
		matchTimeVal = tm.MatchTime
	} else {
		return mv
	}

	mv.BracketMatchUID = bracketUID
	mv.Status = statusVal
	mv.Round = roundVal
	mv.ScoreString = scoreStr
	mv.WinnerParticipantDBID = winnerID
	mv.NextMatchDBID = nextMatchID
	mv.WinnerToSlot = winnerSlot
	mv.MatchTime = matchTimeVal

	if scoreStr != nil && *scoreStr != "" {
		s1, s2, draw, err := parseScore(*scoreStr) // Assuming parseScore is accessible
		if err == nil {
			p1Score = &s1
			p2Score = &s2
			isDraw = draw
		} else {
			s.logger.WarnContext(context.Background(), "Failed to parse score string for MatchView", slog.Int("match_id", mv.MatchID), slog.String("score", *scoreStr), slog.Any("error", err))
		}
	}
	mv.ScoreP1 = p1Score
	mv.ScoreP2 = p2Score
	mv.IsDraw = isDraw

	if mv.BracketMatchUID != nil && mv.Round != 0 {
		prefix := "R" + strconv.Itoa(mv.Round) + "M"
		uidPart := strings.TrimPrefix(*mv.BracketMatchUID, prefix)
		orderPart := strings.SplitN(uidPart, "S", 2)[0] // S for slot, if used in UID
		order, err := strconv.Atoi(orderPart)
		if err == nil && order > 0 {
			mv.OrderInRound = order
		} else if strings.Contains(*mv.BracketMatchUID, "_L1_") || strings.Contains(*mv.BracketMatchUID, "_L2_") { // For RoundRobin UIDs like T%d_RRM%d_L%d...
			parts := strings.Split(*mv.BracketMatchUID, "_")
			for _, part := range parts {
				if strings.HasPrefix(part, "RRM") {
					numStr := strings.TrimPrefix(part, "RRM")
					if orderVal, convErr := strconv.Atoi(numStr); convErr == nil {
						mv.OrderInRound = orderVal
						break
					}
				}
			}
		}
	}

	if p1ID != nil {
		if pView, ok := participantsMap[*p1ID]; ok {
			mv.Participant1 = &pView
		} else if *p1ID != 0 {
			mv.Participant1 = &ParticipantView{ParticipantDBID: *p1ID, Name: fmt.Sprintf("Participant %d (Details Missing)", *p1ID)}
		}
	}
	if p2ID != nil {
		if pView, ok := participantsMap[*p2ID]; ok {
			mv.Participant2 = &pView
		} else if *p2ID != 0 {
			mv.Participant2 = &ParticipantView{ParticipantDBID: *p2ID, Name: fmt.Sprintf("Participant %d (Details Missing)", *p2ID)}
		}
	}
	return mv
}

